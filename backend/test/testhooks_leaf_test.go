package test

import (
	"bufio"
	"go/build/constraint"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// testhooksFreePackages are imported by most of the backend. A testhooks file
// in one of them gives every dependent package a second compiled form, so
// tagged test builds and untagged dev builds stop sharing Go build cache
// entries (#5985). Their test hooks live in leaf packages instead:
// tenant/tenanttest, modules/peopledirectory/parentfixture, and the already
// tagged Communication store. Remove an entry only together with the reason
// it no longer fans out.
var testhooksFreePackages = []string{
	"tenant",
	"modules/peopledirectory",
	"modules/communication",
	"modules/securityruntime/internal/application/authorize",
}

func TestBroadlyImportedPackagesCompileTheSameWithTestHooks(t *testing.T) {
	t.Parallel()
	root, err := findBackendRoot()
	require.NoError(t, err)
	for _, pkg := range testhooksFreePackages {
		entries, err := os.ReadDir(filepath.Join(root, pkg))
		require.NoError(t, err, pkg)
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			path := filepath.Join(pkg, name)
			expr := goBuildConstraint(t, filepath.Join(root, path))
			if expr == nil {
				continue
			}
			if mentionsTag(expr, "testhooks") {
				t.Errorf("%s depends on the testhooks tag; move its hooks into a leaf package (#5985)", path)
			}
		}
	}
}

// mentionsTag reports whether tag occurs anywhere in expr, so combinations such
// as `testhooks && linux` count as tag-dependent too.
func mentionsTag(expr constraint.Expr, tag string) bool {
	switch e := expr.(type) {
	case *constraint.TagExpr:
		return e.Tag == tag
	case *constraint.NotExpr:
		return mentionsTag(e.X, tag)
	case *constraint.AndExpr:
		return mentionsTag(e.X, tag) || mentionsTag(e.Y, tag)
	case *constraint.OrExpr:
		return mentionsTag(e.X, tag) || mentionsTag(e.Y, tag)
	}
	return false
}

// goBuildConstraint returns the //go:build expression of a Go file, or nil.
func goBuildConstraint(t *testing.T, path string) constraint.Expr {
	t.Helper()
	file, err := os.Open(path)
	require.NoError(t, err)
	defer func() { require.NoError(t, file.Close()) }()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "package ") {
			break
		}
		if constraint.IsGoBuild(line) {
			expr, err := constraint.Parse(line)
			require.NoError(t, err, path)
			return expr
		}
	}
	require.NoError(t, scanner.Err(), path)
	return nil
}
