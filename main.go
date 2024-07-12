package main

//go:generate .tools/genqlient genqlient.yaml

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

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

func prepEnvironment() (ghrc *GitHubRepoContext, doraTeam *DoraTeam, err error) {
	ghrc = &GitHubRepoContext{}
	doraTeam = &DoraTeam{}
	doraTeamPerformanceLevel := strings.ToLower(os.Getenv("DORA_TEAM_PERFORMANCE_LEVEL"))
	if doraTeamPerformanceLevel == "" {
		return nil, nil, errors.New("DORA_TEAM_PERFORMANCE_LEVEL is not set")
	}

	switch doraTeamPerformanceLevel {
	case "elite":
		doraTeam = NewEliteDoraTeam()
	case "high":
		doraTeam = NewHighDoraTeam()
	case "medium":
		doraTeam = NewMediumDoraTeam()
	case "low":
		doraTeam = NewLowDoraTeam()
	default:
		logger.Sugar().Info("Unknown team performance level")
		return nil, nil, fmt.Errorf("Unknown team performance level: %s", doraTeamPerformanceLevel)
	}

	ghrc.pat = os.Getenv("GH_PAT")
	if ghrc.pat == "" {
		return nil, nil, errors.New("GH_PAT is not set")
	}

	ghrc.org = os.Getenv("GH_ORG")
	if ghrc.org == "" {
		return nil, nil, errors.New("GH_ORG is not set")
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
		return nil, nil, errors.New("GH_REPO_NAME is not set")
	}

	ghrc.remoteRepoUrl, err = ghrc.CalculateRepoUrl()
	if err != nil {
		return nil, nil, fmt.Errorf("Error calculating repo URL: %s", err)
	}

	return ghrc, doraTeam, nil
}

func main() {
	ctx := context.Background()
	ghrc, doraTeam, err := prepEnvironment()
	if err != nil {
		logger.Sugar().Errorf("Error preparing environment: %s", err)
		return
	}

	logger.Sugar().Infof("Dora team performance level: %s", doraTeam.Level)

	// Create a temp directory and clone the repository
	// dir, err := os.MkdirTemp("", "cloned-repo")
	// if err != nil {
	// 	logger.Sugar().Errorf("Error creating temp dir: %s", err)
	// 	return
	// }
	// logger.Sugar().Infof("Temp dir is: %v", dir)
	// ghrc.localDir = dir

	// defer os.RemoveAll(dir) // clean up

	// // Clones the repository into the given dir, just as a normal git clone does
	// repo, err := git.PlainClone(dir, false, &git.CloneOptions{
	// 	URL: ghrc.remoteRepoUrl,
	// 	Auth: &gitHttp.BasicAuth{
	// 		Username: "dora-the-explorer",
	// 		Password: ghrc.pat,
	// 	},
	// })

	// if err != nil {
	// 	logger.Sugar().Errorf("Error cloning repository: %s", err)
	// 	return
	// }
	// ghrc.repo = repo

	// head, err := repo.Head()
	// if err != nil {
	// 	logger.Sugar().Errorf("Error getting HEAD: %s", err)
	// 	return
	// }

	// ghrc.baseRefName = head.Name().Short()

	// Determine how often we need to generate events based on the DORA team
	// performance level.

	for {
		minutesToNextDeploy, err := doraTeam.MinutesUntilNextDeployment(ctx, ghrc)
		if err != nil {
			logger.Sugar().Errorf("Error calculating minutes until next deployment: %s", err)
			return
		}
		if minutesToNextDeploy != -1 {
			// Generate a deployment
			logger.Sugar().Infof("Minutes until next deployment: %d", minutesToNextDeploy)
			t := time.NewTicker(time.Duration(minutesToNextDeploy) * time.Minute)
			<-t.C // wait for the next deployment time

			logger.Sugar().Info("Creating deployment")
			pullRequest, err := ghrc.GeneratePullRequest(ctx, logger)
			if err != nil {
				logger.Sugar().Errorf("Error generating deployment: %s", err)
				return
			}

			// Wait for status checks to complete
			prNumber := pullRequest.CreatePullRequest.PullRequest.Number
			err = ghrc.WaitForStatusChecks(ctx, prNumber)
			if err != nil {
				logger.Sugar().Errorf("Error waiting for status checks: %s", err)
				return
			}

			// Merge the PR
			mergeResponse, err := mergePullRequest(ctx, ghrc.client, pullRequest.CreatePullRequest.PullRequest.Id)
			if err != nil {
				logger.Sugar().Errorf("Error merging PR: %s", err)
				return
			}

			logger.Sugar().Infof("Merged PR: %s", mergeResponse.MergePullRequest.PullRequest.Merged)

			logger.Sugar().Infof("Created PR: %s", pullRequest)
		} else {
			logger.Sugar().Info("Skipping deployment")
		}
		return
	}

	// needsDowngrade, err := NeedsDowngrade(dir)
	if err != nil {
		logger.Sugar().Errorf("Error checking for downgrade: %s", err)
		return
	}

	// if !needsDowngrade {
	// 	logger.Sugar().Info("No downgrade needed")
	// 	return
	// }

	// branchName, err := GenerateDowngradeRemoteBranch(ghrc, logger)
	if err != nil {
		logger.Sugar().Errorf("Error generating downgrade branch: %s", err)
		return
	}

	// repoIdResp, err := getRepoId(ctx, ghrc.client, ghrc.org, ghrc.name)
	// if err != nil {
	// 	logger.Sugar().Errorf("Error getting repo ID: %s", err)
	// 	return
	// }

	// prId, err := createPullRequest(ctx,
	// 	ghrc.client,
	// 	ghrc.baseRefName,
	// 	"This is the body of the PR",
	// 	branchName,
	// 	repoIdResp.Repository.Id,
	// 	"Title of the PR")

	// Merge PR

	if err != nil {
		logger.Sugar().Errorf("Error creating PR: %s", err)
		return
	}

	// logger.Sugar().Infof("Repo ID: %s", repoIdResp)
	// logger.Sugar().Infof("Created PR: %s", prId)

	// logger.Sugar().Infof("Pushed to remote branch: %s", branchName)
}
