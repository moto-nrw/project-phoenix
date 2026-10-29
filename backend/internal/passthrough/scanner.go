// Package passthrough records exact source evidence, not permission to forward.
package passthrough

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/build/constraint"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/moto-nrw/project-phoenix/architecture/artifacts"
)

// Site identifies a method independent of its source filename. Evidence pins its
// formatted declaration (signature and body), without comments or whitespace.
// Unclassified bodies carry evidence only, never assumed to be translations.
type Site struct {
	ID       string `json:"id"`
	Owner    string `json:"owner"`
	Build    string `json:"build"`
	Form     string `json:"form"`
	Evidence string `json:"evidence"`
	Reason   string `json:"reason"`
	Decision string `json:"decision"`
}

// Scan deliberately visits all backend trees, including future ones, legacy,
// generated sources and fixtures. Only hidden directories and _test.go are
// excluded, exactly as in ADR 0087's source gates. No owner selector can hide a
// relocation. The legacy four-condition predicate remains unchanged.
func Scan(root string) ([]Site, error) {
	var sites []Site
	packages, err := sourcePackages(root)
	if err != nil {
		return nil, err
	}
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path != root && strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		found, err := scanFile(root, path, packages)
		sites = append(sites, found...)
		return err
	})
	sort.Slice(sites, func(i, j int) bool { return sites[i].ID < sites[j].ID })
	return sites, err
}

func owner(path string) string {
	for prefix, consumer := range map[string]string{
		"roots/devicescanbinding/":       "modules/devicescan",
		"roots/openroommovebinding/":     "workflows/openroommove",
		"roots/sessionendbinding/":       "workflows/sessionend",
		"roots/requestreviewbinding/":    "modules/requestreview",
		"api/device_scan_catalog.go":     "modules/devicescan",
		"api/import_staff_invitation.go": "modules/dataimport",
		"api/kiosk_timetable.go":         "modules/devicescan",
	} {
		if strings.HasPrefix(path, prefix) {
			return consumer
		}
	}
	parts := strings.Split(path, "/")
	if len(parts) > 2 && (parts[0] == "modules" || parts[0] == "workflows") {
		return strings.Join(parts[:2], "/")
	}
	if parts[0] == "roots" || parts[0] == "api" || parts[0] == "cmd" || len(parts) == 1 {
		return "roots"
	}
	return "source/" + parts[0]
}

// CompareSites rejects new or changed sites and stale evidence separately. A
// deletion or deepening removes an entry; it cannot fund another method.
func CompareSites(registered, measured []Site) []string {
	before := make(map[string]Site, len(registered))
	var problems []string
	for _, site := range registered {
		if _, exists := before[site.ID]; exists {
			problems = append(problems, "duplicate registration: "+site.ID)
		}
		before[site.ID] = site
	}
	for _, site := range measured {
		previous, exists := before[site.ID]
		if !exists {
			problems = append(problems, "unregistered method: "+site.ID)
		} else if previous.Evidence != site.Evidence || previous.Build != site.Build || previous.Form != site.Form || previous.Owner != site.Owner {
			problems = append(problems, "changed method evidence: "+site.ID)
		}
		delete(before, site.ID)
	}
	for id := range before {
		problems = append(problems, "stale method evidence: "+id)
	}
	sort.Strings(problems)
	return problems
}

// Only package metadata is read, never policy permissions or architecture
// internals. The scan itself does not depend on package classification.
func sourcePackages(root string) (map[string]string, error) {
	data, err := artifacts.ReadPolicy(filepath.Join(root, "architecture/policy.json"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var policy struct {
		Packages []struct{ Path, Role, Owner string }
		Owners   []struct{ ID, Kind string }
	}
	if err := json.Unmarshal(data, &policy); err != nil {
		return nil, err
	}
	roles := map[string]string{}
	kinds := map[string]string{}
	for _, owner := range policy.Owners {
		kinds[owner.ID] = owner.Kind
	}
	for _, pkg := range policy.Packages {
		roles[pkg.Path] = pkg.Role
		if kinds[pkg.Owner] == "tool" {
			roles[pkg.Path] = "tool"
		}
	}
	return roles, nil
}

func buildCategory(file *ast.File, dir string, roles map[string]string) string {
	for _, group := range file.Comments {
		for _, comment := range group.List {
			if !strings.HasPrefix(comment.Text, "//go:build ") {
				continue
			}
			expr, err := constraint.Parse(comment.Text)
			if err == nil && requiresTesthooks(expr) {
				return "testhooks"
			}
		}
	}
	if roles[dir] == "test-support" || dir == "test" || strings.Contains("/"+dir+"/", "/testdata/") {
		return "test-support"
	}
	if roles[dir] == "migration" || roles[dir] == "tool" {
		return roles[dir]
	}
	return "production"
}

// A file is hook-only iff its build expression cannot be true without
// testhooks, regardless of platform/other tags. Enumerate the few tags present
// in real constraints rather than assuming the host's GOOS or tag set.
func requiresTesthooks(expr constraint.Expr) bool {
	tags := map[string]bool{}
	hasHook := false
	var visit func(constraint.Expr)
	visit = func(current constraint.Expr) {
		switch node := current.(type) {
		case *constraint.TagExpr:
			if node.Tag == "testhooks" {
				hasHook = true
			} else {
				tags[node.Tag] = true
			}
		case *constraint.NotExpr:
			visit(node.X)
		case *constraint.AndExpr:
			visit(node.X)
			visit(node.Y)
		case *constraint.OrExpr:
			visit(node.X)
			visit(node.Y)
		}
	}
	visit(expr)
	if !hasHook {
		return false
	}
	names := make([]string, 0, len(tags))
	for tag := range tags {
		names = append(names, tag)
	}
	if len(names) > 16 {
		return false
	} // Conservatively retain production cost.
	for mask := 0; mask < 1<<len(names); mask++ {
		values := map[string]bool{}
		for i, name := range names {
			values[name] = mask&(1<<i) != 0
		}
		if expr.Eval(func(tag string) bool { return values[tag] }) {
			return false
		}
	}
	return true
}

func scanFile(root, path string, packages map[string]string) ([]Site, error) {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return nil, err
	}
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, path, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var sites []Site
	for _, declaration := range file.Decls {
		method, ok := declaration.(*ast.FuncDecl)
		if !ok || !IsDelegation(method, fileSet) {
			continue
		}
		site, err := methodSite(relative, file, method, packages)
		if err != nil {
			return nil, err
		}
		sites = append(sites, site)
	}
	return sites, nil
}

func methodSite(relative string, file *ast.File, method *ast.FuncDecl, packages map[string]string) (Site, error) {
	method.Doc = nil
	var body bytes.Buffer
	if err := format.Node(&body, token.NewFileSet(), method); err != nil {
		return Site{}, err
	}
	receiver := method.Recv.List[0].Type
	if pointer, ok := receiver.(*ast.StarExpr); ok {
		receiver = pointer.X
	}
	var typ bytes.Buffer
	if err := format.Node(&typ, token.NewFileSet(), receiver); err != nil {
		return Site{}, err
	}
	dir := filepath.ToSlash(filepath.Dir(relative))
	return Site{
		ID:    dir + "/" + typ.String() + "." + method.Name.Name,
		Owner: owner(filepath.ToSlash(relative)), Build: buildCategory(file, dir, packages), Form: siteForm(dir, method),
		Evidence: body.String(),
	}, nil
}
