package architecture

import (
	"fmt"
	"path/filepath"
	"strings"
)

// candidateGoPackages returns policy packages whose first Go source file was
// added after the immutable PR base. Existing unclassified packages and
// modified packages do not qualify.
func candidateGoPackages(project, ref string, candidate *Policy) (map[string]struct{}, error) {
	root, err := gitOutput(project, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("resolve repository root for package additions: %w", err)
	}
	root = strings.TrimSpace(root)
	sha, err := resolveBaseCommit(root, ref)
	if err != nil {
		return nil, err
	}
	projectPath, err := repositoryRelativePath(root, project, project)
	if err != nil {
		return nil, fmt.Errorf("resolve project directory: %w", err)
	}
	added, err := goPackageDirs(root, "diff", "--name-only", "--diff-filter=A", "-z", sha, "--", filepath.ToSlash(projectPath))
	if err != nil {
		return nil, fmt.Errorf("list candidate package files: %w", err)
	}
	base, err := goPackageDirs(root, "ls-tree", "-r", "--name-only", "-z", sha, "--", filepath.ToSlash(projectPath))
	if err != nil {
		return nil, fmt.Errorf("list base package files: %w", err)
	}

	result := make(map[string]struct{})
	for path, pkg := range candidate.packageMap() {
		dir := filepath.Clean(filepath.Join(projectPath, filepath.FromSlash(pkg.Path)))
		if _, wasAdded := added[dir]; !wasAdded {
			continue
		}
		if _, alreadyExisted := base[dir]; !alreadyExisted {
			result[path] = struct{}{}
		}
	}
	return result, nil
}

func goPackageDirs(root string, args ...string) (map[string]struct{}, error) {
	output, err := gitOutput(root, args...)
	if err != nil {
		return nil, err
	}
	dirs := make(map[string]struct{})
	for _, name := range strings.Split(output, "\x00") {
		if filepath.Ext(name) == ".go" {
			dirs[filepath.Clean(filepath.Dir(name))] = struct{}{}
		}
	}
	return dirs, nil
}
