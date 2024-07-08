package main

//go:generate .tools/genqlient genqlient.yaml

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/go-git/go-git/v5"
	gitHttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"go.uber.org/zap"
)

var (
	logger *zap.Logger
)

func init() {
	var err error

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

func prepare() (*GitHubRepoContext, error) {
	ghrc := &GitHubRepoContext{}
	// Collect configuration from environment variables
	ghrc.pat = os.Getenv("GH_PAT")
	if ghrc.pat == "" {
		return nil, errors.New("GH_PAT is not set")
	}

	ghrc.org = os.Getenv("GH_ORG")
	if ghrc.org == "" {
		return nil, errors.New("GH_ORG is not set")
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
		return nil, errors.New("GH_REPO_NAME is not set")
	}

	var err error
	ghrc.remoteRepoUrl, err = ghrc.CalculateRepoUrl()
	if err != nil {
		return nil, fmt.Errorf("Error calculating repo URL: %s", err)
	}

	return ghrc, nil
}

func main() {
	ghrc, err := prepare()
	if err != nil {
		logger.Sugar().Errorf("Error preparing: %s", err)
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

	// infinte loop

	needsDowngrade, err := NeedsDowngrade(dir)
	if err != nil {
		logger.Sugar().Errorf("Error checking for downgrade: %s", err)
		return
	}

	if !needsDowngrade {
		logger.Sugar().Info("No downgrade needed")
		return
	}

	branchName, err := GenerateDowngradeRemoteBranch(dir, repo, ghrc, logger)
	if err != nil {
		logger.Sugar().Errorf("Error generating downgrade branch: %s", err)
		return
	}

	repoIdResp, err := getRepoId(ctx, ghrc.client, ghrc.org, ghrc.name)
	if err != nil {
		logger.Sugar().Errorf("Error getting repo ID: %s", err)
		return
	}

	prId, err := createPullRequest(ctx,
		ghrc.client,
		ghrc.baseRefName,
		"This is the body of the PR",
		branchName,
		repoIdResp.Repository.Id,
		"Title of the PR")

	// Merge PR

	if err != nil {
		logger.Sugar().Errorf("Error creating PR: %s", err)
		return
	}

	logger.Sugar().Infof("Repo ID: %s", repoIdResp)
	logger.Sugar().Infof("Created PR: %s", prId)

	logger.Sugar().Infof("Pushed to remote branch: %s", branchName)
}
