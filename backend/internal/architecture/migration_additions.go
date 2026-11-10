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

// migrationCreateDataObjects inspects statically resolved SQL passed to NewRaw,
// Exec, Query or Prepare. Unresolved SQL fails closed in candidate migrations.
func migrationCreateDataObjects(filename string, contents []byte) (map[string]struct{}, map[string]string, error) {
	statements, err := migrationStatements(filename, contents)
	result := make(map[string]struct{})
	renamed := make(map[string]string)
	var problems []error
	if err != nil {
		problems = append(problems, err)
	}
	for _, sql := range statements {
		objects, renames, sqlErr := migrationSQLChanges(sql)
		if sqlErr != nil {
			problems = append(problems, fmt.Errorf("candidate migration %s: %w", filename, sqlErr))
			continue
		}
		maps.Copy(result, objects)
		maps.Copy(renamed, renames)
	}
	return result, renamed, errors.Join(problems...)
}

// migrationStatements resolves the SQL of every NewRaw, Exec, Query and
// Prepare call of a migration file; a call whose SQL is not static, or a
// schema builder, is a problem.
func migrationStatements(filename string, contents []byte) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, contents, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("parse candidate migration %s: %w", filename, err)
	}
	queries := analyzeMigrationQueries(file, fset)
	var statements []string
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
		sqlArgument, ok := migrationSQLArgument(queries, selector)
		if !ok || len(call.Args) <= sqlArgument {
			return true
		}
		sql, ok := queries.expression(call.Args[sqlArgument], 0)
		if !ok {
			problems = append(problems, fmt.Errorf("candidate migration %s: SQL argument cannot be resolved statically", filename))
			return true
		}
		statements = append(statements, sql)
		return true
	})
	return statements, errors.Join(problems...)
}

// migrationSQLArgument is the position of the SQL among the arguments of a
// call that runs or prepares SQL; ok is false for any other call.
func migrationSQLArgument(queries *migrationQueries, selector *ast.SelectorExpr) (int, bool) {
	switch selector.Sel.Name {
	case "ExecContext", "QueryContext", "QueryRowContext", "PrepareContext":
		return 1, true
	case "Exec", "Query", "QueryRow", "Prepare":
		return 0, !queries.builder(selector.X, 0)
	case "NewRaw":
		return 0, true
	}
	return 0, false
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
// names the object creates or renames it, or cannot be classified. A file
// that names the object anywhere outside the SQL the parser resolved (a
// call it does not know, a helper, a comment) holds it too: only a file
// whose every mention is read and creates nothing lets it go.
func baseMigrationHoldsDataObject(root, blob, object string) (bool, error) {
	path := blob[strings.Index(blob, ":")+1:]
	if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
		return true, nil
	}
	contents, err := gitOutput(root, "cat-file", "blob", blob)
	if err != nil {
		return false, fmt.Errorf("read base migration %s: %w", blob, err)
	}
	statements, err := migrationStatements(path, []byte(contents))
	if err != nil {
		return true, nil //nolint:nilerr // an unclassifiable base file keeps holding the object
	}
	mention := dataObjectMention(object)
	read := 0
	for _, sql := range statements {
		objects, renames, sqlErr := migrationSQLChanges(sql)
		if sqlErr != nil {
			return true, nil //nolint:nilerr // an unclassifiable statement keeps holding the object
		}
		_, creates := objects[object]
		_, renamesTo := renames[object]
		if creates || renamesTo {
			return true, nil
		}
		read += len(mention.FindAllStringIndex(sql, -1))
	}
	return len(mention.FindAllStringIndex(contents, -1)) > read, nil
}

// dataObjectMention matches schema.name as the base grep does: case
// insensitive, quoted or not, with space around the dot.
func dataObjectMention(object string) *regexp.Regexp {
	parts := strings.SplitN(object, ".", 2)
	return regexp.MustCompile(fmt.Sprintf(`(?i)(^|[^[:alnum:]_$])"?%s"?[[:space:]]*\.[[:space:]]*"?%s"?([^[:alnum:]_$]|$)`,
		regexp.QuoteMeta(parts[0]), regexp.QuoteMeta(parts[1])))
}
