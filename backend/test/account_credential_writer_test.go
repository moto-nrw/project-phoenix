package test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The credential of an account is auth.accounts.password_hash, the
// one-time-password flag beside it and the credential generation
// (mfa_reset_generation) that every artifact of a password check is bound
// to: MFA codes, enrollment and challenge tokens, remembered devices, reset
// links and sessions (#6289). A second place that writes one of them can move
// the password without moving the generation, and every artifact of the old
// password stays valid; review found three such paths in a row. So exactly
// one function writes them, and this test keeps it that way.
//
// It scans every production Go file of the backend (tests, migrations, the
// test support in test/ and the dev tools excepted) for
//   - a SQL string that sets, inserts or increments a credential column, and
//   - a BUN row field mapped to a credential column of auth.accounts that a
//     model insert or update could write (without scanonly).
//
// Operators keep their own credential in platform.operators (#6308) and are
// not in scope. There is no register: a second writer routes through the one
// below instead.
//
// This is the early, static warning. The rule itself is enforced at run time
// by the database for every writer, raw SQL in any quoting and ORM model
// updates alike: the trigger auth.account_credential_moves_generation
// (migration 1.15.828) refuses a credential change that does not move the
// generation on, tested in
// database/migrations/001015828_account_credential_generation_binding_test.go.
const accountCredentialWriter = "modules/identityaccess/internal/adapters/postgres/account_credential.go:(*Store).WriteAccountCredential"

// accountCredentialStatement matches raw SQL that writes auth.accounts: the
// SET list of an UPDATE, or the column list of an INSERT.
var accountCredentialStatement = regexp.MustCompile(`(?is)update\s+auth\.accounts\b.*?\bset\b(.*?)(\bwhere\b|\breturning\b|\bfrom\b|$)|insert\s+into\s+auth\.accounts\s*\(([^)]*)\)`)

// accountCredentialAssignment matches an assignment to a credential column,
// optionally qualified by a table alias, as a SET list or a BUN Set fragment
// writes it.
var accountCredentialAssignment = regexp.MustCompile(`(?i)(^|[\s,])("?\w+"?\.)?"?(password_hash|is_password_otp|mfa_reset_generation)"?\s*=[^=]`)

var accountCredentialColumnName = regexp.MustCompile(`(?i)\b(password_hash|is_password_otp|mfa_reset_generation)\b`)

// writesAccountCredential reports whether a raw SQL string writes a
// credential column of auth.accounts.
func writesAccountCredential(sql string) bool {
	for _, match := range accountCredentialStatement.FindAllStringSubmatch(sql, -1) {
		if accountCredentialAssignment.MatchString(match[1]) || accountCredentialColumnName.MatchString(match[3]) {
			return true
		}
	}
	return false
}

// bunWriteMethods are the query builder methods whose first argument names a
// column the statement writes.
var bunWriteMethods = map[string]bool{"Set": true, "SetColumn": true, "Value": true}

var accountCredentialColumns = map[string]bool{"password_hash": true, "is_password_otp": true, "mfa_reset_generation": true}

func TestAccountCredentialHasOneWriter(t *testing.T) {
	t.Parallel()
	root, err := findBackendRoot()
	if err != nil {
		t.Fatal(err)
	}
	writers, rowFields, err := accountCredentialWrites(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(writers) != 1 || writers[0] != accountCredentialWriter {
		t.Errorf("the account credential must have exactly one writer, %s; found:\n  %s\n"+
			"Route the write through WriteAccountCredential: it moves the credential generation on in the same statement.",
			accountCredentialWriter, strings.Join(writers, "\n  "))
	}
	if len(rowFields) > 0 {
		t.Errorf("row fields of auth.accounts map a credential column without scanonly, so a model insert or update would write it:\n  %s",
			strings.Join(rowFields, "\n  "))
	}
}

// accountCredentialWrites returns the functions holding a credential write
// and the writable row fields of credential columns, both sorted.
func accountCredentialWrites(root string) ([]string, []string, error) {
	writers, fields := map[string]bool{}, map[string]bool{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel := filepath.ToSlash(strings.TrimPrefix(path, root+string(os.PathSeparator)))
		if entry.IsDir() {
			if strings.HasPrefix(entry.Name(), ".") || rel == "test" || rel == "database/migrations" || rel == "internal/devtools" || entry.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		for _, writer := range credentialWritersIn(file, rel) {
			writers[writer] = true
		}
		for _, field := range writableCredentialFieldsIn(file, rel) {
			fields[field] = true
		}
		return nil
	})
	return sortedKeys(writers), sortedKeys(fields), err
}

func credentialWritersIn(file *ast.File, rel string) []string {
	var found []string
	for _, decl := range file.Decls {
		name := declarationName(decl)
		ast.Inspect(decl, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.BasicLit:
				if text, ok := stringLiteral(node); ok && writesAccountCredential(text) {
					found = append(found, rel+":"+name)
				}
			case *ast.CallExpr:
				selector, ok := node.Fun.(*ast.SelectorExpr)
				if !ok || !bunWriteMethods[selector.Sel.Name] || len(node.Args) == 0 {
					return true
				}
				if literal, ok := node.Args[0].(*ast.BasicLit); ok {
					if text, ok := stringLiteral(literal); ok && bunCredentialFragment(text) {
						found = append(found, rel+":"+name)
					}
				}
			}
			return true
		})
	}
	return found
}

func stringLiteral(literal *ast.BasicLit) (string, bool) {
	if literal.Kind != token.STRING {
		return "", false
	}
	text, err := strconv.Unquote(literal.Value)
	return text, err == nil
}

// bunCredentialFragment reports whether the first argument of a BUN Set or
// Value call names a credential column.
func bunCredentialFragment(fragment string) bool {
	trimmed := strings.TrimSpace(fragment)
	if accountCredentialColumns[strings.Trim(trimmed, `"`)] {
		return true
	}
	return accountCredentialAssignment.MatchString(" " + trimmed)
}

func declarationName(decl ast.Decl) string {
	fn, ok := decl.(*ast.FuncDecl)
	if !ok {
		return "(package level)"
	}
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	receiver := fn.Recv.List[0].Type
	if star, ok := receiver.(*ast.StarExpr); ok {
		if ident, ok := star.X.(*ast.Ident); ok {
			return "(*" + ident.Name + ")." + fn.Name.Name
		}
	}
	if ident, ok := receiver.(*ast.Ident); ok {
		return ident.Name + "." + fn.Name.Name
	}
	return fn.Name.Name
}

// writableCredentialFieldsIn reports struct fields mapped to a credential
// column without scanonly in a row type that is not an operator row: a BUN
// model insert or update writes every mapped column.
func writableCredentialFieldsIn(file *ast.File, rel string) []string {
	var found []string
	ast.Inspect(file, func(node ast.Node) bool {
		spec, ok := node.(*ast.TypeSpec)
		if !ok {
			return true
		}
		structType, ok := spec.Type.(*ast.StructType)
		if !ok || operatorRow(structType) {
			return true
		}
		for _, field := range structType.Fields.List {
			if field.Tag == nil {
				continue
			}
			tag, err := strconv.Unquote(field.Tag.Value)
			if err != nil {
				continue
			}
			options := strings.Split(reflect.StructTag(tag).Get("bun"), ",")
			if accountCredentialColumns[options[0]] && !containsOption(options[1:], "scanonly") {
				found = append(found, rel+":"+spec.Name.Name+"."+options[0])
			}
		}
		return true
	})
	return found
}

func operatorRow(structType *ast.StructType) bool {
	for _, field := range structType.Fields.List {
		if field.Tag != nil && strings.Contains(field.Tag.Value, "table:platform.operators") {
			return true
		}
	}
	return false
}

func containsOption(options []string, option string) bool {
	for _, candidate := range options {
		if candidate == option {
			return true
		}
	}
	return false
}

func sortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
