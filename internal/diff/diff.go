// Package diff provides functions for comparing rendered manifests
package diff

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dlactin/rdv/internal/helm"
	"github.com/dlactin/rdv/internal/kustomize"
	"github.com/dlactin/rdv/internal/options"
	"github.com/gonvenience/bunt"
	"github.com/gonvenience/ytbx"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"github.com/homeport/dyff/pkg/dyff"
	"go.yaml.in/yaml/v3"
)

// ANSI codes for diff colors
const (
	colorRed   = "\033[31m"
	colorGreen = "\033[32m"
	colorCyan  = "\033[36m"
	colorReset = "\033[0m"
)

// RenderManifests will render a Helm Chart or build a Kustomization
// and return the rendered manifests as a string
func RenderManifests(path string, values []string, opts options.CmdOptions) (string, error) {
	var renderedManifests string
	var err error

	if helm.IsHelmChart(path) {
		renderedManifests, err = helm.RenderChart(path, "release", values, opts)
		if err != nil {
			return "", fmt.Errorf("failed to render target Chart: '%w'", err)
		}
		return renderedManifests, nil
	} else if kustomize.IsKustomize(path) {
		renderedManifests, err = kustomize.RenderKustomization(path)
		if err != nil {
			return "", fmt.Errorf("failed to build target Kustomization: '%w'", err)
		}
		return renderedManifests, nil
	}

	return "", fmt.Errorf("path: %s is not a valid Helm Chart or Kustomization", path)
}

// CreateDiff generates a unified diff string between two text inputs.
func CreateDiff(a, b string, fromName, toName string) string {
	edits := myers.ComputeEdits(span.URI(fromName), a, b)
	diff := gotextdiff.ToUnified(fromName, toName, a, edits)

	return fmt.Sprint(diff)
}

// ColorizeDiff adds simple ANSI colors to a diff string.
func ColorizeDiff(diff string, noColor bool) string {
	if noColor {
		return diff
	}
	var coloredDiff strings.Builder
	lines := strings.Split(diff, "\n")

	for _, line := range lines {
		switch {
		// Standard unified diff lines
		case strings.HasPrefix(line, "+"):
			coloredDiff.WriteString(colorGreen + line + colorReset + "\n")
		case strings.HasPrefix(line, "-"):
			coloredDiff.WriteString(colorRed + line + colorReset + "\n")
		case strings.HasPrefix(line, "@@"):
			coloredDiff.WriteString(colorCyan + line + colorReset + "\n")
		// --- and +++ are headers, no special color
		case strings.HasPrefix(line, "---"), strings.HasPrefix(line, "+++"):
			coloredDiff.WriteString(line + "\n")
		// Default (context lines, start with a space)
		default:
			coloredDiff.WriteString(line + "\n")
		}
	}

	return coloredDiff.String()
}

// CreateSemanticDiff uses a more complex but k8s object aware diff engine
// it is better suited for larger scale changes to a k8s resources
func CreateSemanticDiff(targetRender, localRender, fromName, toName string, noColor bool) (*dyff.HumanReport, error) {
	// dyff is using bunt for text colouring
	// noColor flag & writing to a file turns colours off
	// defaults to ON or AUTO if we get an error
	fi, err := os.Stdout.Stat()
	switch {
	case noColor:
		bunt.SetColorSettings(bunt.OFF, bunt.OFF)
	case fi.Mode().IsRegular():
		bunt.SetColorSettings(bunt.OFF, bunt.OFF)
	case err != nil:
		bunt.SetColorSettings(bunt.AUTO, bunt.AUTO)
	default:
		bunt.SetColorSettings(bunt.ON, bunt.ON)
	}

	localRenderFile, err := createInputFileFromString(localRender, toName)
	if err != nil {
		return nil, fmt.Errorf("failed to parse local render for semantic diff: %w", err)
	}

	targetRenderFile, err := createInputFileFromString(targetRender, fromName)
	if err != nil {
		return nil, fmt.Errorf("failed to parse target render for semantic diff: %w", err)
	}

	options := []dyff.CompareOption{
		dyff.IgnoreOrderChanges(true),
		dyff.KubernetesEntityDetection(true),
		dyff.DetectRenames(true),
		dyff.IgnoreWhitespaceChanges(true),
	}

	diff, err := dyff.CompareInputFiles(targetRenderFile, localRenderFile, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to compare manifests: %w", err)
	}

	// Create our human readable report from our diffs
	report := dyff.HumanReport{
		Report:          diff,
		OmitHeader:      true,
		UseGoPatchPaths: true,
	}

	return &report, nil
}

// createInputFileFromString parses a multi-document YAML string into a dyff compatible InputFile format
func createInputFileFromString(content string, location string) (ytbx.InputFile, error) {
	var docs []*yaml.Node
	decoder := yaml.NewDecoder(strings.NewReader(content))

	for {
		var node yaml.Node
		if err := decoder.Decode(&node); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return ytbx.InputFile{}, fmt.Errorf("failed to decode YAML from %s: %w", location, err)
		}
		docs = append(docs, &node)
	}

	return ytbx.InputFile{
		Location:  location,
		Documents: docs,
	}, nil
}

// getDocumentName extracts the name from a Diff path
// It uses the RootDescription which contains the K8s resource identifier
func getDocumentNameFromDiff(diff dyff.Diff) string {
	// The Path.RootDescription() contains the K8s resource identifier
	// Example: "apps/v1/Deployment/helloworld"
	desc := diff.Path.RootDescription()

	if desc != "" {
		// Remove parentheses if present: "(apps/v1/Deployment/helloworld)" -> "apps/v1/Deployment/helloworld"
		return strings.Trim(desc, "()")
	}

	return "unknown"
}

// sortedMapValues returns the values from a map[int]string sorted by key
func sortedMapValues(m map[int]string) []string {
	// Get keys and sort them
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	// Simple insertion sort since we expect small numbers of documents
	for i := 1; i < len(keys); i++ {
		key := keys[i]
		j := i - 1
		for j >= 0 && keys[j] > key {
			keys[j+1] = keys[j]
			j--
		}
		keys[j+1] = key
	}

	// Build result array
	result := make([]string, len(keys))
	for i, k := range keys {
		result[i] = m[k]
	}
	return result
}

// PrintChangeSummary prints a concise summary of changes categorized by type
func PrintChangeSummary(report dyff.Report, isGitHub bool) error {
	added, removed, modified := categorizeDiffs(report.Diffs)

	addedCount := len(added)
	removedCount := len(removed)
	modifiedCount := len(modified)
	totalObjects := addedCount + removedCount + modifiedCount

	var parts []string
	if modifiedCount > 0 {
		parts = append(parts, fmt.Sprintf("%d updated", modifiedCount))
	}
	if addedCount > 0 {
		parts = append(parts, fmt.Sprintf("%d added", addedCount))
	}
	if removedCount > 0 {
		parts = append(parts, fmt.Sprintf("%d removed", removedCount))
	}

	if len(parts) == 0 {
		return nil
	}

	changeStr := "change"
	if totalObjects != 1 {
		changeStr = "changes"
	}

	summaryFormat := "\nSummary: %d %s (%s)\n"
	if isGitHub {
		summaryFormat = "**Summary: %d %s (%s)**\n"
	}

	fmt.Printf(strings.TrimRight(summaryFormat, " \t\r\n")+"\n",
		totalObjects, changeStr, strings.Join(parts, ", "))

	printDetailedLists(modified, added, removed, isGitHub)

	return nil
}

func categorizeDiffs(diffs []dyff.Diff) (added, removed, modified map[int]string) {
	added = make(map[int]string)
	removed = make(map[int]string)
	modified = make(map[int]string)

	for _, diff := range diffs {
		docIdx := diff.Path.DocumentIdx
		docName := getDocumentNameFromDiff(diff)
		isDocumentLevel := len(diff.Path.PathElements) == 0

		for _, detail := range diff.Details {
			switch detail.Kind {
			case dyff.ADDITION:
				if isDocumentLevel || detail.From == nil {
					added[docIdx] = docName
				} else {
					modified[docIdx] = docName
				}
			case dyff.REMOVAL:
				if isDocumentLevel || detail.To == nil {
					removed[docIdx] = docName
				} else {
					modified[docIdx] = docName
				}
			case dyff.MODIFICATION, dyff.ORDERCHANGE:
				modified[docIdx] = docName
			}
		}
	}

	for docIdx := range modified {
		delete(added, docIdx)
		delete(removed, docIdx)
	}

	return added, removed, modified
}

func printDetailedLists(modified, added, removed map[int]string, isGitHub bool) {
	listMarker := "  -"
	categoryMarker := ""
	if isGitHub {
		listMarker = "    -"
		categoryMarker = "*"
	}

	categories := []struct {
		title string
		m     map[int]string
	}{
		{"Updated:", modified},
		{"Added:", added},
		{"Removed:", removed},
	}

	for _, cat := range categories {
		if len(cat.m) > 0 {
			fmt.Printf("\n%s%s\n", categoryMarker, cat.title)
			for _, id := range sortedMapValues(cat.m) {
				if isGitHub {
					fmt.Printf("%s `%s`\n", listMarker, id)
				} else {
					fmt.Printf("%s %s\n", listMarker, id)
				}
			}
		}
	}
}

// FixGitHubDiffOutput post-processes the dyff report output to ensure that
// document-level changes have the necessary +/- symbols for GitHub's diff
// syntax highlighting. dyff's "human" report format normally omits these
// symbols for the contents of added/removed documents.
func FixGitHubDiffOutput(report string) string {
	report = strings.TrimSpace(report)
	lines := strings.Split(report, "\n")
	var result []string
	prefix := ""

	for _, line := range lines {
		// Trim trailing whitespace from the line before processing
		line = strings.TrimRight(line, " \t\r")
		trimmed := strings.TrimSpace(line)

		// Block headers start a new section, stop prefixing
		// dyff headers for field changes start with / (path) or ± (change type)
		if strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, "±") {
			prefix = ""
		}

		// Detect document status lines
		// We use HasSuffix because there might be some leading indentation
		isRemovalHeader := strings.HasSuffix(trimmed, "one document removed:")
		isAdditionHeader := strings.HasSuffix(trimmed, "one document added:")

		if isRemovalHeader {
			prefix = "-"
			continue
		} else if isAdditionHeader {
			prefix = "+"
			continue
		}

		// If we are in a document-level change block, prefix the line
		// but skip prefixing the header itself to avoid "-- one document removed:"
		if prefix != "" && trimmed != "" {
			// Prefix every line including blank ones to maintain the block
			result = append(result, prefix+line)
		} else {
			// Collapse multiple consecutive blank lines
			if trimmed == "" && len(result) > 0 && result[len(result)-1] == "" {
				continue
			}
			result = append(result, line)
		}
	}

	return strings.TrimSpace(strings.Join(result, "\n"))
}
