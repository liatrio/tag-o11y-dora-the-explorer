package main

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"go.uber.org/zap"
)

var (
	hclPath      = "envs/dev/terragrunt.hcl"
	reExpression = `(github\.com/liatrio/dora-lambda-tf-module-demo\?ref=)v0.6.2`
)

type authedTransport struct {
	key     string
	wrapped http.RoundTripper
}

func (t *authedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "bearer "+t.key)
	return t.wrapped.RoundTrip(req)
}

type GitHubRepoContext struct {
	gitHubDomain  string
	pat           string
	client        graphql.Client
	name          string
	org           string
	baseRefName   string
	remoteRepoUrl string
	localDir      string
	repo          *git.Repository
}

func (ghc *GitHubRepoContext) generateClient(url string) graphql.Client {
	httpClient := http.Client{
		Transport: &authedTransport{
			key:     ghc.pat,
			wrapped: http.DefaultTransport,
		},
	}
	return graphql.NewClient(url, &httpClient)
}

func (ghc *GitHubRepoContext) CalculateRepoUrl() (string, error) {
	url, err := url.JoinPath(ghc.gitHubDomain, ghc.org, ghc.name)
	if err != nil {
		return "", err
	}
	return url + ".git", nil
}

// If no deployments exist, return nil
func (ghrc *GitHubRepoContext) GetLastDeployment(ctx context.Context) (*getLatestDeploymentsRepositoryDeploymentsDeploymentConnectionNodesDeployment, error) {
	recentDeployments, err := getLatestDeployments(ctx, ghrc.client, ghrc.org, ghrc.name)
	if err != nil {
		logger.Sugar().Errorf("Error getting latest deployments for %s/%s: %s", ghrc.org, ghrc.name, err)
		return nil, err
	}

	if len(recentDeployments.Repository.Deployments.Nodes) == 0 {
		logger.Sugar().Infof("No deployments found for %s/%s", ghrc.org, ghrc.name)
		return nil, nil
	}

	// return the last deployment
	return &recentDeployments.Repository.Deployments.Nodes[len(recentDeployments.Repository.Deployments.Nodes)-1], nil
}

func NeedsDowngrade(dir string) (bool, error) {
	f := filepath.Join(dir, hclPath)
	bb, err := os.ReadFile(f)
	if err != nil {
		logger.Sugar().Errorf("Error reading file: %s", err)
		return false, err
	}

	re := regexp.MustCompile(reExpression)
	return re.Match(bb), nil
}

func GenerateDowngradeRemoteBranch(
	ghrc *GitHubRepoContext,
	logger *zap.Logger) (string, error) {

	// Create a new branch
	worktree, err := ghrc.repo.Worktree()
	if err != nil {
		logger.Sugar().Errorf("Error getting worktree: %s", err)
		return "", err
	}

	epochMilliseconds := time.Now().UnixMilli()
	branchName := "version-downgrade-" + strconv.FormatInt(epochMilliseconds, 10)
	newBranch := plumbing.NewBranchReferenceName(branchName)

	err = worktree.Checkout(&git.CheckoutOptions{
		Branch: newBranch,
		Create: true,
	})
	if err != nil {
		logger.Sugar().Errorf("Error creating new branch: %s", err)
		return "", err
	}

	// Make changes (if any)
	f := filepath.Join(ghrc.localDir, hclPath)
	bb, err := os.ReadFile(f)
	if err != nil {
		logger.Sugar().Errorf("Error reading file: %s", err)
		return "", err
	}

	re := regexp.MustCompile(reExpression)
	updatedContent := re.ReplaceAll(bb, []byte("${1}v0.3.0"))
	err = os.WriteFile(f, updatedContent, 0600)
	if err != nil {
		logger.Sugar().Errorf("Error writing to file: %s", err)
		return "", err
	}

	// Add the file to the staging area
	_, err = worktree.Add("envs/dev/terragrunt.hcl")
	if err != nil {
		logger.Sugar().Errorf("Error adding file to staging area: %s", err)
		return "", err
	}

	// Commit the changes
	_, err = worktree.Commit("Updated version in terragrunt.hcl", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Your Name",
			Email: "your.email@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		logger.Sugar().Errorf("Error committing changes: %s", err)
		return "", err
	}

	// Push the new branch to the remote repository
	err = ghrc.repo.Push(&git.PushOptions{
		RemoteName: "origin",
		Auth: &githttp.BasicAuth{
			Username: "trashpandas",
			Password: ghrc.pat,
		},
		RefSpecs: []config.RefSpec{
			config.RefSpec("refs/heads/" + branchName + ":refs/heads/" + branchName),
		},
	})
	if err != nil {
		logger.Sugar().Errorf("Error pushing to remote: %s", err)
		return "", err
	}

	return branchName, nil
}
