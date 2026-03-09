package cmd

import (
	"context"
	"fmt"
	"regexp"
	"runtime/debug"
	"strings"

	"github.com/google/go-github/v84/github"
)

// getVersion return the application version
func getVersion() string {
	buildInfo, ok := debug.ReadBuildInfo()
	version := buildInfo.Main.Version
	if !ok || version == "" {
		return "development"
	}

	// Semver regex for release version comparison
	semver, err := regexp.Compile(`^v(\d{1,3}\.?)+`)
	if err != nil {
		// return standard version by default if regex fails
		return version
	}

	// Grab the expected semver rdv version number
	// ex. v0.14.0
	sanitizedVersion := semver.FindString(version)

	return sanitizedVersion
}

// getLatest returns the latest available release published
// via the modules github repository. github.com/dlactin/rdv
// we expect a semver tag here ex. 0.14.0
func getLatest() (string, error) {
	owner, repo := getSourceRepo()
	client := github.NewClient(nil)

	latestRelease, _, err := client.Repositories.GetLatestRelease(context.Background(), owner, repo)
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
	latestVersion, err := getLatest()
	if err != nil {
		return false, "", err
	}

	updateMsg := fmt.Sprintf(`
Update available!

Run: go install github.com/dlactin/rdv@%s

`, latestVersion)

	if currentVersion < latestVersion {
		return true, updateMsg, nil
	}

	return false, "", nil
}
