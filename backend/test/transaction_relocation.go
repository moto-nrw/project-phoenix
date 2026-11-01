package test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

type transactionFingerprint func(key string, before bool) (string, error)

func transactionScopeSubset(current, previous map[string]string, moves map[string]string, fingerprint transactionFingerprint, receiverRemoved func(string, string, string) (bool, error)) error {
	current = withoutReviewedSchoolVacationTransactions(withoutReviewedPayrollDispatchTransactions(withoutReviewedParentDocumentInboxTransactions(withoutReviewedHolidayContributionTransactions(withoutReviewedHolidayRegistrationPrefillTransactions(withoutReviewedHolidayRegistrationTransactions(withoutReviewedTraegerTransactions(withoutReviewedTraegerViewTransactions(withoutReviewedTenantExportTransaction(withoutReviewedCaregiverDisableTransaction(current))))))))))
	for key, reason := range current {
		if old, retained := previous[key]; retained {
			if old == reason {
				continue
			}
			return fmt.Errorf("transaction register changed reason: %s", key)
		}
		oldKey, err := originalReceiverTransactionKey(key, current, previous, receiverRemoved)
		if err != nil {
			return err
		}
		receiverMove := oldKey != ""
		if !receiverMove {
			oldKey = originalTransactionKey(key, moves)
		}
		oldReason, existed := previous[oldKey]
		_, copied := current[oldKey]
		if !existed || copied || oldReason != reason {
			return fmt.Errorf("transaction register may only shrink or relocate unchanged entries: %s", key)
		}
		before, err := fingerprint(oldKey, true)
		if err != nil {
			return err
		}
		after, err := fingerprint(key, false)
		if err != nil {
			return err
		}
		if !receiverMove && before != after {
			return fmt.Errorf("relocated transaction body changed: %s", key)
		}
	}
	return nil
}

func originalTransactionKey(key string, moves map[string]string) string {
	file, operation, ok := strings.Cut(key, ":")
	if !ok || filepath.Ext(file) != ".go" {
		return ""
	}
	for from, to := range moves {
		if filepath.ToSlash(filepath.Dir(file)) == to {
			return from + "/" + filepath.Base(file) + ":" + operation
		}
	}
	return ""
}

func transactionFingerprints(root, base string, git func(...string) ([]byte, error)) transactionFingerprint {
	cache := map[string]map[string]string{}
	return func(key string, before bool) (string, error) {
		file, operation, ok := strings.Cut(key, ":")
		separator := strings.LastIndex(operation, ":")
		if !ok || separator < 0 {
			return "", fmt.Errorf("invalid transaction key: %s", key)
		}
		cacheKey := fmt.Sprintf("%t:%s", before, file)
		functions, loaded := cache[cacheKey]
		if !loaded {
			var raw []byte
			var err error
			if before {
				raw, err = git("show", base+":backend/"+file)
			} else {
				raw, err = os.ReadFile(filepath.Join(root, file))
			}
			if err != nil {
				return "", err
			}
			functions, err = transactionFunctionFingerprints(file, raw)
			if err != nil {
				return "", err
			}
			cache[cacheKey] = functions
		}
		fingerprint, exists := functions[operation[:separator]]
		if !exists {
			return "", fmt.Errorf("transaction function missing: %s", key)
		}
		return fingerprint, nil
	}
}

func transactionFunctionFingerprints(path string, raw []byte) (map[string]string, error) {
	files := token.NewFileSet()
	file, err := parser.ParseFile(files, path, raw, 0)
	if err != nil {
		return nil, err
	}
	result := map[string]string{}
	// The complete source body is an injective fingerprint: unlike a hash,
	// equality cannot accept a collision. File paths never occur in this slice.
	add := func(name string, node ast.Node) {
		result[name] = string(raw[files.Position(node.Pos()).Offset:files.Position(node.End()).Offset])
	}

	for _, declaration := range file.Decls {
		switch declaration := declaration.(type) {
		case *ast.FuncDecl:
			if declaration.Body == nil {
				continue
			}
			name := declaration.Name.Name
			if declaration.Recv != nil {
				name = transactionExpression(declaration.Recv.List[0].Type) + "." + name
			}
			add(name, declaration.Body)
		case *ast.GenDecl:
			if declaration.Tok != token.VAR {
				continue
			}
			for _, spec := range declaration.Specs {
				value := spec.(*ast.ValueSpec)
				for i, expr := range value.Values {
					add("var:"+value.Names[min(i, len(value.Names)-1)].Name, expr)
				}
			}
		}
	}
	return result, nil
}

func transactionExpression(node ast.Expr) string {
	switch node := node.(type) {
	case *ast.Ident:
		return node.Name
	case *ast.SelectorExpr:
		return transactionExpression(node.X) + "." + node.Sel.Name
	case *ast.StarExpr:
		return "*" + transactionExpression(node.X)
	case *ast.CallExpr:
		return transactionExpression(node.Fun) + "()"
	case *ast.IndexExpr:
		return transactionExpression(node.X) + "[]"
	case *ast.IndexListExpr:
		return transactionExpression(node.X) + "[]"
	case *ast.CompositeLit:
		return transactionExpression(node.Type) + "{}"
	case *ast.UnaryExpr:
		return node.Op.String() + transactionExpression(node.X)
	case *ast.ParenExpr:
		return "(" + transactionExpression(node.X) + ")"
	default:
		panic(fmt.Sprintf("unsupported transaction target %T", node))
	}
}
