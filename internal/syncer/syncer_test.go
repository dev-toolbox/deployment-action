package syncer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadLocalFilesReplacesPlaceholder(t *testing.T) {
	tempDir := t.TempDir()
	nestedDir := filepath.Join(tempDir, "nested")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nestedDir, "deployment.yaml"), []byte("image: service:{{VERSION}}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	files, err := LoadLocalFiles(tempDir, "{{VERSION}}", "1.2.3")
	if err != nil {
		t.Fatalf("LoadLocalFiles returned error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected one file, got %d", len(files))
	}
	if got := string(files[0].Content); got != "image: service:1.2.3\n" {
		t.Fatalf("unexpected file content %q", got)
	}
	if files[0].Path != "nested/deployment.yaml" {
		t.Fatalf("unexpected path %q", files[0].Path)
	}
}

func TestDiffExistingComputesUpsertsAndDeletes(t *testing.T) {
	desired := []File{{Path: "dev/service/a.txt", Content: []byte("new")}}
	existing := []File{
		{Path: "dev/service/a.txt", Content: []byte("old"), SHA: "sha-a"},
		{Path: "dev/service/b.txt", Content: []byte("old"), SHA: "sha-b"},
	}

	toUpsert, toDelete := DiffExisting(desired, existing)
	if len(toUpsert) != 1 {
		t.Fatalf("expected one upsert, got %d", len(toUpsert))
	}
	if toUpsert[0].SHA != "sha-a" {
		t.Fatalf("expected sha-a for update, got %q", toUpsert[0].SHA)
	}
	if len(toDelete) != 1 || toDelete[0].Path != "dev/service/b.txt" {
		t.Fatalf("unexpected deletions: %#v", toDelete)
	}
}
