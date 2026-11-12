package architecture

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// ModuleRename records a reviewed change of the Go module path (ADR 0217).
// The immutable base names every package, binding and evidence entry under
// the old path, so the strict base comparison would read the rename as the
// removal and re-addition of the whole tree. A declaration lets the base be
// read under the candidate's path. It carries no permission: every rule,
// ownership, projection and effect comparison still applies to the renamed
// base, and the Go diff must be the pure rename itself.
type ModuleRename struct {
	From        string `json:"from"`
	ApprovalADR string `json:"approval_adr"`
}

// moduleRenameTool is the project-relative tree that implements the rename
// check itself; it is the only Go source a rename change may edit otherwise.
const moduleRenameTool = "internal/architecture/"

var adrNumberPattern = regexp.MustCompile(`^[0-9]{4}$`)

func (p *Policy) validateModuleRename() error {
	rename := p.ModuleRename
	if rename == nil {
		return nil
	}
	if rename.From == "" || !strings.Contains(rename.From, ".") || strings.HasSuffix(rename.From, "/") {
		return fmt.Errorf("module_rename.from must be a canonical Go module path")
	}
	if rename.From == p.ModulePath {
		return fmt.Errorf("module_rename.from must differ from module_path")
	}
	if !adrNumberPattern.MatchString(rename.ApprovalADR) {
		return fmt.Errorf("module_rename.approval_adr must be a four-digit ADR number")
	}
	return nil
}

// baseRewrite presents base artifacts under the candidate's module path.
// A nil rewrite leaves them unchanged.
type baseRewrite struct {
	from, to []byte
}

func (r *baseRewrite) apply(blob []byte) []byte {
	if r == nil {
		return blob
	}
	return bytes.ReplaceAll(blob, r.from, r.to)
}

// resolveBaseRewrite returns the rewrite for an approved module rename, nil
// when the base already uses the candidate's module path or no rename is
// declared for it. An undeclared change stays a module_path loosening.
func resolveBaseRewrite(root, project, sha, policyPath string, candidate *Policy) (*baseRewrite, error) {
	rename := candidate.ModuleRename
	if rename == nil {
		return nil, nil
	}
	blob, err := readBasePolicyDocument(root, project, sha, policyPath)
	if err != nil {
		return nil, fmt.Errorf("read base policy: %w", err)
	}
	var header struct {
		ModulePath string `json:"module_path"`
	}
	if err := json.Unmarshal(blob, &header); err != nil {
		return nil, fmt.Errorf("read base module_path: %w", err)
	}
	if header.ModulePath == candidate.ModulePath || header.ModulePath != rename.From {
		return nil, nil
	}
	adrs, err := addedADRs(root, sha)
	if err != nil {
		return nil, err
	}
	if !adrs[rename.ApprovalADR] {
		return nil, fmt.Errorf("module rename from %s needs approval_adr naming an ADR added in this change, got %s", rename.From, rename.ApprovalADR)
	}
	if err := requirePureModuleRename(root, project, sha, rename.From, candidate.ModulePath); err != nil {
		return nil, err
	}
	return &baseRewrite{from: []byte(rename.From), to: []byte(candidate.ModulePath)}, nil
}

// requirePureModuleRename proves that every Go file of the project outside the
// rename tool equals its base file once the new module path is spelled as the
// old one. Binding digests hash these sources, so a digest that differs from
// the base then differs only by the module path.
func requirePureModuleRename(root, project, sha, from, to string) error {
	relative, err := repositoryRelativePath(root, project, project)
	if err != nil {
		return err
	}
	relative = strings.ReplaceAll(relative, "\\", "/")
	listing, err := gitOutput(root, "diff", "--name-status", "--no-renames", "-z", sha, "--", relative)
	if err != nil {
		return err
	}
	fields := strings.Split(strings.TrimSuffix(listing, "\x00"), "\x00")
	var problems []string
	for index := 0; index+1 < len(fields); index += 2 {
		status, name := fields[index], fields[index+1]
		inProject := name
		if relative != "." && relative != "" {
			inProject = strings.TrimPrefix(name, relative+"/")
		}
		if path.Ext(name) != ".go" || strings.HasPrefix(inProject, moduleRenameTool) {
			continue
		}
		if status != "M" {
			problems = append(problems, fmt.Sprintf("%s %s", status, name))
			continue
		}
		problem, err := moduleRenameFileProblem(root, sha, name, from, to)
		if err != nil {
			return err
		}
		if problem != "" {
			problems = append(problems, problem)
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("module rename must not change Go sources beyond the module path (%d):\n  %s", len(problems), strings.Join(problems, "\n  "))
	}
	return nil
}

func moduleRenameFileProblem(root, sha, name, from, to string) (string, error) {
	candidate, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		return "", err
	}
	base, err := gitOutput(root, "cat-file", "blob", sha+":"+name)
	if err != nil {
		return "", err
	}
	if bytes.Contains([]byte(base), []byte(to)) {
		return name + " already names " + to + " at the base", nil
	}
	if string(bytes.ReplaceAll(candidate, []byte(to), []byte(from))) != base {
		return name + " changes more than the module path", nil
	}
	return "", nil
}
