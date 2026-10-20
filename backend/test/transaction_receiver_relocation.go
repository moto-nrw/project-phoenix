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

type transactionReceiverKey struct {
	pkg, receiver, method, scope string
}

func parseTransactionReceiverKey(key string) (transactionReceiverKey, bool) {
	parts := strings.Split(key, ":")
	if len(parts) != 3 || filepath.Ext(parts[0]) != ".go" {
		return transactionReceiverKey{}, false
	}
	receiver, method, ok := strings.Cut(parts[1], ".")
	if !ok {
		return transactionReceiverKey{}, false
	}
	// Receiver chains change during the collapse; the scope operation and
	// occurrence number must not. The site scanner still verifies every key.
	scope := parts[2][strings.LastIndex(parts[2], ".")+1:]
	return transactionReceiverKey{filepath.ToSlash(filepath.Dir(parts[0])), receiver, method, scope}, true
}

func originalEngineTransactionKey(key string, current, previous map[string]string, removed func(string, string) (bool, error)) (string, error) {
	target, ok := parseTransactionReceiverKey(key)
	if !ok || target.receiver != "*Service" || removed == nil {
		return "", nil
	}
	oldKey := ""
	for candidate := range previous {
		source, valid := parseTransactionReceiverKey(candidate)
		if valid && source.pkg == target.pkg && source.method == target.method && source.scope == target.scope && (source.receiver == "Engine" || source.receiver == "*Engine") {
			if oldKey != "" {
				return "", fmt.Errorf("ambiguous Engine transaction source: %s", key)
			}
			oldKey = candidate
		}
	}
	if oldKey == "" {
		return "", nil
	}
	// Compare the whole method, not just this entry: a partial move must not
	// silently drop a scope, duplicate an allowance or change its reason.
	before, uniqueBefore := transactionReceiverScopes(previous, target, true)
	after, uniqueAfter := transactionReceiverScopes(current, target, false)
	if !uniqueBefore || !uniqueAfter || len(before) != len(after) {
		return "", fmt.Errorf("receiver transaction scopes changed: %s", key)
	}
	for scope, reason := range before {
		if after[scope] != reason {
			return "", fmt.Errorf("receiver transaction scopes changed: %s", key)
		}
	}
	absent, err := removed(target.pkg, target.method)
	if err != nil {
		return "", err
	}
	if !absent {
		return "", fmt.Errorf("engine transaction method still declared: %s", key)
	}
	return oldKey, nil
}

func transactionReceiverScopes(entries map[string]string, target transactionReceiverKey, engine bool) (map[string]string, bool) {
	result := map[string]string{}
	for key, reason := range entries {
		parsed, ok := parseTransactionReceiverKey(key)
		if !ok || parsed.pkg != target.pkg || parsed.method != target.method {
			continue
		}
		if engine {
			if parsed.receiver != "Engine" && parsed.receiver != "*Engine" {
				continue
			}
		} else if parsed.receiver != "*Service" {
			continue
		}
		if _, duplicate := result[parsed.scope]; duplicate {
			return nil, false
		}
		result[parsed.scope] = reason
	}
	return result, true
}

func removedTransactionReceiverMethods(root string) func(string, string) (bool, error) {
	cache := map[string]map[string]bool{}
	return func(pkg, method string) (bool, error) {
		methods, loaded := cache[pkg]
		if !loaded {
			entries, err := os.ReadDir(filepath.Join(root, pkg))
			if err != nil {
				return false, err
			}
			methods = map[string]bool{}
			for _, entry := range entries {
				if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
					continue
				}
				path := filepath.Join(root, pkg, entry.Name())
				raw, err := os.ReadFile(path)
				if err != nil {
					return false, err
				}
				file, err := parser.ParseFile(token.NewFileSet(), path, raw, 0)
				if err != nil {
					return false, err
				}
				for _, declaration := range file.Decls {
					method, ok := declaration.(*ast.FuncDecl)
					if ok && method.Recv != nil {
						methods[transactionExpression(method.Recv.List[0].Type)+"."+method.Name.Name] = true
					}
				}
			}
			cache[pkg] = methods
		}
		return !methods["Engine."+method] && !methods["*Engine."+method], nil
	}
}
