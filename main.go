package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

const (
	githubAccessTokenEnvName = "GITHUB_ACCESS_TOKEN"
	githubAPIEndPointEnvName = "GITHUB_API_END_POINT"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Printf("usage: %s FROM_PR TO_BRANCH", os.Args[0])
		fmt.Printf("example: %s Shanon/SS/pull/12345 20181020", os.Args[0])
		os.Exit(1)
	}

	from := strings.SplitN(os.Args[1], "/", 4)
	owner := from[0]
	repo := from[1]
	prNo, err := strconv.Atoi(from[3])
	if owner == "" || repo == "" || prNo < 0 || err != nil {
		fmt.Printf("usage: %s FROM_PR TO_BRANCH\n", os.Args[0])
		fmt.Printf("example: %s Shanon/SS/pull/12345 20181020", os.Args[0])
		os.Exit(1)
	}

	to := os.Args[2]

	token := os.Getenv(githubAccessTokenEnvName)
	if token == "" {
		fmt.Printf("no valid github access token found\n")
		os.Exit(1)
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	ghc := github.NewClient(tc)
	endPoint := os.Getenv(githubAPIEndPointEnvName)
	if endPoint != "" {
		u, err := url.Parse(endPoint)
		if err == nil {
			ghc.BaseURL = u
		}
	}

	basePullRequest, _, err := ghc.PullRequests.Get(ctx, owner, repo, prNo)
	if err != nil {
		fmt.Printf("fetch base pull request %s failed, %v\n", os.Args[1], err)
		os.Exit(1)
	}

	commits, _, err := ghc.PullRequests.ListCommits(ctx, owner, repo, prNo, nil)
	if err != nil {
		fmt.Printf("fetch base pull requests commits failed, %v\n", err);
		os.Exit(1)
	}
	shas := make([]string, len(commits))
	for i, c := range commits {
		shas[i] = c.GetSHA()
	}


	prTitle := fmt.Sprintf("%s for %s", basePullRequest.GetTitle(), to)
	prBaseBranch := fmt.Sprintf("%s-%s", basePullRequest.GetHead().GetLabel(), to)
	prBody := fmt.Sprintf("duplicated PR for %s from %d", to, prNo)
	pull := &github.NewPullRequest{
		Title: github.String(prTitle),
		Head:  github.String(to),
		Base:  github.String(prBaseBranch),
		Body:  github.String(prBody),
	}

	_, _, err := ghc.PullRequests.Create(ctx, owner, repo, pull)
	if err != nil {
		fmt.Printf("create pr failed. %s", err)
		os.Exit(1)
	}

	fmt.Printf("publisher:[%s]:done\n", re.CurrentWorkDir())
}
