package kustomize

import (
	"strings"
	"testing"
)

func TestEnsureResourceCreatesMinimalFile(t *testing.T) {
	content, err := EnsureResource(nil, "./service-a")
	if err != nil {
		t.Fatalf("EnsureResource returned error: %v", err)
	}

	text := string(content)
	if !strings.Contains(text, "./service-a") {
		t.Fatalf("expected resource to be present, got %q", text)
	}
	if !strings.Contains(text, "apiVersion: kustomize.config.k8s.io/v1beta1") {
		t.Fatalf("expected apiVersion to be present, got %q", text)
	}
}

func TestEnsureResourceAvoidsDuplicates(t *testing.T) {
	initial := []byte("apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources:\n  - ./service-a\n")
	content, err := EnsureResource(initial, "./service-a")
	if err != nil {
		t.Fatalf("EnsureResource returned error: %v", err)
	}

	if strings.Count(string(content), "./service-a") != 1 {
		t.Fatalf("expected resource exactly once, got %q", string(content))
	}
}

func TestRemoveResourceReportsRemoval(t *testing.T) {
	initial := []byte("apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources:\n  - ./service-a\n  - ./service-b\n")
	content, removed, err := RemoveResource(initial, "./service-a")
	if err != nil {
		t.Fatalf("RemoveResource returned error: %v", err)
	}
	if !removed {
		t.Fatal("expected resource to be removed")
	}
	if strings.Contains(string(content), "./service-a") {
		t.Fatalf("expected resource to be removed, got %q", string(content))
	}
}
