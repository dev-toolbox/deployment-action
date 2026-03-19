package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultDeploymentRepository = "_deployment"
	defaultDeploymentBranch     = "main"
	defaultPlaceholder          = "{{VERSION}}"
	defaultSourceDir            = "_deployment"
)

type Config struct {
	Token                string
	Owner                string
	SourceRepository     string
	DeploymentRepository string
	DeploymentBranch     string
	Placeholder          string
	SourceDirectory      string
	CurrentBranch        string
	CommitSHA            string
	Workspace            string
	Actor                string
}

func FromEnv() (Config, error) {
	repository := strings.TrimSpace(os.Getenv("GITHUB_REPOSITORY"))
	parts := strings.Split(repository, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return Config{}, fmt.Errorf("GITHUB_REPOSITORY must be set to owner/repo")
	}

	commitSHA := strings.TrimSpace(os.Getenv("GITHUB_SHA"))
	if len(commitSHA) < 6 {
		return Config{}, fmt.Errorf("GITHUB_SHA must contain at least 6 characters")
	}

	workspace := strings.TrimSpace(os.Getenv("GITHUB_WORKSPACE"))
	if workspace == "" {
		workspace = "."
	}

	token := strings.TrimSpace(os.Getenv("INPUT_TOKEN"))
	if token == "" {
		return Config{}, fmt.Errorf("action input 'token' is required")
	}

	branch := strings.TrimSpace(os.Getenv("GITHUB_REF_NAME"))
	if branch == "" {
		branch = strings.TrimPrefix(strings.TrimSpace(os.Getenv("GITHUB_REF")), "refs/heads/")
	}
	if branch == "" {
		return Config{}, fmt.Errorf("current branch could not be determined from GITHUB_REF_NAME or GITHUB_REF")
	}

	deploymentRepository := envOrDefault("INPUT_DEPLOYMENT_REPOSITORY", defaultDeploymentRepository)
	deploymentBranch := envOrDefault("INPUT_DEPLOYMENT_BRANCH", defaultDeploymentBranch)
	placeholder := envOrDefault("INPUT_PLACEHOLDER", defaultPlaceholder)
	sourceDirectory := envOrDefault("INPUT_SOURCE_DIRECTORY", defaultSourceDir)

	return Config{
		Token:                token,
		Owner:                parts[0],
		SourceRepository:     parts[1],
		DeploymentRepository: deploymentRepository,
		DeploymentBranch:     deploymentBranch,
		Placeholder:          placeholder,
		SourceDirectory:      filepath.Join(workspace, sourceDirectory),
		CurrentBranch:        branch,
		CommitSHA:            commitSHA,
		Workspace:            workspace,
		Actor:                strings.TrimSpace(os.Getenv("GITHUB_ACTOR")),
	}, nil
}

func envOrDefault(name string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}

	return value
}
