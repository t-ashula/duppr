package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

const (
	githubAccessTokenEnvName = "GITHUB_ACCESS_TOKEN"
	githubAPIEndPointEnvName = "GITHUB_API_END_POINT"
)

var fromPR string
var targetBranch string

func init() {
	flag.StringVar(&fromPR, "from-pr", "", "pull request id duplicate from like t-ashula/duppr/pull/123")
	flag.StringVar(&targetBranch, "target-branch", "", "target branch name")
}

func usage() {
	flag.PrintDefaults()
}

type prInfo struct {
	Owner      string
	Repository string
	Number     int
}

func main() {
	flag.Parse()

	if fromPR == "" || targetBranch == "" {
		usage()
		os.Exit(1)
	}
	prInfo, err := parseFromPR(fromPR)
	if err != nil {
		fmt.Printf("parse from pr failed, %v\n", err)
		usage()
		os.Exit(1)
	}

	ctx := context.Background()
	ghc, err := githubClient(ctx)
	if err != nil {
		fmt.Printf("init github client failed, %v\n", err)
		os.Exit(1)
	}

	basePR, err := getGithubPullRequest(ctx, ghc, prInfo)
	if err != nil {
		fmt.Printf("fetch pull request %s failed, %v\n", fromPR, err)
		os.Exit(1)
	}

	commits, err := getGithubPullRequestCommits(ctx, ghc, prInfo)
	if err != nil {
		fmt.Printf("fetch base pull requests commits failed, %v\n", err)
		os.Exit(1)
	}

	shas := make([]string, len(commits))
	for i, c := range commits {
		shas[i] = c.GetSHA()
	}

	requestBranch, err := prepareRepository(ctx, basePR, targetBranch, shas)
	if err != nil {
		fmt.Printf("clone target repository failed, %v\n", err)
		os.Exit(1)
	}

	newPR := makeDuppedPR(basePR, targetBranch, requestBranch)
	created, err := postDuppedPR(ctx, ghc, prInfo, newPR)
	if err != nil {
		fmt.Printf("post dupped pull request failed, %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("pull request dupplication suucess, %v\n", created)
}

func parseFromPR(prID string) (*prInfo, error) {
	parts := strings.SplitN(prID, "/", 4)
	owner := parts[0]
	repository := parts[1]
	prNo, err := strconv.Atoi(parts[3])

	if owner == "" || repository == "" || prNo < 0 || err != nil {
		if err != nil {
			log.Printf("atoi failed, %v", err)
		}
		return nil, fmt.Errorf("pull request id (%s) is invalid", prID)
	}

	return &prInfo{Owner: owner, Repository: repository, Number: prNo}, nil
}

func getGithubPullRequest(ctx context.Context, ghc *github.Client, target *prInfo) (*github.PullRequest, error) {
	pr, res, err := ghc.PullRequests.Get(ctx, target.Owner, target.Repository, target.Number)
	if err != nil {
		log.Printf("get PR failed, error: %v, response: %v", err, res)
		return nil, err
	}
	return pr, nil
}

func getGithubPullRequestCommits(ctx context.Context, ghc *github.Client, target *prInfo) ([]*github.RepositoryCommit, error) {
	commits, res, err := ghc.PullRequests.ListCommits(ctx, target.Owner, target.Repository, target.Number, nil)
	if err != nil {
		log.Printf("get PRCommit failed, error: %v, response: %v", err, res)
		return nil, err
	}
	return commits, nil
}

func githubClient(ctx context.Context) (*github.Client, error) {
	token := os.Getenv(githubAccessTokenEnvName)
	if token == "" {
		return nil, fmt.Errorf("no valid github access token found")
	}

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
	return ghc, nil
}

func prepareRepository(ctx context.Context, pr *github.PullRequest, targetBranch string, shas []string) (string, error) {
	baseBranch := pr.GetBase()
	baseRepositoryURL := baseBranch.GetRepo().GetCloneURL()
	cloneOptions := &git.CloneOptions{URL: baseRepositoryURL}
	tempDir, err := ioutil.TempDir("", "duppr")
	if err != nil {
		log.Printf("clone target repository failed, %v", err)
		return "", err
	}

	defer os.RemoveAll(tempDir)

	r, err := git.PlainClone(tempDir, false, cloneOptions)
	if err != nil {
		log.Printf("clone target repository failed, %v", err)
		return "", err
	}

	headBranch := pr.GetHead()
	sameRemote := baseBranch.GetRepo().GetFullName() == headBranch.GetRepo().GetFullName()
	var remote *git.Remote
	if sameRemote {
		// do nothing
	} else {
		remoteName := "pr"
		headRepositoryURL := headBranch.GetRepo().GetCloneURL()
		remote, err = r.CreateRemote(&config.RemoteConfig{Name: remoteName, URLs: []string{headRepositoryURL}})
		if err != nil {
			log.Printf("clone pr-head repository failed, %v", err)
			return "", err
		}

		err = remote.Fetch(&git.FetchOptions{RemoteName: remoteName})
		if err != nil {
			log.Printf("fetch remote failed, %v", err)
			return "", err
		}
	}
	w, err := r.Worktree()
	if err != nil {
		log.Printf("worktree failed,%v", err)
		return "", err
	}
	err = w.Checkout(&git.CheckoutOptions{Branch: plumbing.ReferenceName(targetBranch)})
	if err != nil {
		log.Printf("checkout target branch faield, %v", err)
		return "", err
	}

	// create branch for new-pr
	requestBranch := fmt.Sprintf("%s-for-%s", headBranch.GetRef(), targetBranch)
	err = w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.ReferenceName(requestBranch),
		Create: true,
	})
	if err != nil {
		log.Printf("create branch failed, %v", err)
		return "", err
	}

	// go-git not support cherry-pick yet
	{
		cd, err := os.Getwd()
		if err != nil {
			log.Printf("get workdir failed, %v", err)
			return "", nil
		}
		defer os.Chdir(cd)

		err = os.Chdir(tempDir)
		if err != nil {
			log.Printf("change workdir failed, %v", err)
			return "", nil
		}
		for _, sha := range shas {
			args := []string{"cherry-pick", sha}
			cmd := exec.Command("git", args...)
			err = cmd.Run()
			if err != nil {
				log.Printf("cherry pick %s failed, %v\n", sha, err)
				return "", err
			}
		}
	}

	pushOptions := &git.PushOptions{RemoteName: requestBranch}
	if sameRemote {
		err = r.PushContext(ctx, pushOptions)
	} else {
		err = remote.PushContext(ctx, pushOptions)
	}
	if err != nil {
		log.Printf("push request branch failed, %v", err)
		return "", err
	}

	return requestBranch, nil
}

func makeDuppedPR(basePR *github.PullRequest, targetBranch string, requestBranch string) *github.NewPullRequest {
	title := fmt.Sprintf("%s for %s", basePR.GetTitle(), targetBranch)
	body := fmt.Sprintf("duplicated PR for %s from #%d\n", targetBranch, basePR.GetNumber())
	user := basePR.GetHead().GetRepo().GetOwner().GetLogin()
	head := fmt.Sprintf("%s:%s", user, requestBranch)
	pull := &github.NewPullRequest{
		Title: github.String(title),
		Head:  github.String(head),
		Base:  github.String(targetBranch),
		Body:  github.String(body),
	}
	return pull
}

func postDuppedPR(ctx context.Context, ghc *github.Client, basePR *prInfo, duppedPR *github.NewPullRequest) (*github.PullRequest, error) {
	pr, res, err := ghc.PullRequests.Create(ctx, basePR.Owner, basePR.Repository, duppedPR)
	if err != nil {
		log.Printf("create pull request failed, response: %v, error: %v", res, err)
		return nil, fmt.Errorf("create pr failed, %v", err)
	}
	return pr, nil
}
