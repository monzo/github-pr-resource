package resource

import (
	"io/ioutil"
	"net/http"
	"net/http/cgi"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitClientDoesNotPersistAccessToken(t *testing.T) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not installed")
	}

	askpass, err := filepath.Abs(filepath.Join("scripts", "askpass.sh"))
	require.NoError(t, err)
	previous := askpassPath
	askpassPath = askpass
	t.Cleanup(func() { askpassPath = previous })
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")

	const token = "test-access-token"
	root := t.TempDir()
	seed := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "master", seed},
		{"-C", seed, "-c", "user.name=test", "-c", "user.email=test@local", "commit", "-q", "--allow-empty", "-m", "init"},
		{"clone", "-q", "--bare", seed, filepath.Join(root, "repo.git")},
	} {
		out, err := exec.Command(gitPath, args...).CombinedOutput()
		require.NoError(t, err, string(out))
	}

	backend := &cgi.Handler{
		Path: gitPath,
		Args: []string{"http-backend"},
		Env:  []string{"GIT_PROJECT_ROOT=" + root, "GIT_HTTP_EXPORT_ALL=1"},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user, pass, ok := r.BasicAuth(); !ok || user != "x-oauth-basic" || pass != token {
			w.Header().Set("WWW-Authenticate", `Basic realm="test"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		backend.ServeHTTP(w, r)
	}))
	defer server.Close()
	repoURL := server.URL + "/repo.git"

	dir := t.TempDir()
	git, err := NewGitClient(&Source{AccessToken: token}, dir, ioutil.Discard)
	require.NoError(t, err)
	require.NoError(t, git.Init("master"))
	require.NoError(t, git.Pull(repoURL, "master", 0, false, false))

	err = filepath.Walk(filepath.Join(dir, ".git"), func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		b, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		assert.NotContains(t, string(b), token, "access token found in %s", path)
		return nil
	})
	require.NoError(t, err)

	// origin already exists, so adding it again fails.
	err = git.Pull(repoURL, "master", 0, false, false)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), token)
}
