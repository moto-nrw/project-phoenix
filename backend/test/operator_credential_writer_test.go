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
	"strconv"
	"strings"
	"testing"
)

// The credential of an operator is platform.operators.password_hash and the
// credential generation beside it, which the e-mail codes, enrollment and
// challenge tokens and remembered devices of a password check are bound to
// (#6308). A second place that writes the password can move it without
// moving the generation, and every artifact of the old password stays valid.
// So exactly one function writes them after the insert that creates the
// operator, and this test keeps it that way, like
// TestAccountCredentialHasOneWriter does for accounts.
//
// It scans every production Go file of the backend (tests, migrations, the
// test support in test/ and the dev tools excepted) for
//   - a SQL string that updates a credential column of platform.operators,
//   - a BUN row of platform.operators that maps the generation writable, and
//   - a BUN row of platform.operators whose package updates the model
//     without excluding the password.
//
// A BUN Set or Value naming password_hash is already a finding of
// TestAccountCredentialHasOneWriter, whatever its table: the one operator
// writer is raw SQL for that reason.
//
// The rule itself is enforced at run time by the database for every writer:
// the trigger platform.operator_credential_moves_generation (migration
// 1.15.877) refuses a password change that does not move the generation on,
// tested in
// database/migrations/001015877_operator_credential_generation_binding_test.go.
const operatorCredentialWriter = "modules/identityaccess/internal/adapters/postgres/operator_credential.go:(*Store).WriteOperatorCredential"

// operatorCredentialStatement matches raw SQL that updates platform.operators,
// quoted or not: its SET list.
var operatorCredentialStatement = regexp.MustCompile(`(?is)update\s+"?platform"?\."?operators"?(\s|$).*?\bset\b(.*?)(\bwhere\b|\breturning\b|\bfrom\b|$)`)

var operatorCredentialAssignment = regexp.MustCompile(`(?i)(^|[\s,])("?\w+"?\.)?"?(password_hash|credential_generation)"?\s*=[^=]`)

// operatorModelUpdate matches a BUN update of a whole row model, the shape
// that writes every mapped column.
var operatorModelUpdate = regexp.MustCompile(`NewUpdate\(\)\.Model\(&`)

func writesOperatorCredential(sql string) bool {
	for _, match := range operatorCredentialStatement.FindAllStringSubmatch(sql, -1) {
		if operatorCredentialAssignment.MatchString(match[2]) {
			return true
		}
	}
	return false
}

func TestOperatorCredentialHasOneWriter(t *testing.T) {
	t.Parallel()
	root, err := findBackendRoot()
	if err != nil {
		t.Fatal(err)
	}
	writers, rowFindings, err := operatorCredentialWrites(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(writers) != 1 || writers[0] != operatorCredentialWriter {
		t.Errorf("the operator credential must have exactly one writer, %s; found:\n  %s\n"+
			"Route the write through WriteOperatorCredential: it moves the credential generation on in the same statement.",
			operatorCredentialWriter, strings.Join(writers, "\n  "))
	}
	if len(rowFindings) > 0 {
		t.Errorf("rows of platform.operators would write the credential in a model update:\n  %s",
			strings.Join(rowFindings, "\n  "))
	}
}

func TestOperatorCredentialStatementRecognizesWriters(t *testing.T) {
	t.Parallel()
	for statement, want := range map[string]bool{
		`UPDATE platform.operators SET password_hash = ? WHERE id = ?`:                                true,
		`UPDATE "platform"."operators" AS "o" SET "password_hash" = ? WHERE "o"."id" = ?`:             true,
		`update platform.operators set credential_generation = credential_generation + 1`:             true,
		`UPDATE platform.operators SET password_attempts = ?, password_locked_until = ? WHERE id = ?`: false,
		`UPDATE platform.operator_mfa_trusted_devices SET credential_generation = 0`:                  false,
		`SELECT password_hash FROM platform.operators WHERE password_hash = ?`:                        false,
	} {
		if got := writesOperatorCredential(statement); got != want {
			t.Errorf("writesOperatorCredential(%q) = %v, want %v", statement, got, want)
		}
	}
}

// operatorCredentialWrites returns the functions holding a credential
// update and the row findings, both sorted.
func operatorCredentialWrites(root string) ([]string, []string, error) {
	writers, findings := map[string]bool{}, map[string]bool{}
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
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, source, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		for _, decl := range file.Decls {
			name := declarationName(decl)
			ast.Inspect(decl, func(node ast.Node) bool {
				if literal, ok := node.(*ast.BasicLit); ok {
					if text, ok := stringLiteral(literal); ok && writesOperatorCredential(text) {
						writers[rel+":"+name] = true
					}
				}
				return true
			})
		}
		for _, finding := range operatorRowFindings(file, rel, string(source)) {
			findings[finding] = true
		}
		return nil
	})
	return sortedKeys(writers), sortedKeys(findings), err
}

// operatorRowFindings reports a row of platform.operators that maps the
// generation writable, and a model update in the row's file that does not
// exclude the password: the insert creating an operator stores its first
// password, every later write is WriteOperatorCredential's.
func operatorRowFindings(file *ast.File, rel, source string) []string {
	var found []string
	ast.Inspect(file, func(node ast.Node) bool {
		spec, ok := node.(*ast.TypeSpec)
		if !ok {
			return true
		}
		structType, ok := spec.Type.(*ast.StructType)
		if !ok || !operatorRow(structType) {
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
			if options[0] == "credential_generation" && !containsOption(options[1:], "scanonly") {
				found = append(found, rel+":"+spec.Name.Name+".credential_generation")
			}
		}
		for _, location := range operatorModelUpdate.FindAllStringIndex(source, -1) {
			statementEnd := strings.Index(source[location[1]:], "Exec(")
			if statementEnd < 0 || !strings.Contains(source[location[1]:location[1]+statementEnd], `ExcludeColumn("password_hash"`) {
				found = append(found, rel+":"+spec.Name.Name+" model update without ExcludeColumn(\"password_hash\")")
			}
		}
		return true
	})
	return found
}
