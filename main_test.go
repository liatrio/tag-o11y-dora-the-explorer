package main

import (
	"os"
	"testing"
)

func TestPrepEnvironmentSuccessful(t *testing.T) {
	// Set up environment variables
	os.Setenv("GH_PAT", "test-pat")
	os.Setenv("GH_ORG", "test-org")
	os.Setenv("GH_GRAPHQL_URL", "test-graphql-url")
	os.Setenv("GH_BASE_URL", "test-base-url")
	os.Setenv("GH_REPO_NAME", "test-repo-name")
	os.Setenv("DORA_TEAM_PERFORMANCE_LEVEL", "elite")

	defer func() {
		os.Unsetenv("GH_PAT")
		os.Unsetenv("GH_ORG")
		os.Unsetenv("GH_GRAPHQL_URL")
		os.Unsetenv("GH_BASE_URL")
		os.Unsetenv("GH_REPO_NAME")
	}()

	ghrc, doraTeam, err := prepEnvironment()
	if err != nil {
		t.Errorf("Error preparing: %s", err)
	}

	// assert values in ghrc
	if ghrc.pat != "test-pat" {
		t.Errorf("Expected pat to be test-pat, got %s", ghrc.pat)
	}
	if ghrc.org != "test-org" {
		t.Errorf("Expected org to be test-org, got %s", ghrc.org)
	}
	if ghrc.client == nil {
		t.Errorf("Expected client to be non-nil")
	}
	if ghrc.gitHubDomain != "test-base-url" {
		t.Errorf("Expected gitHubDomain to be test-base-url, got %s", ghrc.gitHubDomain)
	}
	if ghrc.name != "test-repo-name" {
		t.Errorf("Expected name to be test-repo-name, got %s", ghrc.name)
	}
	if doraTeam.Level != "Elite" {
		t.Errorf("Expected doraTeam.Level to be Elite, got %s", doraTeam.Level)
	}
}
