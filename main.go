package main

//go:generate .tools/genqlient genqlient.yaml

import (
	"context"
	"encoding/json"
	"os"

	"github.com/go-git/go-git/v5"
	gitHttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"go.uber.org/zap"
)

var (
	logger *zap.Logger
	ghrc   *GitHubRepoContext
)

func init() {
	var err error

	ghrc = &GitHubRepoContext{}

	rawJSON := []byte(`{
        "level": "debug",
        "encoding": "json",
        "outputPaths": ["stdout"],
        "errorOutputPaths": ["stderr"],
        "initialFields": {"service": "dora-the-explorer"},
        "encoderConfig": {
            "messageKey": "message",
            "levelKey": "level",
            "levelEncoder": "lowercase"
            }
        }
    `)

	var cfg zap.Config
	if err = json.Unmarshal(rawJSON, &cfg); err != nil {
		panic(err)
	}

	logger = zap.Must(cfg.Build())

	defer func() {
		if err := logger.Sync(); err != nil {
			return
		}
	}()
}

func main() {
	// Collect configuration from environment variables
	ghrc.pat = os.Getenv("GH_PAT")
	if ghrc.pat == "" {
		logger.Sugar().Errorf("GH_PAT is not set")
		return
	}

	ghrc.org = os.Getenv("GH_ORG")
	if ghrc.org == "" {
		logger.Sugar().Errorf("GH_ORG is not set")
		return
	}

	graphqlUrl := os.Getenv("GH_GRAPHQL_URL")
	if graphqlUrl == "" {
		graphqlUrl = "https://api.github.com/graphql"
	}

	ghrc.gitHubDomain = os.Getenv("GH_BASE_URL")
	if ghrc.gitHubDomain == "" {
		ghrc.gitHubDomain = "https://github.com"
	}

	ghrc.client = ghrc.generateClient(graphqlUrl)

	ghrc.name = os.Getenv("GH_REPO_NAME")
	if ghrc.name == "" {
		logger.Sugar().Errorf("GH_REPO_NAME is not set")
		return
	}

	var err error
	ghrc.remoteRepoUrl, err = ghrc.CalculateRepoUrl()
	if err != nil {
		logger.Sugar().Errorf("Error calculating repo URL: %s", err)
		return
	}

	// Create a temp directory and clone the repository
	dir, err := os.MkdirTemp("", "cloned-repo")
	if err != nil {
		logger.Sugar().Errorf("Error creating temp dir: %s", err)
		return
	}
	logger.Sugar().Infof("Temp dir is: %v", dir)

	defer os.RemoveAll(dir) // clean up

	//region Test Graphql client

	ctx := context.Background()
	var viewerResp *getViewerResponse
	viewerResp, err = getViewer(ctx, ghrc.client)

	if err != nil {
		logger.Sugar().Errorf("Error getting viewer: %s", err)
		return
	}
	logger.Sugar().Infof("Viewer name: %s", viewerResp.Viewer.MyName)
	//endregion

	// Clones the repository into the given dir, just as a normal git clone does
	repo, err := git.PlainClone(dir, false, &git.CloneOptions{
		// URL: "https://github.com/liatrio/dora-deploy-demo.git",
		URL: ghrc.remoteRepoUrl,
		Auth: &gitHttp.BasicAuth{
			Username: "dora-the-explorer",
			Password: ghrc.pat,
		},
	})

	if err != nil {
		logger.Sugar().Errorf("Error cloning repository: %s", err)
		return
	}

	head, err := repo.Head()
	if err != nil {
		logger.Sugar().Errorf("Error getting HEAD: %s", err)
		return
	}

	ghrc.baseRefName = head.Name().Short()

	branchName, err := GenerateDowngradeRemoteBranch(dir, repo, ghrc, logger)
	if err != nil {
		logger.Sugar().Errorf("Error generating downgrade branch: %s", err)
		return
	}

	repoId, err := getRepoId(ctx, ghrc.client, ghrc.org, ghrc.name)
	if err != nil {
		logger.Sugar().Errorf("Error getting repo ID: %s", err)
		return
	}

	prId, err := createPullRequest(ctx,
		ghrc.client,
		ghrc.baseRefName,
		"This is the body of the PR",
		branchName,
		"repoid",
		"Title of the PR")

	if err != nil {
		logger.Sugar().Errorf("Error creating PR: %s", err)
		return
	}

	logger.Sugar().Infof("Repo ID: %s", repoId)
	logger.Sugar().Infof("Created PR: %s", prId)

	logger.Sugar().Infof("Pushed to remote branch: %s", branchName)
}
