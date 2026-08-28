package architecture

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

type IssueAuditResult struct {
	Issues  int
	Entries int
}

type issueResponse struct {
	State       string          `json:"state"`
	PullRequest json.RawMessage `json:"pull_request"`
}

func AuditLegacyIssues(ctx context.Context, client *http.Client, apiURL, token string, manifest *LegacyManifest) (IssueAuditResult, error) {
	base, err := url.Parse(apiURL)
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return IssueAuditResult{}, fmt.Errorf("GitHub API URL %q is invalid", apiURL)
	}
	issues, err := uniqueManifestIssues(manifest)
	if err != nil {
		return IssueAuditResult{}, err
	}
	for _, issue := range issues {
		if err := auditIssue(ctx, client, base, token, issue); err != nil {
			return IssueAuditResult{}, err
		}
	}
	return IssueAuditResult{Issues: len(issues), Entries: len(manifest.Entries)}, nil
}

func uniqueManifestIssues(manifest *LegacyManifest) ([]GitHubIssue, error) {
	byURL := make(map[string]GitHubIssue)
	for _, entry := range manifest.Entries {
		issue, err := ParseGitHubIssue(entry.Issue)
		if err != nil {
			return nil, err
		}
		byURL[issue.URL] = issue
	}
	urls := make([]string, 0, len(byURL))
	for issueURL := range byURL {
		urls = append(urls, issueURL)
	}
	sort.Strings(urls)
	issues := make([]GitHubIssue, 0, len(urls))
	for _, issueURL := range urls {
		issues = append(issues, byURL[issueURL])
	}
	return issues, nil
}

func auditIssue(ctx context.Context, client *http.Client, base *url.URL, token string, issue GitHubIssue) error {
	endpoint := *base
	endpoint.Path = strings.TrimRight(base.Path, "/") + fmt.Sprintf("/repos/%s/%s/issues/%d", issue.Owner, issue.Repo, issue.Number)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return fmt.Errorf("build request for %s: %w", issue.URL, err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "project-phoenix-backend-architecture-audit")
	if token != "" && base.Scheme == "https" && base.Hostname() == "api.github.com" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("audit migration issue %s: %w", issue.URL, err)
	}
	defer func() { _ = response.Body.Close() }()
	return validateIssueResponse(response, issue)
}

func validateIssueResponse(response *http.Response, issue GitHubIssue) error {
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("audit migration issue %s: GitHub returned %s", issue.URL, response.Status)
	}
	var payload issueResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil {
		return fmt.Errorf("decode migration issue %s: %w", issue.URL, err)
	}
	if len(payload.PullRequest) != 0 {
		return fmt.Errorf("migration issue %s is a pull request, not an issue", issue.URL)
	}
	if payload.State != "open" {
		return fmt.Errorf("migration issue %s is %s", issue.URL, payload.State)
	}
	return nil
}
