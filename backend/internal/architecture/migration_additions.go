package architecture

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// candidateMigrationDataObjects returns schema-qualified tables and views created by Go
// migration files that are new relative to the immutable PR base. Modified
// historical migrations are deliberately excluded: they cannot be used to
// backfill ownership for an already-existing table. The second result holds
// the tables those files rename, keyed by the new name and valued by the old
// one; a renamed table keeps its write owner (ownershipLoosenings).
func candidateMigrationDataObjects(project, ref string) (map[string]struct{}, map[string]string, error) {
	result := make(map[string]struct{})
	renamed := make(map[string]string)
	migrationDir := filepath.Join(project, "database", "migrations")
	if _, err := os.Stat(migrationDir); err != nil {
		if os.IsNotExist(err) {
			return result, renamed, nil
		}
		return nil, nil, fmt.Errorf("inspect migration directory: %w", err)
	}

	root, err := gitOutput(project, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, nil, fmt.Errorf("resolve repository root for migration additions: %w", err)
	}
	root = strings.TrimSpace(root)
	sha, err := resolveBaseCommit(root, ref)
	if err != nil {
		return nil, nil, err
	}
	relativeMigrationDir, err := repositoryRelativePath(root, project, migrationDir)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve migration directory: %w", err)
	}

	changed, err := gitOutput(root, "diff", "--name-only", "--diff-filter=A", "-z", sha, "--", filepath.ToSlash(relativeMigrationDir))
	if err != nil {
		return nil, nil, fmt.Errorf("list candidate migrations: %w", err)
	}
	untracked, err := gitOutput(root, "ls-files", "--others", "--exclude-standard", "-z", "--", filepath.ToSlash(relativeMigrationDir))
	if err != nil {
		return nil, nil, fmt.Errorf("list untracked candidate migrations: %w", err)
	}
	for _, relativeFile := range strings.Split(changed+untracked, "\x00") {
		if relativeFile == "" || filepath.Ext(relativeFile) != ".go" || strings.HasSuffix(relativeFile, "_test.go") {
			continue
		}
		if err := addCandidateMigration(root, sha, relativeMigrationDir, relativeFile, result, renamed); err != nil {
			return nil, nil, err
		}
	}
	return result, renamed, nil
}

// addCandidateMigration adds the data objects one new migration file creates
// or renames, unless a base migration already mentions them.
func addCandidateMigration(root, sha, migrationDir, relativeFile string, created map[string]struct{}, renamed map[string]string) error {
	contents, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relativeFile)))
	if err != nil {
		return fmt.Errorf("read candidate migration %s: %w", relativeFile, err)
	}
	objects, renames, err := migrationCreateDataObjects(relativeFile, contents)
	if err != nil {
		return err
	}
	for object := range objects {
		mentioned, err := baseMigrationsMentionDataObject(root, sha, migrationDir, object)
		if err != nil {
			return err
		}
		if !mentioned {
			created[object] = struct{}{}
		}
	}
	for object, previous := range renames {
		mentioned, err := baseMigrationsMentionDataObject(root, sha, migrationDir, object)
		if err != nil {
			return err
		}
		if !mentioned {
			renamed[object] = previous
		}
	}
	return nil
}

// migrationCreateDataObjects inspects statically resolved SQL passed to NewRaw
// or ExecContext. Unresolved SQL fails closed in candidate migrations.
func migrationCreateDataObjects(filename string, contents []byte) (map[string]struct{}, map[string]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, contents, parser.SkipObjectResolution)
	if err != nil {
		return nil, nil, fmt.Errorf("parse candidate migration %s: %w", filename, err)
	}
	result := make(map[string]struct{})
	renamed := make(map[string]string)
	queries := analyzeMigrationQueries(file, fset)
	var problems []error
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector := queries.selector(call.Fun, 0)
		if selector == nil {
			return true
		}
		if selector.Sel.Name == "NewCreateTable" || selector.Sel.Name == "NewCreateView" {
			problems = append(problems, fmt.Errorf("candidate migration %s: schema builder cannot be classified; use static SQL", filename))
			return true
		}
		sqlArgument := 0
		switch selector.Sel.Name {
		case "ExecContext", "QueryContext", "QueryRowContext":
			sqlArgument = 1
		case "Exec", "Query", "QueryRow":
			if queries.builder(selector.X, 0) {
				return true
			}
		case "NewRaw":
		default:
			return true
		}
		if len(call.Args) <= sqlArgument {
			return true
		}
		sql, ok := queries.expression(call.Args[sqlArgument], 0)
		if !ok {
			problems = append(problems, fmt.Errorf("candidate migration %s: SQL argument cannot be resolved statically", filename))
			return true
		}
		objects, renames, sqlErr := migrationSQLChanges(sql)
		if sqlErr != nil {
			problems = append(problems, fmt.Errorf("candidate migration %s: %w", filename, sqlErr))
			return true
		}
		maps.Copy(result, objects)
		maps.Copy(renamed, renames)
		return true
	})
	return result, renamed, errors.Join(problems...)
}

// baseMigrationsMentionDataObject reports whether the base already holds the
// object: a base migration file that names it counts, unless every such file
// is a Go migration whose statically resolved SQL neither creates nor renames
// it. A guarded forward reference of a base migration (a DELETE behind
// to_regclass for a table a later migration creates, 1.15.843 for #6361)
// therefore does not hold the table; a file the parser cannot classify, a test
// or a non-Go file still does, so the guard fails closed.
func baseMigrationsMentionDataObject(root, sha, migrationDir, object string) (bool, error) {
	parts := strings.SplitN(object, ".", 2)
	if len(parts) != 2 {
		return false, fmt.Errorf("inspect base migrations: invalid data object %q", object)
	}
	pattern := fmt.Sprintf(`(^|[^[:alnum:]_$])"?%s"?[[:space:]]*\.[[:space:]]*"?%s"?([^[:alnum:]_$]|$)`, regexp.QuoteMeta(parts[0]), regexp.QuoteMeta(parts[1]))
	command := exec.Command("git", "-C", root, "grep", "-E", "-i", "-l", "-z", pattern, sha, "--", filepath.ToSlash(migrationDir))
	output, err := command.CombinedOutput()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, fmt.Errorf("inspect base migrations for %s: %w: %s", object, err, strings.TrimSpace(string(output)))
	}
	for _, blob := range strings.Split(string(output), "\x00") {
		if blob == "" {
			continue
		}
		held, err := baseMigrationHoldsDataObject(root, blob, object)
		if err != nil || held {
			return true, err
		}
	}
	return false, nil
}

// baseMigrationHoldsDataObject reports whether one base migration file that
// names the object creates or renames it, or cannot be classified.
func baseMigrationHoldsDataObject(root, blob, object string) (bool, error) {
	path := blob[strings.Index(blob, ":")+1:]
	if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
		return true, nil
	}
	contents, err := gitOutput(root, "cat-file", "blob", blob)
	if err != nil {
		return false, fmt.Errorf("read base migration %s: %w", blob, err)
	}
	created, renamed, err := migrationCreateDataObjects(path, []byte(contents))
	if err != nil {
		return true, nil //nolint:nilerr // an unclassifiable base file keeps holding the object
	}
	_, creates := created[object]
	_, renames := renamed[object]
	return creates || renames, nil
}
