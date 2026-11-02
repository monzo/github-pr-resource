package resource

import (
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/shurcooL/githubv4"
)

var trustedTeamMembers map[string]bool = map[string]bool{}

// Check (business logic)
func Check(request CheckRequest, manager Github) (CheckResponse, error) {
	pulls, err := filteredPullRequests(request.Source, manager)
	if err != nil {
		return nil, err
	}

	if request.Source.List {
		version, _, err := listVersion(pulls)
		if err != nil {
			return nil, err
		}
		return CheckResponse{version}, nil
	}

	var response CheckResponse
	for _, p := range pulls {
		version := NewVersion(p)
		version.ConfigKey = request.Source.ConfigKey
		response = append(response, version)
	}

	log.Printf("Finished processing pull requests")

	// Sort the commits by date
	sort.Sort(response)

	// If there are no new but an old version = return the old
	if len(response) == 0 && request.Version.PR != "" {
		response = append(response, request.Version)
	}
	// If there are new versions and no previous = return just the latest
	if len(response) != 0 && request.Version.PR == "" {
		response = CheckResponse{response[len(response)-1]}
	}

	return response, nil
}

// filteredPullRequests returns the pull requests that pass the source's filters.
func filteredPullRequests(source Source, manager Github) ([]*PullRequest, error) {
	var filtered []*PullRequest

	// Filter out pull request if it does not have a filtered state
	filterStates := []githubv4.PullRequestState{githubv4.PullRequestStateOpen}
	if len(source.States) > 0 {
		filterStates = source.States
	}
	log.Printf("Filtering for states: %v", filterStates)

	var prUpdatedAfterTime *time.Time
	if source.UpdatedInTheLast != "" {
		duration, err := time.ParseDuration(source.UpdatedInTheLast)
		if err == nil {
			updatedInTheLastTime := time.Now().Add(-duration)
			prUpdatedAfterTime = &updatedInTheLastTime
			log.Printf("Filtering PRs updated since %v", prUpdatedAfterTime)
		} else {
			log.Printf("Error parsing updated_in_the_last field: %v", err)
		}
	}

	var pulls []*PullRequest
	if source.Number != 0 {
		p, err := manager.GetPullRequestByNumber(source.Number)
		if err != nil {
			return nil, fmt.Errorf("failed to get pull request #%d: %s", source.Number, err)
		}
		// Apply the state and updated_in_the_last filters that ListPullRequests applies server side.
		if p != nil && slices.Contains(filterStates, p.State) &&
			(prUpdatedAfterTime == nil || !p.UpdatedAt.Time.Before(*prUpdatedAfterTime)) {
			pulls = append(pulls, p)
		}
	} else {
		var err error
		pulls, err = manager.ListPullRequests(filterStates, source.Branch, prUpdatedAfterTime)
		if err != nil {
			return nil, fmt.Errorf("failed to get last commits: %s", err)
		}
	}

	disableSkipCI := source.DisableCISkip

	log.Printf("Processing pull requests")

Loop:
	for _, p := range pulls {
		// [ci skip]/[skip ci] in Pull request title
		if !disableSkipCI && ContainsSkipCI(p.Title) {
			continue
		}

		// [ci skip]/[skip ci] in Commit message
		if !disableSkipCI && ContainsSkipCI(p.Tip.Message) {
			continue
		}

		// Filter pull request if the BaseBranch does not match the one specified in source
		if source.BaseBranch != "" && p.PullRequestObject.BaseRefName != source.BaseBranch {
			continue
		}

		// Filter out pull request if it does not contain at least one of the desired labels
		if len(source.Labels) > 0 {
			labelFound := false

		LabelLoop:
			for _, wantedLabel := range source.Labels {
				for _, targetLabel := range p.Labels {
					if targetLabel.Name == wantedLabel {
						labelFound = true
						break LabelLoop
					}
				}
			}

			if !labelFound {
				continue Loop
			}
		}

		// Filter out forks.
		if source.DisableForks && p.IsCrossRepository {
			continue
		}

		// Filter out drafts.
		if source.IgnoreDrafts && p.IsDraft {
			continue
		}

		prAuthorTrusted, err := userTrusted(p.Author.Login, source.TrustedUsers, source.TrustedTeams, manager)
		if err != nil {
			return nil, fmt.Errorf("failed to list users in team %v: %s", source.TrustedTeams, err)
		}

		prApproved := p.ApprovedReviewCount >= source.RequiredReviewApprovals

		if !prAuthorTrusted && !prApproved {
			continue
		}

		// Fetch files once if paths/ignore_paths are specified.
		var files []string

		if len(source.Paths) > 0 || len(source.IgnorePaths) > 0 {
			files, err = manager.ListModifiedFiles(p.Number)
			if err != nil {
				return nil, fmt.Errorf("failed to list modified files: %s", err)
			}
		}

		// Skip version if no files match the specified paths.
		if len(source.Paths) > 0 {
			var wanted []string
			for _, pattern := range source.Paths {
				w, err := FilterPath(files, pattern)
				if err != nil {
					return nil, fmt.Errorf("path match failed: %s", err)
				}
				wanted = append(wanted, w...)
			}
			if len(wanted) == 0 {
				continue Loop
			}
		}

		// Skip version if all files are ignored.
		if len(source.IgnorePaths) > 0 {
			wanted := files
			for _, pattern := range source.IgnorePaths {
				wanted, err = FilterIgnorePath(wanted, pattern)
				if err != nil {
					return nil, fmt.Errorf("ignore path match failed: %s", err)
				}
			}
			if len(wanted) == 0 {
				continue Loop
			}
		}

		filtered = append(filtered, p)
	}

	return filtered, nil
}

func userTrusted(user string, trustedUsers, trustedTeams []string, manager Github) (bool, error) {
	for _, u := range trustedUsers {
		if user == u {
			return true, nil
		}
	}

	if trustedTeamMembers[user] {
		return true, nil
	}

	for _, team := range trustedTeams {
		teamMembers, err := manager.ListTeamMembers(team)
		if err != nil {
			return false, fmt.Errorf("failed to list team members of team %q: %s", trustedTeams, err)
		}

		for _, u := range teamMembers {
			trustedTeamMembers[u] = true
		}

		if trustedTeamMembers[user] {
			return true, nil
		}
	}

	return false, nil
}

// ContainsSkipCI returns true if a string contains [ci skip] or [skip ci].
func ContainsSkipCI(s string) bool {
	re := regexp.MustCompile(`(?i)\[(ci skip|skip ci)\]`)
	return re.MatchString(s)
}

// FilterIgnorePath ...
func FilterIgnorePath(files []string, pattern string) ([]string, error) {
	var out []string
	for _, file := range files {
		match, err := filepath.Match(pattern, file)
		if err != nil {
			return nil, err
		}
		if !match && !IsInsidePath(pattern, file) {
			out = append(out, file)
		}
	}
	return out, nil
}

// FilterPath ...
func FilterPath(files []string, pattern string) ([]string, error) {
	var out []string
	for _, file := range files {
		match, err := filepath.Match(pattern, file)
		if err != nil {
			return nil, err
		}
		if match || IsInsidePath(pattern, file) {
			out = append(out, file)
		}
	}
	return out, nil
}

// IsInsidePath checks whether the child path is inside the parent path.
//
// /foo/bar is inside /foo, but /foobar is not inside /foo.
// /foo is inside /foo, but /foo is not inside /foo/
func IsInsidePath(parent, child string) bool {
	if parent == child {
		return true
	}

	// we add a trailing slash so that we only get prefix matches on a
	// directory separator
	parentWithTrailingSlash := parent
	if !strings.HasSuffix(parentWithTrailingSlash, string(filepath.Separator)) {
		parentWithTrailingSlash += string(filepath.Separator)
	}

	return strings.HasPrefix(child, parentWithTrailingSlash)
}

// CheckRequest ...
type CheckRequest struct {
	Source  Source  `json:"source"`
	Version Version `json:"version"`
}

// CheckResponse ...
type CheckResponse []Version

func (r CheckResponse) Len() int {
	return len(r)
}

func (r CheckResponse) Less(i, j int) bool {
	return r[j].CommittedDate.After(r[i].CommittedDate)
}

func (r CheckResponse) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}
