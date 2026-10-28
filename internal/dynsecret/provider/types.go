package provider

import "time"

const (
	ProviderPostgres  = 0
	ProviderMySQL     = 1
	ProviderOracle    = 2
	ProviderCassandra = 3
	ProviderRedis     = 4
	ProviderAwsIAM    = 5

	ActionCreate = 0
	ActionRenew  = 1
	ActionRevoke = 2
)

type ProviderConfig struct {
	SSLVerify          *bool  `json:"sslVerify"`
	Host               string `json:"host"`
	Username           string `json:"username"`
	Password           string `json:"password"`
	Database           string `json:"database"`
	CA                 string `json:"ca"`
	Hosts              string `json:"hosts"`
	LocalDc            string `json:"localDc"`
	Keyspace           string `json:"keyspace"`
	AuthMethod         string `json:"authMethod"`
	CredentialType     string `json:"credentialType"`
	AccessKeyId        string `json:"accessKeyId"`
	SecretAccessKey    string `json:"secretAccessKey"`
	RoleArn            string `json:"roleArn"`
	Region             string `json:"region"`
	PolicyDocument     string `json:"policyDocument"`
	PermissionBoundary string `json:"permissionBoundary"`
	IamUserPath        string `json:"iamUserPath"`
	Port               int    `json:"port"`
	DurationSeconds    int    `json:"durationSeconds"`
	SSL                bool   `json:"ssl"`
	TLS                bool   `json:"tls"`
}

type Minted struct {
	Username     string `json:"username,omitempty"`
	Password     string `json:"password,omitempty"`
	SessionToken string `json:"sessionToken,omitempty"`
	AccessKeyId  string `json:"accessKeyId,omitempty"`
	URL          string `json:"url,omitempty"`
}

type WorkContext struct {
	Config              ProviderConfig
	Username            string
	Password            string
	CreationStatement   string
	RevocationStatement string
	RenewStatement      string
	URLTemplate         string
	ExpiresAt           time.Time
	EntityId            string
}
