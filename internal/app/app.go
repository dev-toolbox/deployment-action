package app

import (
	"context"
	"fmt"

	"github.com/dev-toolbox/deployment-action/internal/config"
	"github.com/dev-toolbox/deployment-action/internal/githubapi"
	"github.com/dev-toolbox/deployment-action/internal/kustomize"
	"github.com/dev-toolbox/deployment-action/internal/syncer"
	"github.com/dev-toolbox/deployment-action/internal/versioning"
)

func Run(ctx context.Context, cfg config.Config, client githubapi.RepositoryClient) (versioning.Result, error) {
	if err := client.EnsureRepository(ctx, cfg.Owner, cfg.DeploymentRepository); err != nil {
		return versioning.Result{}, err
	}

	resolved, err := versioning.Resolve(ctx, client, cfg.Owner, cfg.SourceRepository, cfg.CurrentBranch, cfg.CommitSHA)
	if err != nil {
		return versioning.Result{}, err
	}

	stage := string(resolved.Stage)
	service := cfg.SourceRepository
	resource := "./" + service
	serviceRoot := githubapi.Join(stage, service)
	kustomizationPath := githubapi.Join(stage, "kustomization.yaml")
	commitMessage := fmt.Sprintf("deploy %s to %s (%s)", service, stage, resolved.Version)

	sourceExists, err := syncer.SourceExists(cfg.SourceDirectory)
	if err != nil {
		return versioning.Result{}, fmt.Errorf("check source directory: %w", err)
	}

	if !sourceExists {
		changes, err := cleanupStage(ctx, cfg, client, serviceRoot, kustomizationPath, resource)
		if err != nil {
			return versioning.Result{}, err
		}
		if err := client.CommitChanges(ctx, cfg.Owner, cfg.DeploymentRepository, cfg.DeploymentBranch, commitMessage, changes); err != nil {
			return versioning.Result{}, fmt.Errorf("commit cleanup changes: %w", err)
		}
		fmt.Printf("stage=%s\nversion=%s\n", stage, resolved.Version)
		return resolved, nil
	}

	localFiles, err := syncer.LoadLocalFiles(cfg.SourceDirectory, cfg.Placeholder, resolved.Version)
	if err != nil {
		return versioning.Result{}, fmt.Errorf("load local deployment files: %w", err)
	}

	desiredFiles := syncer.PrefixFiles(serviceRoot, localFiles)
	existingFiles, err := client.ListFiles(ctx, cfg.Owner, cfg.DeploymentRepository, cfg.DeploymentBranch, serviceRoot)
	if err != nil {
		return versioning.Result{}, fmt.Errorf("list existing deployment files: %w", err)
	}

	toUpsert, toDelete := syncer.DiffExisting(desiredFiles, existingFiles)
	changes := make([]githubapi.Change, 0, len(toUpsert)+len(toDelete)+1)
	for _, file := range toDelete {
		changes = append(changes, githubapi.Change{Path: file.Path, Delete: true})
	}
	for _, file := range toUpsert {
		changes = append(changes, githubapi.Change{Path: file.Path, Content: file.Content})
	}

	kustomizationChange, err := ensureKustomization(ctx, cfg, client, kustomizationPath, resource)
	if err != nil {
		return versioning.Result{}, err
	}
	if kustomizationChange != nil {
		changes = append(changes, *kustomizationChange)
	}
	if err := client.CommitChanges(ctx, cfg.Owner, cfg.DeploymentRepository, cfg.DeploymentBranch, commitMessage, changes); err != nil {
		return versioning.Result{}, fmt.Errorf("commit deployment changes: %w", err)
	}

	fmt.Printf("stage=%s\nversion=%s\n", stage, resolved.Version)
	return resolved, nil
}

func cleanupStage(ctx context.Context, cfg config.Config, client githubapi.RepositoryClient, serviceRoot string, kustomizationPath string, resource string) ([]githubapi.Change, error) {
	content, sha, exists, err := client.ReadFile(ctx, cfg.Owner, cfg.DeploymentRepository, cfg.DeploymentBranch, kustomizationPath)
	if err != nil {
		return nil, fmt.Errorf("read kustomization for cleanup: %w", err)
	}
	if !exists {
		return nil, nil
	}

	updated, removed, err := kustomize.RemoveResource(content, resource)
	if err != nil {
		return nil, fmt.Errorf("remove kustomization resource: %w", err)
	}
	if !removed {
		return nil, nil
	}
	changes := []githubapi.Change{{Path: kustomizationPath, Content: updated}}
	if string(updated) == string(content) {
		return changes, nil
	}

	existingFiles, err := client.ListFiles(ctx, cfg.Owner, cfg.DeploymentRepository, cfg.DeploymentBranch, serviceRoot)
	if err != nil {
		return nil, fmt.Errorf("list service files for cleanup: %w", err)
	}
	for _, file := range existingFiles {
		changes = append(changes, githubapi.Change{Path: file.Path, Delete: true})
	}

	_ = sha
	return changes, nil
}

func ensureKustomization(ctx context.Context, cfg config.Config, client githubapi.RepositoryClient, kustomizationPath string, resource string) (*githubapi.Change, error) {
	content, _, exists, err := client.ReadFile(ctx, cfg.Owner, cfg.DeploymentRepository, cfg.DeploymentBranch, kustomizationPath)
	if err != nil {
		return nil, fmt.Errorf("read kustomization: %w", err)
	}
	if !exists {
		content = nil
	}

	updated, err := kustomize.EnsureResource(content, resource)
	if err != nil {
		return nil, fmt.Errorf("ensure kustomization resource: %w", err)
	}
	if exists && string(updated) == string(content) {
		return nil, nil
	}

	return &githubapi.Change{Path: kustomizationPath, Content: updated}, nil
}
