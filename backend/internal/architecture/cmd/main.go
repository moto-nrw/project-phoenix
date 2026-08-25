package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/moto-nrw/project-phoenix/internal/architecture"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("command is required: check or explain")
	}

	switch args[0] {
	case "check":
		return runCheck(args[1:])
	case "explain":
		return runExplain(args[1:])
	default:
		return fmt.Errorf("unknown command %q: expected check or explain", args[0])
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
	policy, err := architecture.LoadPolicy(*policyPath)
	if err != nil {
		return err
	}
	explanation, err := policy.Explain(architecture.Scope(*scope), *source, *target)
	if err != nil {
		return err
	}
	fmt.Println(explanation)
	return nil
}

func runCheck(args []string) error {
	flags := flag.NewFlagSet("check", flag.ContinueOnError)
	project := flags.String("project", ".", "Go project directory")
	policyPath := flags.String("policy", filepath.Join("architecture", "policy.json"), "architecture policy")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("check: unexpected arguments: %v", flags.Args())
	}

	path := *policyPath
	if !filepath.IsAbs(path) {
		path = filepath.Join(*project, path)
	}
	policy, err := architecture.LoadPolicy(path)
	if err != nil {
		return err
	}
	graph, err := architecture.LoadGraph(*project, policy)
	if err != nil {
		return err
	}
	return architecture.FormatViolations(architecture.Check(policy, graph))
}
