package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"go.uber.org/zap"
)

var (
	hclPath      = "envs/dev/terragrunt.hcl"
	reExpression = `(github\.com/liatrio/dora-lambda-tf-module-demo\?ref=)v0.6.2`
)

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
	dir string,
	repo *git.Repository,
	ghrc *GitHubRepoContext,
	logger *zap.Logger) (string, error) {

	// Create a new branch
	worktree, err := repo.Worktree()
	if err != nil {
		logger.Sugar().Errorf("Error getting worktree: %s", err)
		return "", err
	}

	epocMiliseconds := time.Now().UnixMilli()
	branchName := "version-downgrade-" + strconv.FormatInt(epocMiliseconds, 10)
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
	err = repo.Push(&git.PushOptions{
		RemoteName: "origin",
		Auth: &http.BasicAuth{
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
