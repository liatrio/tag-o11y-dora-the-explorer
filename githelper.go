package main

import (
	"context"
	"errors"
	"fmt"
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
	hclPath              = "envs/dev/terragrunt.hcl"
	reExpression         = `(github\.com/liatrio/dora-lambda-tf-module-demo\?ref=)v\d+\.\d+\.\d+`
	upToDateReExpression = `(github\.com/liatrio/dora-lambda-tf-module-demo\?ref=)v0.6.2`
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
	remoteRepoUrl string
	logger        *zap.Logger
	// localDir      string
	// repo          *git.Repository
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

// This function will wait for up to 10 minutes for the deployment to complete
func (ghrc *GitHubRepoContext) WaitForDeployment(ctx context.Context, sha string) error {
	logger.Sugar().Infof("Waiting for Deploy workflow to complete for %s", sha)
	timeout := time.After(10 * time.Minute)
	tick := time.Tick(10 * time.Second)

	for {
		select {
		case <-timeout:
			return errors.New("Timed out after 10 minutes waiting for deployment")
		case <-tick:
			commitGitHubActionRuns, err := getCommitGitHubActionsRuns(ctx, ghrc.client, ghrc.org, ghrc.name, sha)
			if err != nil {
				return err
			}

			if commit, ok := commitGitHubActionRuns.Repository.Object.(*getCommitGitHubActionsRunsRepositoryObjectCommit); ok {
				for _, node := range commit.StatusCheckRollup.Contexts.Nodes {
					if cr, ok := node.(*GitHubActionCheckRun); ok {
						if cr.Name != "deploy" {
							continue
						}
						switch cr.Status {
						case "COMPLETED":
							if cr.Conclusion == "SUCCESS" {
								return nil
							} else {
								return errors.New("Deployment failed")
							}
						case "IN_PROGRESS":
							continue
						case "QUEUED":
							continue
						case "REQUESTED":
							continue
						default:
							return fmt.Errorf("Unknown deployment state: %s", cr.Status)
						}
					}
				}
			} else {
				return errors.New("Error getting commit")
			}

			// switch deploymentAction {
			// // case "SUCCESS":
			// // 	return nil
			// // case "FAILURE":
			// // 	return errors.New("Deployment failed")
			// // case "PENDING":
			// // 	continue
			// // case "ERROR":
			// // 	return errors.New("Deployment errored")
			// // case "EXPECTED":
			// // 	continue
			// default:
			// 	return fmt.Errorf("Unknown deployment state: %s", deploymentAction)
			// }
		}
	}
}

// This function will generate a deployment for the given repository
// Each call it will clone the remote repository to a temp directory.
// It will create a  branch, make a change, commit the change, and push the
// branch to the remote. Create a Pull Request and Merge it.
// Workflows will then run to create a Deployment.
func (ghrc *GitHubRepoContext) GeneratePullRequest(ctx context.Context, logger *zap.Logger) (prId *createPullRequestResponse, err error) {
	// Create a temp directory and clone the repository
	dir, err := os.MkdirTemp("", "cloned-repo")
	if err != nil {
		logger.Sugar().Errorf("Error creating temp dir: %s", err)
		return
	}
	logger.Sugar().Infof("Temp dir is: %v", dir)

	defer os.RemoveAll(dir) // clean up

	// Clones the repository into the given dir, just as a normal git clone does
	repo, err := git.PlainClone(dir, false, &git.CloneOptions{
		URL: ghrc.remoteRepoUrl,
		Auth: &githttp.BasicAuth{
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

	baseRefName := head.Name().Short()

	// Generate a remote branch with a change to the repo
	branchName, err := GenerateChangeRemoteBranch(dir, ghrc, repo, logger)

	// Create a Pull Request
	repoIdResp, err := getRepoId(ctx, ghrc.client, ghrc.org, ghrc.name)
	if err != nil {
		logger.Sugar().Errorf("Error getting repo ID: %s", err)
		return
	}

	prId, err = createPullRequest(ctx,
		ghrc.client,
		baseRefName,
		"Generated by Dora the Explorer",
		branchName,
		repoIdResp.Repository.Id,
		"fix: Change app version")

	logger.Sugar().Infof("Created PR: %d", prId.CreatePullRequest.PullRequest.Number)

	return prId, err
}

func (ghrc *GitHubRepoContext) UpdateBaseBranch() {

}

// This function will wait for up to 10 min for the status checks to complete
func (ghrc *GitHubRepoContext) WaitForStatusChecks(ctx context.Context, prNumber int) error {
	logger.Sugar().Infof("Waiting for status checks for PR %d", prNumber)
	timeout := time.After(10 * time.Minute)
	tick := time.Tick(10 * time.Second)

	for {
		select {
		case <-timeout:
			return errors.New("Timed out after 10 minutes waiting for status checks")
		case <-tick:
			pr, err := getPullRequestStatusCheckRollup(ctx,
				ghrc.client,
				ghrc.org,
				ghrc.name,
				prNumber)

			if err != nil {
				return err
			}

			switch pr.Repository.PullRequest.StatusCheckRollup.State {
			case "SUCCESS":
				return nil
			case "FAILURE":
				return fmt.Errorf("PR %d failed status checks", prNumber)
			case "PENDING":
				continue
			case "ERROR":
				return fmt.Errorf("PR %d errored status checks", prNumber)
			case "EXPECTED":
				continue
			default:
				return fmt.Errorf("Unknown status check state: %s", pr.Repository.PullRequest.StatusCheckRollup.State)
			}

		}
	}
}

func NeedsDowngrade(dir string) (bool, error) {
	f := filepath.Join(dir, hclPath)
	bb, err := os.ReadFile(f)
	if err != nil {
		logger.Sugar().Errorf("Error reading file: %s", err)
		return false, err
	}

	re := regexp.MustCompile(upToDateReExpression)
	return re.Match(bb), nil
}

func GenerateChangeRemoteBranch(
	dir string,
	ghrc *GitHubRepoContext,
	repo *git.Repository,
	logger *zap.Logger) (string, error) {

	// Create a new branch
	worktree, err := repo.Worktree()
	if err != nil {
		logger.Sugar().Errorf("Error getting worktree: %s", err)
		return "", err
	}

	epochMilliseconds := time.Now().UnixMilli()
	branchName := "dora-the-explorer-" + strconv.FormatInt(epochMilliseconds, 10)
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
	f := filepath.Join(dir, hclPath)
	bb, err := os.ReadFile(f)
	if err != nil {
		logger.Sugar().Errorf("Error reading file: %s", err)
		return "", err
	}

	changeString := "v0.6.2"
	if needsDowngrade, err := NeedsDowngrade(dir); err == nil && needsDowngrade {
		changeString = "v0.3.0"
	} else if err != nil {
		logger.Sugar().Errorf("Error checking for downgrade: %s", err)
		return "", err
	}

	re := regexp.MustCompile(reExpression)
	updatedContent := re.ReplaceAll(bb, []byte("${1}"+changeString))
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
			Name:  "Bill Murray",
			Email: "ghostbuster-bill@hookandladder8.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		logger.Sugar().Errorf("Error committing changes: %s", err)
		return "", err
	}

	// Push the new branch to the remote repository
	err = repo.Push(&git.PushOptions{
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
