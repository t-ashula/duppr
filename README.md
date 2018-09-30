# duppr
duppr dupplicate pull request for another branch

## installation

```
go get -u github.com/t-ashula/dupper
```

## usage

0. install git command
0. setup GitHub (or GitHub Enterprise)'s personal access token via https://github.com/settings/tokens
1. set the token to environment variable GITHUB_ACCESS_TOKEN
2. set API end point URL to environment variable GITHUB_API_END_POINT if you use GitHub Enterprise
3. run `duppr --from-pr Owner/Repo/pull/ID --target-branch another-branch`

## limitation

- if `git cherry-pick [pr's commits]` failed, no PR create
- if your token does not have write access to original PR's repository, no PR create
- ...

## license

MIT

## TODO

- tests
- refactoring