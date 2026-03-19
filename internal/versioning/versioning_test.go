package versioning

import (
	"context"
	"testing"
)

type fakeTagService struct {
	tags []Tag
	err  error
}

func (f fakeTagService) ListTagsForCommit(context.Context, string, string, string) ([]Tag, error) {
	return f.tags, f.err
}

func TestResolveMainWithVersionTagUsesProd(t *testing.T) {
	result, err := Resolve(context.Background(), fakeTagService{tags: []Tag{{Name: "v1.2.3"}}}, "dev-toolbox", "service", "main", "abcdef123456")
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}

	if result.Stage != StageProd {
		t.Fatalf("expected prod stage, got %q", result.Stage)
	}
	if result.Version != "1.2.3" {
		t.Fatalf("expected version 1.2.3, got %q", result.Version)
	}
}

func TestResolveMainWithoutVersionTagFallsBackToDev(t *testing.T) {
	result, err := Resolve(context.Background(), fakeTagService{tags: []Tag{{Name: "build-123"}}}, "dev-toolbox", "service", "main", "abcdef123456")
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}

	if result.Stage != StageDev {
		t.Fatalf("expected dev stage, got %q", result.Stage)
	}
	if result.Version != "abcdef" {
		t.Fatalf("expected short hash abcdef, got %q", result.Version)
	}
}

func TestResolveDevelopUsesShortHash(t *testing.T) {
	result, err := Resolve(context.Background(), fakeTagService{}, "dev-toolbox", "service", "develop", "1234567890ab")
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}

	if result.Stage != StageDev {
		t.Fatalf("expected dev stage, got %q", result.Stage)
	}
	if result.Version != "123456" {
		t.Fatalf("expected short hash 123456, got %q", result.Version)
	}
}

func TestResolveUnsupportedBranchFails(t *testing.T) {
	_, err := Resolve(context.Background(), fakeTagService{}, "dev-toolbox", "service", "feature/test", "1234567890ab")
	if err == nil {
		t.Fatal("expected error for unsupported branch")
	}
}
