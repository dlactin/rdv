// Package git provides functions for setting up a temporary git work tree
package git

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

// SetupWorkTree creates a temporary git work tree we can use for checking out our references
func SetupWorkTree(repoRoot, gitRef string) (string, func(), error) {
	// Fetch from all remotes
	fetchCmd := exec.Command("git", "fetch", "--all")
	fetchCmd.Dir = repoRoot
	if output, err := fetchCmd.CombinedOutput(); err != nil {
		return "", nil, fmt.Errorf("failed to run 'git fetch --all': %w\nOutput: %s", err, string(output))
	}

	// Set up a Git Worktree for gitref
	tempDir, err := os.MkdirTemp("", "diff-ref-")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Combined worktree and tempdir cleanup
	// Returning this function to defer in rootCmd
	cleanup := func() {
		// Using --force to avoid errors if dir is already partially cleaned
		cleanupCmd := exec.Command("git", "worktree", "remove", "--force", tempDir)
		cleanupCmd.Dir = repoRoot
		if output, err := cleanupCmd.CombinedOutput(); err != nil {
			log.Printf("Warning: failed to run 'git worktree remove'. Manual cleanup may be required. Error: %v, Output: %s", err, string(output))
		}
		if err := os.RemoveAll(tempDir); err != nil {
			log.Printf("Warning: error removing temporary directory %s: %v", tempDir, err)
		}
	}

	// Create the worktree
	// Using -d to allow checking out a branch that is already checked out (like 'main')
	addCmd := exec.Command("git", "worktree", "add", "-d", tempDir, gitRef)
	addCmd.Dir = repoRoot
	if output, err := addCmd.CombinedOutput(); err != nil {
		return "", nil, fmt.Errorf("failed to create worktree for '%s': %w\nOutput: %s", gitRef, err, string(output))
	}

	return tempDir, cleanup, nil
}

// GetRepoRoot finds the top-level directory of the current git repository.
func GetRepoRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to find git repo root: %w. Make sure you are running this inside a git repository. Output: %s", err, string(output))
	}
	return strings.TrimSpace(string(output)), nil
}
