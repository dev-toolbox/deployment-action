package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/dev-toolbox/deployment-action/internal/config"
	"github.com/dev-toolbox/deployment-action/internal/githubapi"
	"github.com/dev-toolbox/deployment-action/internal/syncer"
	"github.com/dev-toolbox/deployment-action/internal/versioning"
)

type fakeRepositoryClient struct {
	repoExists bool
	tags       []versioning.Tag
	files      map[string]syncer.File
	commits    [][]string
}

func (f *fakeRepositoryClient) EnsureRepository(context.Context, string, string) error {
	if !f.repoExists {
		return errors.New("deployment repository dev-toolbox/_deployment does not exist")
	}
	return nil
}

func (f *fakeRepositoryClient) ListTagsForCommit(context.Context, string, string, string) ([]versioning.Tag, error) {
	return f.tags, nil
}

func (f *fakeRepositoryClient) ListFiles(_ context.Context, _ string, _ string, _ string, prefix string) ([]syncer.File, error) {
	files := make([]syncer.File, 0)
	for path, file := range f.files {
		if strings.HasPrefix(path, prefix) {
			files = append(files, file)
		}
	}
	sort.Slice(files, func(i int, j int) bool {
		return files[i].Path < files[j].Path
	})
	return files, nil
}

func (f *fakeRepositoryClient) ReadFile(_ context.Context, _ string, _ string, _ string, filePath string) ([]byte, string, bool, error) {
	file, ok := f.files[filePath]
	if !ok {
		return nil, "", false, nil
	}
	return file.Content, file.SHA, true, nil
}

func (f *fakeRepositoryClient) CommitChanges(_ context.Context, _ string, _ string, _ string, _ string, changes []githubapi.Change) error {
	if len(changes) == 0 {
		return nil
	}

	paths := make([]string, 0, len(changes))
	for _, change := range changes {
		paths = append(paths, change.Path)
		if change.Delete {
			delete(f.files, change.Path)
			continue
		}
		f.files[change.Path] = syncer.File{Path: change.Path, Content: change.Content, SHA: nextSHA(change.Path, "")}
	}
	f.commits = append(f.commits, paths)
	return nil
}

func TestRunDeploysFilesAndUpdatesKustomization(t *testing.T) {
	tempDir := t.TempDir()
	sourceDir := filepath.Join(tempDir, "_deployment")
	if err := os.MkdirAll(filepath.Join(sourceDir, "manifests"), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "manifests", "deploy.yaml"), []byte("image: app:{{VERSION}}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	client := &fakeRepositoryClient{
		repoExists: true,
		tags:       []versioning.Tag{{Name: "v1.4.0", CommitSHA: "abcdef123456"}},
		files: map[string]syncer.File{
			"prod/kustomization.yaml": {Path: "prod/kustomization.yaml", SHA: "sha-k", Content: []byte("apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources:\n")},
		},
	}

	cfg := config.Config{
		Owner:                "dev-toolbox",
		SourceRepository:     "service-a",
		DeploymentRepository: "_deployment",
		DeploymentBranch:     "main",
		Placeholder:          "{{VERSION}}",
		SourceDirectory:      sourceDir,
		CurrentBranch:        "main",
		CommitSHA:            "abcdef123456",
	}

	result, err := Run(context.Background(), cfg, client)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result.Stage != versioning.StageProd || result.Version != "1.4.0" {
		t.Fatalf("unexpected result: %#v", result)
	}

	deployed, ok := client.files["prod/service-a/manifests/deploy.yaml"]
	if !ok {
		t.Fatal("expected deployed file to exist in target repo")
	}
	if string(deployed.Content) != "image: app:1.4.0\n" {
		t.Fatalf("unexpected deployed content %q", string(deployed.Content))
	}
	if len(client.commits) != 1 {
		t.Fatalf("expected exactly one commit, got %d", len(client.commits))
	}

	kustomization := string(client.files["prod/kustomization.yaml"].Content)
	if !strings.Contains(kustomization, "./service-a") {
		t.Fatalf("expected kustomization to reference service, got %q", kustomization)
	}
}

func TestRunCleansUpWhenSourceDirectoryMissing(t *testing.T) {
	client := &fakeRepositoryClient{
		repoExists: true,
		files: map[string]syncer.File{
			"dev/service-a/deploy.yaml": {Path: "dev/service-a/deploy.yaml", SHA: "sha-a", Content: []byte("old")},
			"dev/kustomization.yaml":    {Path: "dev/kustomization.yaml", SHA: "sha-k", Content: []byte("apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources:\n  - ./service-a\n")},
		},
	}

	cfg := config.Config{
		Owner:                "dev-toolbox",
		SourceRepository:     "service-a",
		DeploymentRepository: "_deployment",
		DeploymentBranch:     "main",
		Placeholder:          "{{VERSION}}",
		SourceDirectory:      filepath.Join(t.TempDir(), "_deployment"),
		CurrentBranch:        "develop",
		CommitSHA:            "abcdef123456",
	}

	result, err := Run(context.Background(), cfg, client)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result.Stage != versioning.StageDev || result.Version != "abcdef" {
		t.Fatalf("unexpected result: %#v", result)
	}

	if _, ok := client.files["dev/service-a/deploy.yaml"]; ok {
		t.Fatal("expected service file to be deleted")
	}
	if strings.Contains(string(client.files["dev/kustomization.yaml"].Content), "./service-a") {
		t.Fatalf("expected kustomization resource to be removed, got %q", string(client.files["dev/kustomization.yaml"].Content))
	}
	if len(client.commits) != 1 {
		t.Fatalf("expected exactly one commit, got %d", len(client.commits))
	}
}

func TestRunFailsWhenDeploymentRepositoryMissing(t *testing.T) {
	client := &fakeRepositoryClient{repoExists: false, files: map[string]syncer.File{}}
	cfg := config.Config{
		Owner:                "dev-toolbox",
		SourceRepository:     "service-a",
		DeploymentRepository: "_deployment",
		DeploymentBranch:     "main",
		SourceDirectory:      filepath.Join(t.TempDir(), "_deployment"),
		CurrentBranch:        "develop",
		CommitSHA:            "abcdef123456",
	}

	_, err := Run(context.Background(), cfg, client)
	if err == nil {
		t.Fatal("expected error when deployment repository is missing")
	}
}

func TestRunKeepsFilesWhenKustomizationHasNoServiceEntry(t *testing.T) {
	client := &fakeRepositoryClient{
		repoExists: true,
		files: map[string]syncer.File{
			"dev/service-a/deploy.yaml": {Path: "dev/service-a/deploy.yaml", SHA: "sha-a", Content: []byte("old")},
			"dev/kustomization.yaml":    {Path: "dev/kustomization.yaml", SHA: "sha-k", Content: []byte("apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources:\n  - ./service-b\n")},
		},
	}

	cfg := config.Config{
		Owner:                "dev-toolbox",
		SourceRepository:     "service-a",
		DeploymentRepository: "_deployment",
		DeploymentBranch:     "main",
		SourceDirectory:      filepath.Join(t.TempDir(), "_deployment"),
		CurrentBranch:        "develop",
		CommitSHA:            "abcdef123456",
	}

	_, err := Run(context.Background(), cfg, client)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if _, ok := client.files["dev/service-a/deploy.yaml"]; !ok {
		t.Fatal("expected service file to remain because no kustomization entry exists")
	}
	if len(client.commits) != 0 {
		t.Fatalf("expected no commit when nothing changes, got %d", len(client.commits))
	}
}

func nextSHA(path string, existing string) string {
	if existing != "" {
		return existing + "-updated"
	}
	return path + "-sha"
}
