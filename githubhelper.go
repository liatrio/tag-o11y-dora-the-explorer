package main

import (
	"net/http"
	"net/url"

	"github.com/Khan/genqlient/graphql"
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
	graphqlUrl    string
	pat           string
	client        graphql.Client
	name          string
	org           string
	baseRefName   string
	remoteRepoUrl string
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
