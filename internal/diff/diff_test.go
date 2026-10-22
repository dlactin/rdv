package diff

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dlactin/rdv/internal/git"
	"github.com/dlactin/rdv/internal/options"
	"go.yaml.in/yaml/v3"
)

func TestGetRepoRoot(t *testing.T) {
	path, err := git.GetRepoRoot()
	if err != nil {
		t.Fatalf("GetRepoRoot() failed: %v", err)
	}

	if path == "" {
		t.Errorf("Expected a non-empty path, got empty string")
	}

	if !filepath.IsAbs(path) {
		t.Errorf("Expected an absolute path, got: %s", path)
	}
}

// TestRenderManifests uses the chart and kustomization in our
// examples directory
func TestRenderManifests(t *testing.T) {
	testCases := []struct {
		name        string
		path        string
		opts        options.CmdOptions
		values      []string
		wantContent string
		wantErr     bool
	}{
		{
			name: "Renders Helm chart",
			path: "../../examples/helm/helloworld",
			opts: options.CmdOptions{
				Debug:      false,
				UpdateDeps: false,
				Lint:       false,
			},
			values:      nil,
			wantContent: "kind: ConfigMap",
			wantErr:     false,
		},
		{
			name: "Renders Helm chart with values",
			path: "../../examples/helm/helloworld",
			opts: options.CmdOptions{
				Debug:      false,
				UpdateDeps: false,
				Lint:       false,
			},
			values:      []string{"../../examples/helm/helloworld/values-dev.yaml"},
			wantContent: "nginx:dev",
			wantErr:     false,
		},
		{
			name: "Renders Kustomize project",
			path: "../../examples/kustomize/helloworld",
			opts: options.CmdOptions{
				Debug:      false,
				UpdateDeps: false,
				Lint:       false,
			},
			values:      nil,
			wantContent: "kind: ConfigMap",
			wantErr:     false,
		},
		{
			name: "Returns error for invalid path",
			path: "../../examples/not-a-real-path",
			opts: options.CmdOptions{
				Debug:      false,
				UpdateDeps: false,
				Lint:       false,
			},
			values:  nil,
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output, err := RenderManifests(tc.path, tc.values, tc.opts)

			if (err != nil) != tc.wantErr {
				t.Fatalf("RenderManifests() error = %v, wantErr %v", err, tc.wantErr)
			}

			if !tc.wantErr && !strings.Contains(output, tc.wantContent) {
				t.Errorf("RenderManifests() output did not contain %q. Got:\n%s", tc.wantContent, output)
			}
		})
	}
}

func TestCreateDiff(t *testing.T) {
	testCases := []struct {
		name     string
		a        string
		b        string
		fromName string
		toName   string
		want     string
	}{
		{
			name:     "Simple change",
			a:        "line 1\nline 2\nline 3",
			b:        "line 1\nline two\nline 3",
			fromName: "a.txt",
			toName:   "b.txt",
			want:     "--- a.txt\n+++ b.txt\n@@ -1,3 +1,3 @@\n line 1\n-line 2\n+line two\n line 3\n\\ No newline at end of file\n",
		},
		{
			name:     "No changes",
			a:        "line 1\nline 2",
			b:        "line 1\nline 2",
			fromName: "a.txt",
			toName:   "b.txt",
			want:     "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := CreateDiff(tc.a, tc.b, tc.fromName, tc.toName)
			if strings.TrimSpace(got) != strings.TrimSpace(tc.want) {
				t.Errorf("CreateDiff() =\n%q\nWant:\n%q", got, tc.want)
			}
		})
	}
}

func TestPrintChangeSummary(t *testing.T) {
	tests := []struct {
		name     string
		yaml1    string
		yaml2    string
		expected string
	}{
		{
			name:     "Addition",
			yaml1:    "",
			yaml2:    "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cm1\ndata:\n  key: value",
			expected: "Summary: 1 change (1 added)",
		},
		{
			name:     "Removal",
			yaml1:    "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cm1\ndata:\n  key: value",
			yaml2:    "",
			expected: "Summary: 1 change (1 removed)",
		},
		{
			name:     "Update",
			yaml1:    "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cm1\ndata:\n  key: value",
			yaml2:    "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cm1\ndata:\n  key: modified",
			expected: "Summary: 1 change (1 updated)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report, _ := CreateSemanticDiff(tt.yaml1, tt.yaml2, "old", "new", true)

			// Capture output
			old := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			if err := PrintChangeSummary(report.Report, false); err != nil {
				t.Errorf("PrintChangeSummary() error = %v", err)
			}

			if err := w.Close(); err != nil {
				t.Errorf("w.Close() error = %v", err)
			}
			var buf bytes.Buffer
			if _, err := io.Copy(&buf, r); err != nil {
				t.Errorf("io.Copy() error = %v", err)
			}
			os.Stdout = old

			output := buf.String()
			if !strings.Contains(output, tt.expected) {
				t.Errorf("Expected summary %q not found in output:\n%s", tt.expected, output)
			}
		})
	}

	t.Run("GitHub format", func(t *testing.T) {
		yaml1 := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cm1\ndata:\n  key: value"
		yaml2 := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cm1\ndata:\n  key: modified"
		expected := "**Summary: 1 change (1 updated)**"

		report, _ := CreateSemanticDiff(yaml1, yaml2, "old", "new", true)

		// Capture output
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		if err := PrintChangeSummary(report.Report, true); err != nil {
			t.Errorf("PrintChangeSummary() error = %v", err)
		}

		if err := w.Close(); err != nil {
			t.Errorf("w.Close() error = %v", err)
		}
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, r); err != nil {
			t.Errorf("io.Copy() error = %v", err)
		}
		os.Stdout = old

		output := buf.String()
		if !strings.Contains(output, expected) {
			t.Errorf("Expected GitHub summary %q not found in output:\n%s", expected, output)
		}
	})
}

func TestDedupeMappingKeys(t *testing.T) {
	input := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: dup
  labels:
    executor: "computed"
    executor: ""
spec:
  template:
    spec:
      containers:
        - name: app
          env:
            - name: FOO
              value: first
            - name: FOO
              value: last
`
	inputFile, err := createInputFileFromString(input, "test")
	if err != nil {
		t.Fatalf("createInputFileFromString() error = %v", err)
	}
	if len(inputFile.Documents) != 1 {
		t.Fatalf("Expected 1 document, got %d", len(inputFile.Documents))
	}

	labels := findMappingValue(t, inputFile.Documents[0], "metadata", "labels")
	executorCount := 0
	for i := 0; i < len(labels.Content); i += 2 {
		if labels.Content[i].Value == "executor" {
			executorCount++
			if got := labels.Content[i+1].Value; got != "" {
				t.Errorf("Expected last-wins duplicate resolution (\"\"), got %q", got)
			}
		}
	}
	if executorCount != 1 {
		t.Errorf("Expected 1 executor key after dedup, got %d", executorCount)
	}
}

func TestCreateSemanticDiffDuplicateKeys(t *testing.T) {
	doc := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: sched
  labels:
    executor: "computed"
    executor: ""
`
	tests := []struct {
		name      string
		from      string
		to        string
		wantDiffs int
	}{
		{
			name:      "Identical documents with duplicate keys produce no diff",
			from:      doc,
			to:        doc,
			wantDiffs: 0,
		},
		{
			name: "Real change in the winning duplicate value is still detected",
			from: doc,
			to: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: sched
  labels:
    executor: "computed"
    executor: "override"
`,
			wantDiffs: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report, err := CreateSemanticDiff(tt.from, tt.to, "old", "new", true)
			if err != nil {
				t.Fatalf("CreateSemanticDiff() error = %v", err)
			}
			if len(report.Diffs) != tt.wantDiffs {
				t.Errorf("Expected %d diffs, got %d", tt.wantDiffs, len(report.Diffs))
			}
		})
	}
}

// findMappingValue walks a document node's mapping tree by key path and
// returns the value node at the end of the path.
func findMappingValue(t *testing.T, node *yaml.Node, keys ...string) *yaml.Node {
	t.Helper()
	for node.Kind == yaml.DocumentNode {
		node = node.Content[0]
	}
	for _, key := range keys {
		var next *yaml.Node
		for i := 0; i < len(node.Content); i += 2 {
			if node.Content[i].Value == key {
				next = node.Content[i+1]
				break
			}
		}
		if next == nil {
			t.Fatalf("Key %q not found in mapping", key)
		}
		node = next
	}
	return node
}

func TestFixGitHubDiffOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "Document removal",
			input: `
/
- one document removed:
---
apiVersion: v1
kind: Service
`,
			expected: `
/
----
-apiVersion: v1
-kind: Service
`,
		},
		{
			name: "Value change",
			input: `
/data/key
± value change
- old
+ new
`,
			expected: `
/data/key
± value change
- old
+ new
`,
		},
		{
			name: "Multiple changes with blank lines",
			input: `
/
- one document removed:
---
apiVersion: v1

kind: Service

/data/key
± value change
- old
+ new
`,
			expected: `
/
----
-apiVersion: v1

-kind: Service

/data/key
± value change
- old
+ new
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FixGitHubDiffOutput(strings.TrimSpace(tt.input))
			if strings.TrimSpace(got) != strings.TrimSpace(tt.expected) {
				t.Errorf("FixGitHubDiffOutput() = %q, want %q", got, tt.expected)
			}
		})
	}
}
