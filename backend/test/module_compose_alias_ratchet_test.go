package test

import (
	"fmt"
	"go/ast"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestModuleComposeAliasRatchet counts the vocabulary aliases the owner
// compositions hand out (#4085): a type alias or a value re-export in a
// composition that names a package outside its owner. Such an alias lets a
// consumer use a foreign type, often a models/* row, without the import the
// architecture ratchet would see (ADR 0050, ADR 0053). Consumers get their own
// values through ports or public contracts instead, and the alias packages
// (compose/vocabulary, compose/retained) are deleted. The register only falls;
// #4085 is done when it is empty.
//
// What counts, per declaration in a non-test .go file:
//
//  1. The file sits in a composition: a directory named compose or
//     composition, or below one, anywhere in the backend tree. Its owner is
//     the path up to that directory ("modules/settings/compose/vocabulary"
//     belongs to "modules/settings").
//  2. A type alias (`type X = pkg.Y`, also instantiated: `pkg.Y[T]`) or a
//     value spec whose value is a bare selector (`var X = pkg.Y`) into an
//     imported package of this module.
//  3. That package is not the owner and not below it. Aliases of the owner's
//     own packages (compose naming its internal application types) do not
//     count here; they are #4084's and the owner's business.
//
// Form: a budget per composition package, keyed by its backend-relative
// directory. A package without aliases has no entry.
var moduleComposeAliasBudgets = map[string]int{
	"modules/auditlog/compose":                   8,
	"modules/communication/composition":          4,
	"modules/dataimport/compose/retained":        16,
	"modules/delivery/compose/email":             23,
	"modules/delivery/compose/realtime":          17,
	"modules/peopledirectory/compose/vocabulary": 88,
	"modules/requestreview/compose":              6,
	"modules/schoolstructure/compose":            2,
	"modules/settings/compose":                   27,
	"modules/timetable/compose/httpadapter":      1,
	"observability/compose":                      1,
	"workflows/staffclock/compose":               19,
}

// moduleComposeAliasBudgetTotal is the sum of moduleComposeAliasBudgets.
const moduleComposeAliasBudgetTotal = 212

const backendModulePath = "github.com/moto-nrw/project-phoenix/"

func TestModuleComposeAliasRatchet(t *testing.T) {
	t.Parallel()

	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Skipf("Could not find backend root: %v", err)
		return
	}

	counts, err := moduleComposeAliasScan(backendRoot)
	if err != nil {
		t.Fatalf("compose alias scan failed: %v", err)
	}

	var violations []string
	for pkg, got := range counts {
		budget, ok := moduleComposeAliasBudgets[pkg]
		switch {
		case !ok:
			violations = append(violations, fmt.Sprintf(
				"[compose alias] %s: %d alias(es) of foreign packages, but the package has no budget.\n"+
					"  A composition hands out no foreign vocabulary: give the consumer its own value through a port\n"+
					"  or a public contract (#4085). Never add an entry.", pkg, got))
		case got > budget:
			violations = append(violations, fmt.Sprintf(
				"[compose alias] %s: %d alias(es) of foreign packages, budget %d.\n"+
					"  Aliases were added. Never raise a budget; a raise needs an ADR (ADR 0057 point 5).", pkg, got, budget))
		case got < budget:
			violations = append(violations, fmt.Sprintf(
				"[compose alias] %s: %d alias(es) of foreign packages, budget says %d.\n"+
					"  Nice — lower the budget to %d so the reduction cannot regress.", pkg, got, budget, got))
		}
	}
	for pkg, budget := range moduleComposeAliasBudgets {
		if _, ok := counts[pkg]; !ok {
			violations = append(violations, fmt.Sprintf(
				"[compose alias] %s: budget %d, but no alias is left (deleted, moved or renamed?).\n"+
					"  Remove the entry.", pkg, budget))
		}
	}
	sum := 0
	for _, budget := range moduleComposeAliasBudgets {
		sum += budget
	}
	switch {
	case sum > moduleComposeAliasBudgetTotal:
		violations = append(violations, fmt.Sprintf(
			"[compose alias] moduleComposeAliasBudgetTotal: the entries sum to %d, the total is %d.\n"+
				"  An entry was raised. Never raise a budget; a raise needs an ADR (ADR 0057 point 5).",
			sum, moduleComposeAliasBudgetTotal))
	case sum < moduleComposeAliasBudgetTotal:
		violations = append(violations, fmt.Sprintf(
			"[compose alias] moduleComposeAliasBudgetTotal: the entries sum to %d, the total is %d.\n"+
				"  Nice — lower moduleComposeAliasBudgetTotal to %d so the reduction cannot regress.",
			sum, moduleComposeAliasBudgetTotal, sum))
	}
	if len(violations) > 0 {
		sort.Strings(violations)
		t.Errorf("Compose alias ratchet check failed (%d issue(s)):\n\n%s", len(violations), strings.Join(violations, "\n\n"))
	}
}

// moduleComposeAliasOwner returns the owner of a backend-relative directory
// inside a composition, or "" when the directory is in none.
func moduleComposeAliasOwner(dir string) string {
	parts := strings.Split(dir, "/")
	for index, part := range parts {
		if index > 0 && (part == "compose" || part == "composition") {
			return strings.Join(parts[:index], "/")
		}
	}
	return ""
}

// moduleComposeAliasScan counts the foreign aliases per composition package.
// Packages without one do not appear, which makes a stale budget detectable.
func moduleComposeAliasScan(backendRoot string) (map[string]int, error) {
	counts := make(map[string]int)
	err := filepath.WalkDir(backendRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if path != backendRoot && (strings.HasPrefix(name, ".") || name == "tmp" || name == "node_modules" || name == "vendor") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, relErr := filepath.Rel(backendRoot, filepath.Dir(path))
		if relErr != nil {
			return relErr
		}
		dir := filepath.ToSlash(rel)
		owner := moduleComposeAliasOwner(dir)
		if owner == "" {
			return nil
		}
		_, file, parseErr := parseGoSourceCached(path, nil)
		if parseErr != nil {
			return fmt.Errorf("parse %s: %w", path, parseErr)
		}
		if n := moduleComposeForeignAliases(file, owner); n > 0 {
			counts[dir] += n
		}
		return nil
	})
	return counts, err
}

// moduleComposeForeignAliases counts the declarations of file that alias a
// package outside owner.
func moduleComposeForeignAliases(file *ast.File, owner string) int {
	imports := make(map[string]string)
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name := filepath.Base(path)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		imports[name] = path
	}
	foreign := func(expr ast.Expr) bool {
		switch typed := expr.(type) {
		case *ast.IndexExpr:
			expr = typed.X
		case *ast.IndexListExpr:
			expr = typed.X
		}
		selector, ok := expr.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		ident, ok := selector.X.(*ast.Ident)
		if !ok {
			return false
		}
		path, ok := imports[ident.Name]
		if !ok || !strings.HasPrefix(path, backendModulePath) {
			return false
		}
		target := strings.TrimPrefix(path, backendModulePath)
		return target != owner && !strings.HasPrefix(target, owner+"/")
	}
	count := 0
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gen.Specs {
			switch typed := spec.(type) {
			case *ast.TypeSpec:
				if typed.Assign.IsValid() && foreign(typed.Type) {
					count++
				}
			case *ast.ValueSpec:
				for _, value := range typed.Values {
					if _, ok := value.(*ast.SelectorExpr); ok && foreign(value) {
						count++
					}
				}
			}
		}
	}
	return count
}

func TestModuleComposeAliasScanCountsForeignAliasesOnly(t *testing.T) {
	t.Parallel()

	if got := moduleComposeAliasOwner("modules/settings/compose/vocabulary"); got != "modules/settings" {
		t.Fatalf("owner of a vocabulary package = %q, want modules/settings", got)
	}
	if got := moduleComposeAliasOwner("modules/settings/internal/application"); got != "" {
		t.Fatalf("owner outside a composition = %q, want none", got)
	}

	source := `package vocabulary

import (
	users "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/settings/internal/application"
	importService "github.com/moto-nrw/project-phoenix/services/import"
	"time"
)

type Person = users.Person
type Service = application.Service
type Generic = importService.ImportService[int]
type Clock = time.Duration
type Own struct{ users.Person }

var NewPerson = users.NewPerson
var local = application.New
var computed = users.NewPerson()
`
	_, file, err := parseGoSourceCached("compose_alias_ratchet_fixture.go", source)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	// Person, Generic and NewPerson are foreign; the owner's own
	// application, the standard library, a defined type and a call do not
	// count.
	if got := moduleComposeForeignAliases(file, "modules/settings"); got != 3 {
		t.Fatalf("foreign aliases = %d, want 3", got)
	}
}
