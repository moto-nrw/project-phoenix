package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/architecture"
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
	dependencies := architecture.CLIDependencies{
		IssueClient: issueHTTPClient{inner: &http.Client{Timeout: 15 * time.Second}},
		Getenv:      os.Getenv,
	}
	if err := architecture.RunCLI(os.Args[1:], dependencies); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
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
