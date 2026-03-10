package cmd

import (
	"context"
	"fmt"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	"github.com/google/go-github/v84/github"
	"golang.org/x/mod/semver"
)

// Semver regex for release version comparison
var semanticVersion = regexp.MustCompile(`^v(\d{1,3}\.?)+`)

// getVersion return the application version
func getVersion() string {
	buildInfo, ok := debug.ReadBuildInfo()
	version := buildInfo.Main.Version
	if !ok || version == "" {
		return "development"
	}

	// Grab the expected semver rdv version number
	// ex. v0.14.0
	sanitizedVersion := semanticVersion.FindString(version)

	return sanitizedVersion
}

// getLatest returns the latest available release published
// via the modules github repository. github.com/dlactin/rdv
// we expect a semver tag here ex. 0.14.0
func getLatest() (string, error) {
	owner, repo := getSourceRepo()
	client := github.NewClient(nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	latestRelease, _, err := client.Repositories.GetLatestRelease(ctx, owner, repo)
	if err != nil {
		return "", fmt.Errorf("failed to find latest release: %w", err)
	}
	return *latestRelease.TagName, nil
}

// getSourceRepo returns the owner and repository from build info
func getSourceRepo() (string, string) {
	buildInfo, _ := debug.ReadBuildInfo()

	module := buildInfo.Main.Path
	details := strings.Split(module, "/")

	owner := details[1]
	repo := details[2]

	return owner, repo
}

func updateRequired() (bool, string, error) {
	currentVersion := getVersion()
	if currentVersion == "" {
		return false, "", nil
	}

	latestVersion, err := getLatest()
	if err != nil {
		return false, "", err
	}

	updateMsg := fmt.Sprintf(`
Update available!

Run: go install github.com/dlactin/rdv@%s

`, latestVersion)

	// Use proper semver comparison instead of string comparison
	// semver.Compare returns -1 if v < w, 0 if v == w, +1 if v > w
	if semver.Compare(currentVersion, latestVersion) < 0 {
		return true, updateMsg, nil
	}

	return false, "", nil
}
