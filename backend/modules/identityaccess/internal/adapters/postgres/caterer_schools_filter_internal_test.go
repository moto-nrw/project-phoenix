package postgres

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCatererSchoolsQueriesFilterOnSchoolOrCaterer is the ratchet of the
// caterer's read across schools (ADR 0077 in the shape of ADR 0084): the
// global identity read bypasses the school's RLS, so every statement of this
// package on auth.caterer_schools must carry its own filter. A write or read
// for the school filters on the school in scope; the one read across schools,
// ListCatererSchools, filters on the caterer.
func TestCatererSchoolsQueriesFilterOnSchoolOrCaterer(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	fset := token.NewFileSet()
	var touching []string
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		source, err := os.ReadFile(name)
		require.NoError(t, err)
		file, err := parser.ParseFile(fset, name, source, 0)
		require.NoError(t, err)
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			body := string(source[fset.Position(fn.Body.Pos()).Offset:fset.Position(fn.Body.End()).Offset])
			if !strings.Contains(body, "catererSchoolRow") && !strings.Contains(body, "caterer_schools") {
				continue
			}
			touching = append(touching, fn.Name.Name)
			schoolScoped := strings.Contains(body, "scopeTenantQuery(s.scope(ctx)") && strings.Contains(body, `"caterer_school".tenant_id = ?`)
			insertForSchool := strings.Contains(body, "NewInsert()") && strings.Contains(body, "s.scope(ctx).TenantID")
			catererFiltered := strings.Contains(body, `Where(`+"`"+`"caterer_school".caterer_id = ?`+"`"+`, catererID)`)
			assert.True(t, schoolScoped || insertForSchool || catererFiltered, "%s: a statement on auth.caterer_schools filters on the school in scope or on the caterer", fn.Name.Name)
			if catererFiltered && !schoolScoped {
				assert.Equal(t, "ListCatererSchools", fn.Name.Name, "the caterer's list is the only read across schools")
			}
		}
	}
	slices.Sort(touching)
	assert.Equal(t, []string{"EndCatererSchool", "FindSchoolCaterer", "InsertCatererSchool", "ListCatererSchools"}, touching,
		"a new statement on auth.caterer_schools needs its filter reviewed here")
}
