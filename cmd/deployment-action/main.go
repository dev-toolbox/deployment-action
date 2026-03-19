package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	app "github.com/dev-toolbox/deployment-action/internal/app"
	"github.com/dev-toolbox/deployment-action/internal/config"
	"github.com/dev-toolbox/deployment-action/internal/githubapi"
)

func main() {
	ctx := context.Background()

	cfg, err := config.FromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(1)
	}

	client := githubapi.NewClient(ctx, cfg.Token)

	result, err := app.Run(ctx, cfg, client)
	if err != nil {
		fmt.Fprintf(os.Stderr, "deployment action failed: %v\n", err)
		os.Exit(1)
	}

	if err := writeOutputs(map[string]string{
		"stage":   string(result.Stage),
		"version": result.Version,
		"tag":     result.TagName,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write action outputs: %v\n", err)
		os.Exit(1)
	}
}

func writeOutputs(values map[string]string) error {
	outputPath := strings.TrimSpace(os.Getenv("GITHUB_OUTPUT"))
	if outputPath == "" {
		return nil
	}

	file, err := os.OpenFile(outputPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	for key, value := range values {
		if _, err := fmt.Fprintf(file, "%s=%s\n", key, value); err != nil {
			return err
		}
	}

	return nil
}
