package resource

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
)

// listFile is the file get writes the pull request list to in list mode.
const listFile = "prs.json"

// ListedPullRequest is a pull request in list mode's list.
type ListedPullRequest struct {
	Number       int    `json:"number"`
	HeadRefOID   string `json:"headRefOid"`
	ChangedFiles int    `json:"changedFiles"`
}

// listVersion returns list mode's version for the filtered pull requests, a digest of their list, and the list
// as JSON.
func listVersion(pulls []*PullRequest) (Version, []byte, error) {
	listed := make([]ListedPullRequest, 0, len(pulls))
	for _, p := range pulls {
		listed = append(listed, ListedPullRequest{Number: p.Number, HeadRefOID: p.Tip.OID, ChangedFiles: p.ChangedFiles})
	}
	sort.Slice(listed, func(i, j int) bool { return listed[i].Number < listed[j].Number })

	content, err := json.Marshal(listed)
	if err != nil {
		return Version{}, nil, err
	}
	digest := sha256.Sum256(content)
	return Version{List: hex.EncodeToString(digest[:])}, content, nil
}

// getList writes the current pull request list to prs.json. If the list has changed since check, it writes the
// current one: check will emit its version next.
func getList(request GetRequest, github Github, outputDir string) (*GetResponse, error) {
	pulls, err := filteredPullRequests(request.Source, github)
	if err != nil {
		return nil, err
	}
	version, content, err := listVersion(pulls)
	if err != nil {
		return nil, err
	}
	if version.List != request.Version.List {
		log.Printf("The pull request list has changed since check, writing the current list")
	}
	if err := os.WriteFile(filepath.Join(outputDir, listFile), content, 0644); err != nil {
		return nil, fmt.Errorf("failed to write %s: %s", listFile, err)
	}
	return &GetResponse{Version: request.Version}, nil
}
