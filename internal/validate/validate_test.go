package validate

import (
	"strings"
	"testing"
)

func TestValidateManifests_InvalidManifest(t *testing.T) {
	// Invalid ConfigMap: data must be a map[string]string
	manifest := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: invalid-config
data:
  - not: "a map"
`
	err := Manifests(manifest, false)
	if err == nil {
		t.Fatalf("expected validation error, got nil")
	}

	msg := err.Error()
	if !strings.Contains(msg, "manifest validation failed") {
		t.Errorf("expected error to contain 'manifest validation failed', got: %s", msg)
	}
	if !strings.Contains(msg, "invalid-config") {
		t.Errorf("expected error message to include resource name 'invalid-config'")
	}
	if !strings.Contains(msg, "ConfigMap") {
		t.Errorf("expected error message to include kind 'ConfigMap'")
	}
}

func TestValidateManifests_ValidManifest(t *testing.T) {
	// Valid ConfigMap
	manifest := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: valid-config
data:
  foo: bar
`
	err := Manifests(manifest, false)
	if err != nil {
		t.Errorf("expected successful validation, got %s", err)
	}
}

func TestValidateManifests_CompletelyInvalidYAML(t *testing.T) {
	manifest := "not: [valid: yaml"

	err := Manifests(manifest, false)
	if err == nil {
		t.Fatal("expected validation error for malformed YAML, got nil")
	}

	if !strings.Contains(err.Error(), "manifest validation failed") {
		t.Errorf("unexpected error message: %s", err)
	}
}

func TestValidateManifests_EmptyManifest(t *testing.T) {
	// kubeconform treats empty input as 0 resources
	manifest := ""

	if err := Manifests(manifest, false); err != nil {
		t.Fatalf("expected nil error for empty manifest, got: %v", err)
	}
}
