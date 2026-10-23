package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/architecture"
	"github.com/moto-nrw/project-phoenix/internal/passthrough"
)

func main() {
	project := os.Getenv("PHOENIX_ARCHITECTURE_PROJECT")
	if project == "" {
		fmt.Fprintln(os.Stderr, "PHOENIX_ARCHITECTURE_PROJECT is required")
		os.Exit(1)
	}
	if err := os.Chdir(project); err != nil {
		fmt.Fprintf(os.Stderr, "change to backend project: %v\n", err)
		os.Exit(1)
	}
	if len(os.Args) > 1 && os.Args[1] == "passthrough-snapshot" {
		if err := passthrough.WriteSnapshot(os.Stdout, project, "0090"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	dependencies := architecture.CLIDependencies{
		IssueClient: issueHTTPClient{inner: &http.Client{Timeout: 15 * time.Second}},
		Getenv:      os.Getenv,
		ProjectRoot: filepath.Dir(project),
	}
	if err := architecture.RunCLI(os.Args[1:], dependencies); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if len(os.Args) > 1 && os.Args[1] == "check" {
		scanProject, base := project, ""
		for i := 2; i < len(os.Args); i++ {
			arg := os.Args[i]
			switch {
			case arg == "--base-ref" && i+1 < len(os.Args):
				i++
				base = os.Args[i]
			case strings.HasPrefix(arg, "--base-ref="):
				base = strings.TrimPrefix(arg, "--base-ref=")
			case arg == "--project" && i+1 < len(os.Args):
				i++
				scanProject = os.Args[i]
			case strings.HasPrefix(arg, "--project="):
				scanProject = strings.TrimPrefix(arg, "--project=")
			}
		}
		if err := passthrough.CheckProject(project, scanProject, base); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

}

type issueHTTPClient struct{ inner *http.Client }

func (client issueHTTPClient) Do(ctx context.Context, request architecture.IssueRequest) (architecture.IssueResponse, error) {
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, request.URL, nil)
	if err != nil {
		return architecture.IssueResponse{}, err
	}
	for key, value := range request.Headers {
		httpRequest.Header.Set(key, value)
	}
	response, err := client.inner.Do(httpRequest)
	if err != nil {
		return architecture.IssueResponse{}, err
	}
	return architecture.IssueResponse{StatusCode: response.StatusCode, Status: response.Status, Body: response.Body}, nil
}
