package architecture

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

type CLIDependencies struct {
	IssueClient IssueClient
	Getenv      func(string) string
}

func RunCLI(args []string, dependencies CLIDependencies) error {
	if len(args) == 0 {
		return fmt.Errorf("command is required: check, explain, diagram, dependencies, or audit-issues")
	}

	switch args[0] {
	case "check":
		return runCheck(args[1:])
	case "explain":
		return runExplain(args[1:])
	case "audit-issues":
		return runAuditIssues(args[1:], dependencies)
	case "diagram":
		return runDiagram(args[1:])
	case "dependencies":
		return runDependencies(args[1:])
	default:
		return fmt.Errorf("unknown command %q: expected check, explain, diagram, dependencies, or audit-issues", args[0])
	}
}

func runExplain(args []string) error {
	flags := flag.NewFlagSet("explain", flag.ContinueOnError)
	policyPath := flags.String("policy", filepath.Join("architecture", "policy.json"), "architecture policy")
	scope := flags.String("scope", "", "edge scope")
	source := flags.String("source", "", "source import path")
	target := flags.String("target", "", "target import path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *scope == "" || *source == "" || *target == "" {
		return fmt.Errorf("explain requires --scope, --source, and --target")
	}
	policy, err := LoadPolicy(*policyPath)
	if err != nil {
		return err
	}
	explanation, err := policy.Explain(Scope(*scope), *source, *target)
	if err != nil {
		return err
	}
	fmt.Println(explanation)
	return nil
}

func runCheck(args []string) error {
	options, err := parseCheckOptions(args)
	if err != nil {
		return err
	}
	policy, err := LoadPolicy(options.policy)
	if err != nil {
		return err
	}
	graph, err := LoadGraph(options.project, policy)
	if err != nil {
		return err
	}
	violations := Check(policy, graph)
	if options.baseline == "" {
		return FormatViolations(violations)
	}
	return runRatchet(options, policy, violations)
}

type checkOptions struct {
	project, policy, baseline, baseRef string
}

func parseCheckOptions(args []string) (checkOptions, error) {
	flags := flag.NewFlagSet("check", flag.ContinueOnError)
	project := flags.String("project", ".", "Go project directory")
	policyPath := flags.String("policy", filepath.Join("architecture", "policy.json"), "architecture policy")
	baselinePath := flags.String("baseline", "", "exact legacy JSONL baseline")
	baseRef := flags.String("base-ref", "", "immutable pull-request base commit SHA")
	if err := flags.Parse(args); err != nil {
		return checkOptions{}, err
	}
	if flags.NArg() != 0 {
		return checkOptions{}, fmt.Errorf("check: unexpected arguments: %v", flags.Args())
	}
	if *baselinePath == "" && *baseRef != "" {
		return checkOptions{}, fmt.Errorf("check: --base-ref requires --baseline")
	}
	absoluteProject, err := filepath.Abs(*project)
	if err != nil {
		return checkOptions{}, fmt.Errorf("resolve project path: %w", err)
	}
	return checkOptions{
		project:  absoluteProject,
		policy:   projectPath(absoluteProject, *policyPath),
		baseline: projectPath(absoluteProject, *baselinePath),
		baseRef:  *baseRef,
	}, nil
}

func runRatchet(options checkOptions, policy *Policy, violations []Violation) error {
	manifest, err := LoadLegacyManifest(options.baseline)
	if err != nil {
		return err
	}
	remaining, localErr := EnforceLegacyBaseline(violations, manifest)
	var baseErr error
	if options.baseRef != "" {
		baseErr = compareWithBase(options, policy, manifest)
	}
	if err := errors.Join(localErr, baseErr); err != nil {
		return fmt.Errorf("%w\nremaining legacy violations: %d", err, remaining)
	}
	fmt.Printf("backend architecture ratchet passed: %d legacy violation(s) remain\n", remaining)
	return nil
}

func compareWithBase(options checkOptions, policy *Policy, manifest *LegacyManifest) error {
	basePolicy, baseManifest, err := LoadBasePolicyAndManifest(options.project, options.policy, options.baseline, options.baseRef)
	if err != nil {
		return err
	}
	return errors.Join(
		CompareCandidatePolicyStrictness(options.project, options.baseRef, basePolicy, policy),
		CompareLegacyBaselines(manifest, baseManifest),
	)
}

func runAuditIssues(args []string, dependencies CLIDependencies) error {
	flags := flag.NewFlagSet("audit-issues", flag.ContinueOnError)
	baselinePath := flags.String("baseline", "", "exact legacy JSONL baseline")
	apiURL := flags.String("api-url", "", "GitHub API base URL")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *baselinePath == "" || *apiURL == "" {
		return fmt.Errorf("audit-issues requires --baseline, --api-url, and no positional arguments")
	}
	manifest, err := LoadLegacyManifest(*baselinePath)
	if err != nil {
		return err
	}
	if dependencies.IssueClient == nil {
		return fmt.Errorf("audit-issues requires an issue client dependency")
	}
	getenv := dependencies.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}
	result, err := AuditLegacyIssues(context.Background(), dependencies.IssueClient, *apiURL, getenv("GITHUB_TOKEN"), manifest)
	if err != nil {
		return err
	}
	fmt.Printf("issue audit passed: %d open migration issue(s) cover %d legacy violation(s)\n", result.Issues, result.Entries)
	return nil
}

func projectPath(project, path string) string {
	if path == "" {
		return ""
	}
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(project, path)
}
