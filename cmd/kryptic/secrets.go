package main

import (
	"fmt"
	"os"

	"github.com/dev-kryptic/daemon/internal/api"
	"github.com/dev-kryptic/daemon/internal/dotenv"
	"github.com/dev-kryptic/daemon/internal/ipc"
	"github.com/dev-kryptic/daemon/internal/login"
)

func secrets() error {
	if len(os.Args) < 3 {
		usage()
		return nil
	}

	switch os.Args[2] {
	case "list":
		return secretsList()
	case "get":
		return secretsGet()
	case "export":
		return secretsExport()
	default:
		usage()
		return nil
	}
}

func secretsList() error {
	client := api.NewClient()
	accessToken, err := login.AccessToken(client)
	if err != nil {
		return err
	}
	projects, err := client.Projects(accessToken)
	if err != nil {
		return err
	}
	for _, project := range projects {
		fmt.Printf("%s  %s  %v\n", project.PublicId, project.Name, project.Environments)
	}
	return nil
}

func secretsGet() error {
	key, projectId, environment := "", "", "development"
	args := os.Args[3:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--project":
			i++
			projectId = args[i]
		case "--env":
			i++
			environment = args[i]
		default:
			key = args[i]
		}
	}
	if key == "" || projectId == "" {
		return fmt.Errorf("usage: kryptic secrets get KEY --project proj_x [--env development]")
	}

	pairs, err := fetchSecretPairs(projectId, environment)
	if err != nil {
		return err
	}
	for _, pair := range pairs {
		if pair.Key == key {
			fmt.Println(pair.Value)
			return nil
		}
	}
	return fmt.Errorf("no secret named %s in %s/%s", key, projectId, environment)
}

func secretsExport() error {
	projectId, environment := "", "development"
	args := os.Args[3:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--project":
			i++
			projectId = args[i]
		case "--env":
			i++
			environment = args[i]
		}
	}
	if projectId == "" {
		return fmt.Errorf("usage: kryptic secrets export --project proj_x [--env development]")
	}

	pairs, err := fetchSecretPairs(projectId, environment)
	if err != nil {
		return err
	}
	dotenv.PrintDotenv(pairs)
	return nil
}

func fetchSecretPairs(projectId, environment string) ([]dotenv.Pair, error) {
	response, err := ipc.Request(map[string]any{"type": "secrets", "projectId": projectId, "environment": environment})
	if err != nil {
		return nil, err
	}
	if response["ok"] != true {
		return nil, fmt.Errorf("%v", response["message"])
	}
	entries, _ := response["secrets"].([]any)
	pairs := make([]dotenv.Pair, 0, len(entries))
	for _, raw := range entries {
		entry, _ := raw.(map[string]any)
		key, _ := entry["key"].(string)
		value, _ := entry["value"].(string)
		pairs = append(pairs, dotenv.Pair{Key: key, Value: value})
	}
	return pairs, nil
}

func flush() error {
	response, err := ipc.Request(map[string]any{"type": "flush"})
	if err != nil {
		return err
	}
	if response["ok"] != true {
		return fmt.Errorf("%v", response["message"])
	}
	fmt.Printf("secrets cache cleared (%v bundle(s) dropped)\n", response["cleared"])
	return nil
}
