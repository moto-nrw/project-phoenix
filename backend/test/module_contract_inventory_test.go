package test

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type moduleContractEntry struct {
	Module          string `json:"module"`
	ContractPackage string `json:"contract_package"`
	TestFile        string `json:"test_file"`
	Test            string `json:"test"`
}

// TestModuleContractInventoryRatchet pins every current domain/platform to an
// executable test of its public contract. Missing coverage has a
// zero ceiling: there is no exception list or budget that can grow. Paths and
// test names are exact, so deleted, renamed or moved tests require an update.
// The inventory is evidence of a named contract test, not a claim that one
// test exhausts the module's entire API.
func TestModuleContractInventoryRatchet(t *testing.T) {
	t.Parallel()
	root, err := findBackendRoot()
	require.NoError(t, err)
	policyBytes, err := os.ReadFile(filepath.Join(root, "architecture/policy.json"))
	require.NoError(t, err)
	var policy struct {
		ModulePath string `json:"module_path"`
		Owners     []struct {
			ID   string `json:"id"`
			Kind string `json:"kind"`
		} `json:"owners"`
		Packages []struct {
			Path  string `json:"path"`
			Owner string `json:"owner"`
		} `json:"packages"`
	}
	require.NoError(t, json.Unmarshal(policyBytes, &policy))
	inventoryBytes, err := os.ReadFile(filepath.Join(root, "test/module_contract_inventory.json"))
	require.NoError(t, err)
	var inventory []moduleContractEntry
	require.NoError(t, json.Unmarshal(inventoryBytes, &inventory))
	expected := map[string]bool{}
	domains, platforms := 0, 0
	for _, owner := range policy.Owners {
		switch owner.Kind {
		case "domain":
			domains++
			expected[owner.ID] = true
		case "platform":
			platforms++
			expected[owner.ID] = true
		}
	}
	require.Equal(t, 18, domains)
	require.Equal(t, 10, platforms)
	packages := map[string]string{}
	for _, pkg := range policy.Packages {
		packages[pkg.Path] = pkg.Owner
	}
	seen := map[string]bool{}
	for _, entry := range inventory {
		require.True(t, expected[entry.Module], "unknown/stale module %q", entry.Module)
		require.False(t, seen[entry.Module], "duplicate module %q", entry.Module)
		seen[entry.Module] = true
		require.Equal(t, entry.Module, packages[entry.ContractPackage], "contract package belongs to another owner")
		require.NotContains(t, "/"+entry.ContractPackage+"/", "/internal/", "contracts must be public")
		require.True(t, strings.HasSuffix(entry.TestFile, "_test.go"))
		require.True(t, filepath.IsLocal(entry.TestFile), "test path must stay in the backend")
		source, err := os.ReadFile(filepath.Join(root, entry.TestFile))
		require.NoError(t, err)
		require.NoError(t, verifyModuleContractTest(source, entry, policy.ModulePath))
	}
	require.Equal(t, expected, seen, "every domain and platform must have exactly one contract-test entry; missing entries may never grow")
}

func verifyModuleContractTest(source []byte, entry moduleContractEntry, modulePath string) error {
	file, err := parser.ParseFile(token.NewFileSet(), entry.TestFile, source, 0)
	if err != nil {
		return err
	}
	if file.Name.Name != filepath.Base(entry.ContractPackage) && !strings.HasSuffix(file.Name.Name, "_test") {
		return fmt.Errorf("%s is not a contract test package", entry.TestFile)
	}
	importsContract := false
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return err
		}
		if path == modulePath+"/"+entry.ContractPackage {
			importsContract = true
		}
	}
	localContract := filepath.Dir(entry.TestFile) == entry.ContractPackage && file.Name.Name == filepath.Base(entry.ContractPackage)
	if !importsContract && !localContract {
		return fmt.Errorf("%s does not import its public contract %s", entry.TestFile, entry.ContractPackage)
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || fn.Name.Name != entry.Test {
			continue
		}
		if !strings.HasPrefix(fn.Name.Name, "Test") || fn.Body == nil || fn.Type.Params.NumFields() != 1 || fn.Type.Results != nil {
			return fmt.Errorf("%s is not an executable Go test", entry.Test)
		}
		typ, ok := fn.Type.Params.List[0].Type.(*ast.StarExpr)
		if !ok {
			return fmt.Errorf("%s must receive *testing.T", entry.Test)
		}
		target, ok := typ.X.(*ast.SelectorExpr)
		if !ok || target.Sel.Name != "T" {
			return fmt.Errorf("%s must receive *testing.T", entry.Test)
		}
		if !hasObservableContractAssertion(fn.Body) {
			return fmt.Errorf("%s checks no observable result beyond success or presence", entry.Test)
		}
		return nil
	}
	return fmt.Errorf("contract test %s:%s is missing", entry.TestFile, entry.Test)
}

// Constructor presence and err == nil do not prove the contract's output.
func hasObservableContractAssertion(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		receiver, ok := selector.X.(*ast.Ident)
		if !ok {
			return true
		}
		switch receiver.Name {
		case "assert", "require":
			switch selector.Sel.Name {
			case "NoError", "NotNil", "NotZero", "NotEmpty":
			default:
				found = true
			}
		case "t":
			switch selector.Sel.Name {
			case "Fatalf", "Errorf", "Fatal", "Error":
				found = true
			}
		}
		return true
	})
	return found
}

func TestModuleContractInventoryRejectsStaleOrPrivateEvidence(t *testing.T) {
	t.Parallel()
	entry := moduleContractEntry{ContractPackage: "modules/example", TestFile: "modules/example/contract_test.go", Test: "TestContract"}
	for name, source := range map[string]string{
		"empty test":        `package example_test; import "github.com/moto-nrw/project-phoenix/modules/example";func TestContract(t *testing.T){}`,
		"success only":      `package example_test; import "github.com/moto-nrw/project-phoenix/modules/example";func TestContract(t *testing.T){require.NoError(t, call())}`,
		"method value only": `package example_test; import "github.com/moto-nrw/project-phoenix/modules/example";func TestContract(t *testing.T){require.NotNil(t, module.Read)}`,

		"private package": `package example_internal; import "github.com/moto-nrw/project-phoenix/modules/example";func TestContract(t *testing.T){}`,
		"wrong contract":  `package example_test; import "github.com/moto-nrw/project-phoenix/modules/other";func TestContract(t *testing.T){}`,
		"renamed test":    `package example_test; import "github.com/moto-nrw/project-phoenix/modules/example";func TestRenamed(t *testing.T){}`,
		"not a Go test":   `package example_test; import "github.com/moto-nrw/project-phoenix/modules/example";func TestContract(){}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Error(t, verifyModuleContractTest([]byte(source), entry, "github.com/moto-nrw/project-phoenix"))
		})
	}
}

func TestModuleContractInventoryAcceptsObservablePublicEvidence(t *testing.T) {
	t.Parallel()
	entry := moduleContractEntry{ContractPackage: "modules/example", TestFile: "modules/example/contract_test.go", Test: "TestContract"}
	source := `package example_test; import "github.com/moto-nrw/project-phoenix/modules/example";func TestContract(t *testing.T){result,err:=module.Read();require.NoError(t,err);require.Equal(t,"expected",result)}`
	require.NoError(t, verifyModuleContractTest([]byte(source), entry, "github.com/moto-nrw/project-phoenix"))
}
