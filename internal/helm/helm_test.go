package helm

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dlactin/rdv/internal/options"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
)

func TestRenderChartRepositoryCache(t *testing.T) {
	for _, failure := range []bool{false, true} {
		t.Run(fmt.Sprintf("download failure=%t", failure), func(t *testing.T) {
			root, cache, chartPath := repositoryCacheFixture(t, failure)
			output, err := RenderChart(chartPath, "test", nil, options.CmdOptions{UpdateDeps: true})
			if failure {
				if err == nil || !strings.Contains(err.Error(), "500") {
					t.Fatalf("expected download error, got %v", err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(output, "name: dependency") {
					t.Fatalf("missing dependency render: %s", output)
				}
				if _, err := RenderChart(chartPath, "test", nil, options.CmdOptions{}); err != nil {
					t.Fatalf("locked build without registered repository: %v", err)
				}
			}
			entries, err := os.ReadDir(cache)
			if err != nil || len(entries) != 0 {
				t.Fatalf("temporary cache not cleaned: %v, %v", entries, err)
			}
			if _, err := os.Stat(filepath.Join(root, "missing-repositories.yaml")); !os.IsNotExist(err) {
				t.Fatalf("global repository config was modified: %v", err)
			}
			if _, err := os.Stat(filepath.Join(chartPath, "tmpcharts")); !os.IsNotExist(err) {
				t.Fatalf("temporary charts not cleaned: %v", err)
			}
		})
	}
}

func repositoryCacheFixture(t *testing.T, failure bool) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	cache := filepath.Join(root, "temp")
	if err := os.Mkdir(cache, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", cache)
	t.Setenv("HELM_REPOSITORY_CONFIG", filepath.Join(root, "missing-repositories.yaml"))
	t.Setenv("HELM_REPOSITORY_CACHE", filepath.Join(root, "global-cache"))
	archive, err := chartutil.Save(&chart.Chart{
		Metadata:  &chart.Metadata{APIVersion: "v2", Name: "dep", Version: "1.0.0"},
		Templates: []*chart.File{{Name: "templates/configmap.yaml", Data: []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: dependency\n")}},
	}, root)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/index.yaml" {
			if _, err := fmt.Fprint(writer, "apiVersion: v1\nentries:\n  dep:\n    - apiVersion: v2\n      name: dep\n      version: 1.0.0\n      urls:\n        - dep-1.0.0.tgz\n"); err != nil {
				t.Errorf("write repository index: %v", err)
			}
			return
		}
		if failure {
			http.Error(writer, "download failed", http.StatusInternalServerError)
			return
		}
		http.ServeFile(writer, request, archive)
	}))
	t.Cleanup(server.Close)
	chartPath := filepath.Join(root, "parent")
	if err := os.Mkdir(chartPath, 0700); err != nil {
		t.Fatal(err)
	}
	metadata := fmt.Sprintf("apiVersion: v2\nname: parent\nversion: 1.0.0\ndependencies:\n  - name: dep\n    version: 1.0.0\n    repository: %s\n", server.URL)
	if err := os.WriteFile(filepath.Join(chartPath, "Chart.yaml"), []byte(metadata), 0600); err != nil {
		t.Fatal(err)
	}
	return root, cache, chartPath
}

func TestRenderChartVendoredDependency(t *testing.T) {
	chartPath := t.TempDir()
	t.Setenv("HELM_REPOSITORY_CONFIG", filepath.Join(chartPath, "missing-repositories.yaml"))
	dependencyPath := filepath.Join(chartPath, "charts", "dep")
	if err := os.MkdirAll(filepath.Join(dependencyPath, "templates"), 0700); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		filepath.Join(chartPath, "Chart.yaml"):                       "apiVersion: v2\nname: parent\nversion: 1.0.0\ndependencies:\n  - name: dep\n    version: 1.0.0\n",
		filepath.Join(dependencyPath, "Chart.yaml"):                  "apiVersion: v2\nname: dep\nversion: 1.0.0\n",
		filepath.Join(dependencyPath, "templates", "configmap.yaml"): "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: vendored\n",
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, update := range []bool{true, false} {
		output, err := RenderChart(chartPath, "test", nil, options.CmdOptions{UpdateDeps: update})
		if err != nil {
			t.Fatalf("vendored dependency with update=%t: %v", update, err)
		}
		if !strings.Contains(output, "name: vendored") {
			t.Fatalf("missing vendored dependency render: %s", output)
		}
	}
}

func TestIsHelmChart(t *testing.T) {
	testCases := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "Valid Helm chart from examples",
			path: "../../examples/helm/helloworld",
			want: true,
		},
		{
			name: "Kustomize directory (should be false)",
			path: "../../examples/kustomize/helloworld",
			want: false,
		},
		{
			name: "Non-existent directory",
			path: "testdata/does-not-exist",
			want: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := IsHelmChart(tc.path)
			if got != tc.want {
				t.Errorf("IsHelmChart(%q) = %v; want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestRenderChart(t *testing.T) {
	// Using our example helm chart
	chartPath := "../../examples/helm/helloworld"
	releaseName := "test-release"

	t.Run("Render with default values", func(t *testing.T) {
		valuesFiles := []string{}
		opts := options.CmdOptions{
			Debug:      false,
			UpdateDeps: false,
			Lint:       true,
		}

		output, err := RenderChart(chartPath, releaseName, valuesFiles, opts)
		if err != nil {
			t.Fatalf("RenderChart failed: %v", err)
		}

		if !strings.Contains(output, "kind: ConfigMap") {
			t.Errorf("Output missing expected content 'kind: ConfigMap'. Got:\n%s", output)
		}

		if !strings.Contains(output, "kind: Deployment") {
			t.Errorf("Output missing expected content 'kind: Deployment'. Got:\n%s", output)
		}

		if !strings.Contains(output, "kind: Service") {
			t.Errorf("Output missing expected content 'kind: Service'. Got:\n%s", output)
		}

		if !strings.Contains(output, "name: test-release-") {
			t.Errorf("Output missing expected release name 'test-release-'. Got:\n%s", output)
		}
	})

	t.Run("Render with override values", func(t *testing.T) {
		// Using dev values file
		valuesFile := "../../examples/helm/helloworld/values-dev.yaml"

		valuesFiles := []string{valuesFile}
		opts := options.CmdOptions{
			Debug:      false,
			UpdateDeps: false,
			Lint:       true,
		}

		output, err := RenderChart(chartPath, releaseName, valuesFiles, opts)
		if err != nil {
			t.Fatalf("RenderChart failed: %v", err)
		}

		// Checking for the .Values.image.tag change
		if !strings.Contains(output, "nginx:dev") {
			t.Errorf("Output missing expected nginx:dev. Got:\n%s", output)
		}

		if output == "" {
			t.Errorf("Rendered output was empty")
		}
	})

	t.Run("Render with override values and chart dependencies", func(t *testing.T) {
		// Using dev values file
		valuesFile := "../../examples/helm/helloworld/values-dev.yaml"

		valuesFiles := []string{valuesFile}
		opts := options.CmdOptions{
			Debug:      true,
			UpdateDeps: true,
			Lint:       true,
		}

		output, err := RenderChart(chartPath, releaseName, valuesFiles, opts)
		if err != nil {
			t.Fatalf("RenderChart failed: %v", err)
		}

		// Checking for the .Values.image.tag change
		if !strings.Contains(output, "nginx:dev") {
			t.Errorf("Output missing expected nginx:dev. Got:\n%s", output)
		}

		// Checking for the dep configMap change
		if !strings.Contains(output, "test-release-dep") {
			t.Errorf("Output missing expected test-release-dep. Got:\n%s", output)
		}

		if output == "" {
			t.Errorf("Rendered output was empty")
		}
	})
}
