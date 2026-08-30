package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/architecture"
)

type projectionOptions struct {
	project, policy, baseline, output, focus string
}

type projectionInputs struct {
	policy   *architecture.Policy
	graph    *architecture.Graph
	baseline *architecture.LegacyManifest
}

type diagramBundle struct {
	SchemaVersion int                     `json:"schema_version"`
	Target        architecture.Projection `json:"target"`
	Migration     architecture.Projection `json:"migration"`
}

func runDiagram(args []string) error {
	options, err := parseProjectionOptions("diagram", args, false)
	if err != nil {
		return err
	}
	inputs, err := loadProjectionInputs(options)
	if err != nil {
		return err
	}
	target := architecture.TargetProjection(inputs.policy)
	migration := architecture.MigrationProjection(inputs.policy, inputs.graph, inputs.baseline)
	artifacts, err := diagramArtifacts(inputs.policy, target, migration)
	if err != nil {
		return err
	}
	return writeProjectionArtifacts(options.output, artifacts)
}

func runDependencies(args []string) error {
	options, err := parseProjectionOptions("dependencies", args, true)
	if err != nil {
		return err
	}
	inputs, err := loadProjectionInputs(options)
	if err != nil {
		return err
	}
	projection, err := architecture.DependenciesProjection(inputs.policy, inputs.graph, inputs.baseline, options.focus)
	if err != nil {
		return err
	}
	artifacts, err := dependencyArtifacts(inputs.policy, inputs.graph, projection, options.focus)
	if err != nil {
		return err
	}
	return writeProjectionArtifacts(options.output, artifacts)
}

func parseProjectionOptions(command string, args []string, requireFocus bool) (projectionOptions, error) {
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	project := flags.String("project", ".", "Go project directory")
	policyPath := flags.String("policy", filepath.Join("architecture", "policy.json"), "architecture policy")
	baselinePath := flags.String("baseline", "", "exact legacy JSONL baseline")
	output := flags.String("output", "", "temporary output directory")
	focus := flags.String("focus", "", "exact module owner or package path")
	if err := flags.Parse(args); err != nil {
		return projectionOptions{}, err
	}
	if flags.NArg() != 0 {
		return projectionOptions{}, fmt.Errorf("%s: unexpected arguments: %v", command, flags.Args())
	}
	if requireFocus && *focus == "" {
		return projectionOptions{}, fmt.Errorf("dependencies requires --focus")
	}
	if !requireFocus && *focus != "" {
		return projectionOptions{}, fmt.Errorf("diagram does not accept --focus; use dependencies")
	}
	absoluteProject, err := filepath.Abs(*project)
	if err != nil {
		return projectionOptions{}, fmt.Errorf("resolve project path: %w", err)
	}
	return projectionOptions{project: absoluteProject, policy: projectPath(absoluteProject, *policyPath), baseline: projectPath(absoluteProject, *baselinePath), output: *output, focus: *focus}, nil
}

func loadProjectionInputs(options projectionOptions) (projectionInputs, error) {
	policy, err := architecture.LoadPolicy(options.policy)
	if err != nil {
		return projectionInputs{}, err
	}
	graph, err := architecture.LoadGraph(options.project, policy)
	if err != nil {
		return projectionInputs{}, err
	}
	var baseline *architecture.LegacyManifest
	if options.baseline != "" {
		baseline, err = architecture.LoadLegacyManifest(options.baseline)
		if err != nil {
			return projectionInputs{}, err
		}
	}
	return projectionInputs{policy: policy, graph: graph, baseline: baseline}, nil
}

func diagramArtifacts(policy *architecture.Policy, target, migration architecture.Projection) (map[string][]byte, error) {
	targetSVG, err := architecture.RenderSVG(target)
	if err != nil {
		return nil, err
	}
	migrationSVG, err := architecture.RenderSVG(migration)
	if err != nil {
		return nil, err
	}
	bundleJSON, err := json.MarshalIndent(diagramBundle{SchemaVersion: architecture.ProjectionSchemaVersion, Target: target, Migration: migration}, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal diagram bundle: %w", err)
	}
	return map[string][]byte{
		"target.svg": targetSVG, "migration.svg": migrationSVG,
		"architecture.json": append(bundleJSON, '\n'), "go-arch-lint.yml": architecture.GoArchLintProjection(policy),
	}, nil
}

func dependencyArtifacts(policy *architecture.Policy, graph *architecture.Graph, projection architecture.Projection, focus string) (map[string][]byte, error) {
	svg, err := architecture.RenderSVG(projection)
	if err != nil {
		return nil, err
	}
	projectionJSON, err := architecture.MarshalProjection(projection)
	if err != nil {
		return nil, err
	}
	query, err := architecture.GodaQuery(policy, graph, focus)
	if err != nil {
		return nil, err
	}
	return map[string][]byte{"dependencies.svg": svg, "dependencies.json": projectionJSON, "dependencies.goda": []byte(query)}, nil
}

func writeProjectionArtifacts(output string, artifacts map[string][]byte) error {
	directory, err := prepareOutputDirectory(output)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(artifacts))
	for name := range artifacts {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := writeProjectionArtifact(filepath.Join(directory, name), artifacts[name]); err != nil {
			return fmt.Errorf("write generated architecture artifact %s: %w", name, err)
		}
	}
	fmt.Println(directory)
	return nil
}

func writeProjectionArtifact(path string, contents []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(contents); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func prepareOutputDirectory(output string) (string, error) {
	if output == "" {
		directory, err := os.MkdirTemp("", "phoenix-backend-architecture-")
		if err != nil {
			return "", fmt.Errorf("create temporary architecture output: %w", err)
		}
		return directory, nil
	}
	directory, err := filepath.Abs(output)
	if err != nil {
		return "", fmt.Errorf("resolve architecture output directory: %w", err)
	}
	temporaryRoot, err := filepath.Abs(os.TempDir())
	if err != nil {
		return "", fmt.Errorf("resolve system temporary directory: %w", err)
	}
	if !pathWithin(temporaryRoot, directory) {
		return "", fmt.Errorf("architecture output %q must be inside the system temporary directory %q", directory, temporaryRoot)
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", fmt.Errorf("create architecture output directory: %w", err)
	}
	realTemporaryRoot, err := filepath.EvalSymlinks(temporaryRoot)
	if err != nil {
		return "", fmt.Errorf("resolve system temporary directory symlinks: %w", err)
	}
	realDirectory, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return "", fmt.Errorf("resolve architecture output directory symlinks: %w", err)
	}
	if !pathWithin(realTemporaryRoot, realDirectory) || realDirectory == realTemporaryRoot {
		return "", fmt.Errorf("architecture output %q must resolve inside the system temporary directory %q", directory, temporaryRoot)
	}
	return realDirectory, nil
}

func pathWithin(parent, child string) bool {
	relative, err := filepath.Rel(parent, child)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
