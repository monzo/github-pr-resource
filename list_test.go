package resource_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/telia-oss/github-pr-resource"
	"github.com/telia-oss/github-pr-resource/fakes"
)

func listTestPRs() []*resource.PullRequest {
	open := githubv4.PullRequestStateOpen
	fork := createTestPR(3, "master", "anonymous", false, true, 0, nil, false, open)
	otherBase := createTestPR(4, "develop", "anonymous", false, false, 0, nil, false, open)
	second := createTestPR(2, "master", "anonymous", false, false, 0, nil, false, open)
	second.ChangedFiles = 7
	first := createTestPR(1, "master", "anonymous", false, false, 0, nil, false, open)
	first.ChangedFiles = 3
	return []*resource.PullRequest{fork, otherBase, second, first}
}

func listSource() resource.Source {
	return resource.Source{
		Repository:   "itsdalmo/test-repository",
		AccessToken:  "oauthtoken",
		List:         true,
		BaseBranch:   "master",
		DisableForks: true,
	}
}

func TestCheckList(t *testing.T) {
	wantList := `[{"number":1,"headRefOid":"oid1","changedFiles":3},{"number":2,"headRefOid":"oid2","changedFiles":7}]`
	digest := sha256.Sum256([]byte(wantList))

	github := new(fakes.FakeGithub)
	github.ListPullRequestsReturns(listTestPRs(), nil)
	output, err := resource.Check(resource.CheckRequest{Source: listSource()}, github)
	require.NoError(t, err)

	out, err := json.Marshal(output)
	require.NoError(t, err)
	assert.Equal(t, `[{"list":"`+hex.EncodeToString(digest[:])+`"}]`, string(out))
}

func TestGetList(t *testing.T) {
	github := new(fakes.FakeGithub)
	github.ListPullRequestsReturns(listTestPRs(), nil)
	checked, err := resource.Check(resource.CheckRequest{Source: listSource()}, github)
	require.NoError(t, err)

	dir := t.TempDir()
	output, err := resource.Get(resource.GetRequest{Source: listSource(), Version: checked[0]}, github, new(fakes.FakeGit), dir)
	require.NoError(t, err)
	assert.Equal(t, checked[0], output.Version)

	content, err := os.ReadFile(filepath.Join(dir, "prs.json"))
	require.NoError(t, err)
	assert.Equal(t, `[{"number":1,"headRefOid":"oid1","changedFiles":3},{"number":2,"headRefOid":"oid2","changedFiles":7}]`, string(content))
}
