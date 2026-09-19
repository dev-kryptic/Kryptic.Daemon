package dynsecret

import (
	"context"
	"errors"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

type awsProvider struct{}

func awsConfig(ctx context.Context, cfg ProviderConfig) (aws.Config, error) {
	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}
	method := strings.ToLower(cfg.AuthMethod)
	switch method {
	case "", "access_key", "accesskey":
		return config.LoadDefaultConfig(ctx,
			config.WithRegion(region),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyId, cfg.SecretAccessKey, "")),
		)
	case "assume_role", "assumerole":
		base, err := config.LoadDefaultConfig(ctx,
			config.WithRegion(region),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyId, cfg.SecretAccessKey, "")),
		)
		if err != nil {
			return aws.Config{}, err
		}
		stsClient := sts.NewFromConfig(base)
		out, err := stsClient.AssumeRole(ctx, &sts.AssumeRoleInput{
			RoleArn:         aws.String(cfg.RoleArn),
			RoleSessionName: aws.String("kryptic-connector"),
		})
		if err != nil {
			return aws.Config{}, err
		}
		creds := out.Credentials
		return config.LoadDefaultConfig(ctx,
			config.WithRegion(region),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				aws.ToString(creds.AccessKeyId),
				aws.ToString(creds.SecretAccessKey),
				aws.ToString(creds.SessionToken),
			)),
		)
	default:
		// irsa / instance role: host chain
		return config.LoadDefaultConfig(ctx, config.WithRegion(region))
	}
}

func (awsProvider) Mint(ctx context.Context, work WorkContext) (Minted, error) {
	awsCfg, err := awsConfig(ctx, work.Config)
	if err != nil {
		return Minted{}, err
	}
	kind := strings.ToLower(work.Config.CredentialType)
	if kind == "temporary" || kind == "sts" {
		return mintSTS(ctx, awsCfg, work)
	}
	return mintIAMUser(ctx, awsCfg, work)
}

func mintSTS(ctx context.Context, awsCfg aws.Config, work WorkContext) (Minted, error) {
	client := sts.NewFromConfig(awsCfg)
	duration := int32(work.Config.DurationSeconds)
	if duration <= 0 {
		duration = 3600
	}
	if work.Config.RoleArn != "" {
		if duration > 3600 {
			duration = 3600
		}
		out, err := client.AssumeRole(ctx, &sts.AssumeRoleInput{
			RoleArn:         aws.String(work.Config.RoleArn),
			RoleSessionName: aws.String(work.Username),
			DurationSeconds: aws.Int32(duration),
		})
		if err != nil {
			return Minted{}, err
		}
		c := out.Credentials
		return Minted{
			Username:     work.Username,
			AccessKeyId:  aws.ToString(c.AccessKeyId),
			Password:     aws.ToString(c.SecretAccessKey),
			SessionToken: aws.ToString(c.SessionToken),
		}, nil
	}
	if duration > 43200 {
		duration = 43200
	}
	out, err := client.GetSessionToken(ctx, &sts.GetSessionTokenInput{DurationSeconds: aws.Int32(duration)})
	if err != nil {
		return Minted{}, err
	}
	c := out.Credentials
	return Minted{
		Username:     work.Username,
		AccessKeyId:  aws.ToString(c.AccessKeyId),
		Password:     aws.ToString(c.SecretAccessKey),
		SessionToken: aws.ToString(c.SessionToken),
	}, nil
}

func mintIAMUser(ctx context.Context, awsCfg aws.Config, work WorkContext) (Minted, error) {
	client := iam.NewFromConfig(awsCfg)
	path := work.Config.IamUserPath
	if path == "" {
		path = "/"
	}
	input := &iam.CreateUserInput{UserName: aws.String(work.Username), Path: aws.String(path)}
	if work.Config.PermissionBoundary != "" {
		input.PermissionsBoundary = aws.String(work.Config.PermissionBoundary)
	}
	if _, err := client.CreateUser(ctx, input); err != nil {
		return Minted{}, err
	}
	if work.Config.PolicyDocument != "" {
		_, err := client.PutUserPolicy(ctx, &iam.PutUserPolicyInput{
			UserName:       aws.String(work.Username),
			PolicyName:     aws.String("kryptic-lease"),
			PolicyDocument: aws.String(work.Config.PolicyDocument),
		})
		if err != nil {
			return Minted{}, err
		}
	}
	key, err := client.CreateAccessKey(ctx, &iam.CreateAccessKeyInput{UserName: aws.String(work.Username)})
	if err != nil {
		return Minted{}, err
	}
	return Minted{
		Username:    work.Username,
		AccessKeyId: aws.ToString(key.AccessKey.AccessKeyId),
		Password:    aws.ToString(key.AccessKey.SecretAccessKey),
	}, nil
}

func (awsProvider) Renew(context.Context, WorkContext) error { return nil }

func (awsProvider) Revoke(ctx context.Context, work WorkContext) error {
	if strings.EqualFold(work.Config.CredentialType, "temporary") || strings.EqualFold(work.Config.CredentialType, "sts") {
		return nil
	}
	awsCfg, err := awsConfig(ctx, work.Config)
	if err != nil {
		return err
	}
	client := iam.NewFromConfig(awsCfg)
	name := work.EntityId
	if name == "" {
		name = work.Username
	}
	keys, err := client.ListAccessKeys(ctx, &iam.ListAccessKeysInput{UserName: aws.String(name)})
	if err != nil {
		var notFound *iamtypes.NoSuchEntityException
		if errors.As(err, &notFound) {
			return nil
		}
		return err
	}
	for _, key := range keys.AccessKeyMetadata {
		_, _ = client.DeleteAccessKey(ctx, &iam.DeleteAccessKeyInput{
			UserName:    aws.String(name),
			AccessKeyId: key.AccessKeyId,
		})
	}
	// DeleteUser refuses while the inline lease policy exists, which would
	// leave the minted IAM user alive past revocation.
	_, _ = client.DeleteUserPolicy(ctx, &iam.DeleteUserPolicyInput{
		UserName:   aws.String(name),
		PolicyName: aws.String("kryptic-lease"),
	})
	_, err = client.DeleteUser(ctx, &iam.DeleteUserInput{UserName: aws.String(name)})
	if err != nil {
		var notFound *iamtypes.NoSuchEntityException
		if errors.As(err, &notFound) {
			return nil
		}
	}
	return err
}
