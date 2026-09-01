package architecture

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var fullCommitPattern = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)

func LoadBasePolicyAndManifest(project, policyPath, baselinePath, ref string) (*Policy, *LegacyManifest, error) {
	root, err := gitOutput(project, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, nil, fmt.Errorf("resolve repository root: %w", err)
	}
	root = strings.TrimSpace(root)
	sha, err := resolveBaseCommit(root, ref)
	if err != nil {
		return nil, nil, err
	}
	policyBlob, err := readGitBlob(root, project, sha, policyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read base policy: %w", err)
	}
	baselineBlob, err := readGitBlob(root, project, sha, baselinePath)
	if err != nil {
		return nil, nil, fmt.Errorf("read base legacy baseline: %w", err)
	}
	policy, err := DecodePolicy(bytes.NewReader(policyBlob))
	if err != nil {
		return nil, nil, fmt.Errorf("decode base policy: %w", err)
	}
	manifest, err := DecodeLegacyManifest(bytes.NewReader(baselineBlob))
	if err != nil {
		return nil, nil, fmt.Errorf("decode base legacy baseline: %w", err)
	}
	return policy, manifest, nil
}

func resolveBaseCommit(root, ref string) (string, error) {
	if !fullCommitPattern.MatchString(ref) {
		return "", fmt.Errorf("base ref %q must be a full immutable 40-character commit SHA", ref)
	}
	sha, err := gitOutput(root, "rev-parse", "--verify", ref+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("resolve base ref %q: %w", ref, err)
	}
	return strings.TrimSpace(sha), nil
}

func readGitBlob(root, project, sha, candidatePath string) ([]byte, error) {
	relative, err := repositoryRelativePath(root, project, candidatePath)
	if err != nil {
		return nil, err
	}
	output, err := exec.Command("git", "-C", root, "cat-file", "blob", sha+":"+filepath.ToSlash(relative)).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git cat-file %s:%s: %w: %s", sha, relative, err, strings.TrimSpace(string(output)))
	}
	return output, nil
}

func repositoryRelativePath(root, project, candidatePath string) (string, error) {
	project, err := filepath.Abs(project)
	if err != nil {
		return "", fmt.Errorf("resolve project path %q: %w", project, err)
	}
	absolute := candidatePath
	if !filepath.IsAbs(absolute) {
		absolute = filepath.Join(project, absolute)
	}
	relative, err := filepath.Rel(project, filepath.Clean(absolute))
	if err != nil {
		return "", fmt.Errorf("resolve repository path %q: %w", candidatePath, err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q is outside project %s", candidatePath, project)
	}
	physicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve repository root %q: %w", root, err)
	}
	physicalProject, err := filepath.EvalSymlinks(project)
	if err != nil {
		return "", fmt.Errorf("resolve project path %q: %w", project, err)
	}
	projectRelative, err := filepath.Rel(physicalRoot, physicalProject)
	if err != nil {
		return "", fmt.Errorf("resolve project path %q: %w", project, err)
	}
	if projectRelative == ".." || strings.HasPrefix(projectRelative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("project %q is outside repository %s", project, root)
	}
	return filepath.Join(projectRelative, relative), nil
}

func gitOutput(dir string, args ...string) (string, error) {
	output, err := processOutput(dir, nil, "git", append([]string{"-C", dir}, args...)...)
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

func processOutput(dir string, environment []string, executable string, args ...string) ([]byte, error) {
	command := exec.Command(executable, args...)
	command.Dir = dir
	command.Env = environment
	return command.CombinedOutput()
}
