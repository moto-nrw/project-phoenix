package application

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// renewalOutcomes is every error a refresh or a session validation can
// return, with what it means for the session. Refresh and validation answer
// through identityaccess.SessionOutcomeOf, so one table decides both routes.
var renewalOutcomes = map[string]struct {
	err     error
	outcome identityaccess.SessionOutcome
}{
	"identityaccess.ErrInvalidToken":                 {identityaccess.ErrInvalidToken, identityaccess.SessionEnded},
	"identityaccess.ErrTokenExpired":                 {identityaccess.ErrTokenExpired, identityaccess.SessionEnded},
	"identityaccess.ErrTokenNotFound":                {identityaccess.ErrTokenNotFound, identityaccess.SessionEnded},
	"identityaccess.ErrAccountInactive":              {identityaccess.ErrAccountInactive, identityaccess.SessionEnded},
	"domain.ErrAccountNotFound":                      {domain.ErrAccountNotFound, identityaccess.SessionEnded},
	"identityaccess.ErrTenantNotFound":               {identityaccess.ErrTenantNotFound, identityaccess.SessionEnded},
	"identityaccess.ErrTenantAccessDenied":           {identityaccess.ErrTenantAccessDenied, identityaccess.SessionEnded},
	"identityaccess.ErrParentMustUseParentPortal":    {identityaccess.ErrParentMustUseParentPortal, identityaccess.SessionEnded},
	"identityaccess.ErrMustUseSchoolPortal":          {identityaccess.ErrMustUseSchoolPortal, identityaccess.SessionEnded},
	"identityaccess.ErrAccountNoSchoolPortalRole":    {identityaccess.ErrAccountNoSchoolPortalRole, identityaccess.SessionEnded},
	"identityaccess.ErrAccountNoGuardianRole":        {identityaccess.ErrAccountNoGuardianRole, identityaccess.SessionEnded},
	"identityaccess.ErrNoTraegerMembership":          {identityaccess.ErrNoTraegerMembership, identityaccess.SessionEnded},
	"identityaccess.ErrTraegerSchoolSwitchForbidden": {identityaccess.ErrTraegerSchoolSwitchForbidden, identityaccess.SessionEnded},
	"identityaccess.ErrTraegerSchoolNotFound":        {identityaccess.ErrTraegerSchoolNotFound, identityaccess.SessionEnded},
	"domain.ErrAccountNotCaterer":                    {domain.ErrAccountNotCaterer, identityaccess.SessionEnded},
	"identityaccess.ErrCatererWithoutSchool":         {identityaccess.ErrCatererWithoutSchool, identityaccess.SessionEnded},
	"identityaccess.ErrMFAStatusUnavailable":         {identityaccess.ErrMFAStatusUnavailable, identityaccess.SessionUndecided},
}

// TestEveryRenewalErrorHasItsOutcome walks the session policy, validation
// and refresh code and the functions of this package they reach, and fails
// on an error sentinel whose outcome renewalOutcomes does not declare: a new
// guard refusal must say whether it ends the session instead of falling to
// a retry answer unnoticed.
func TestEveryRenewalErrorHasItsOutcome(t *testing.T) {
	t.Parallel()
	reached := reachedSentinels(t, "session_policy.go", "session_validation.go", "session_refresh.go")
	var unclassified []string
	for _, name := range reached {
		expected, known := renewalOutcomes[name]
		if !known {
			if _, retry := renewalRetries[name]; !retry {
				unclassified = append(unclassified, name)
			}
			continue
		}
		got := identityaccess.SessionOutcomeOf(authenticationError(failed("renew session", expected.err)))
		assert.Equal(t, expected.outcome, got, name)
	}
	assert.Empty(t, unclassified, "renewal errors without an outcome: add them to renewalOutcomes and identityaccess.SessionOutcomeOf, or to renewalRetries")
	for name := range renewalOutcomes {
		assert.Contains(t, reached, name, "stale renewalOutcomes entry")
	}
	for name := range renewalRetries {
		assert.Contains(t, reached, name, "stale renewalRetries entry")
	}
	notBooked := &identityaccess.PortalNotBookedError{Module: "eltern_app"}
	assert.Equal(t, identityaccess.SessionModuleNotBooked, identityaccess.SessionOutcomeOf(authenticationError(failed("renew session", notBooked))))
}

// renewalRetries are reachable errors that are no refusal of the session: a
// read or a dependency failed, and the caller retries.
var renewalRetries = map[string]string{
	"errTraegerMembershipsUnavailable":          "the module was composed without the Träger memberships",
	"errAccountAuthenticationUnavailable":       "the module was composed without account authentication",
	"errPortalBookingChanged":                   "the booking changed between its two reads",
	"errMissingClaimsPayload":                   "a guard returned no claims",
	"errSchoolMFARequiredAtMint":                "a school sign-in's MFA gate went stale; a renewal never re-decides the gate",
	"identityaccess.ErrOperatorSessionNotFound": "the operator family read; the operator path maps a miss to ErrInvalidToken",
	"identityaccess.ErrOperatorNotFound":        "the operator read; the operator path maps a miss to ErrInvalidToken",
	"identityaccess.ErrAccountSessionNotFound":  "the family and row reads; both paths map a miss to ErrInvalidToken",
}

// funcKey names a free function by its name and a method of
// AccountAuthentication as Type.name; the flows call their steps on s, so a
// selector follows only that type's methods.
func funcKey(decl *ast.FuncDecl) string {
	if decl.Recv == nil || len(decl.Recv.List) == 0 {
		return decl.Name.Name
	}
	receiver := decl.Recv.List[0].Type
	if star, ok := receiver.(*ast.StarExpr); ok {
		receiver = star.X
	}
	if ident, ok := receiver.(*ast.Ident); ok {
		return ident.Name + "." + decl.Name.Name
	}
	return decl.Name.Name
}

// errorTranslations map errors to the public contract; they name every
// sentinel of the module and decide no session.
var errorTranslations = map[string]bool{"authenticationError": true, "MapError": true}

// reachedSentinels returns the error sentinels (identityaccess.Err*,
// domain.Err*, package err* variables) the functions of the given files and
// every package function they name use, sorted.
func reachedSentinels(t *testing.T, roots ...string) []string {
	t.Helper()
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	fset := token.NewFileSet()
	funcs := map[string][]*ast.FuncDecl{}
	errVars := map[string]bool{}
	var rootDecls []ast.Node
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(fset, file, nil, 0)
		require.NoError(t, err)
		isRoot := false
		for _, root := range roots {
			isRoot = isRoot || root == file
		}
		for _, decl := range parsed.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				funcs[funcKey(d)] = append(funcs[funcKey(d)], d)
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					if value, ok := spec.(*ast.ValueSpec); ok {
						for _, name := range value.Names {
							if strings.HasPrefix(name.Name, "err") {
								errVars[name.Name] = true
							}
						}
					}
				}
			}
			if isRoot {
				rootDecls = append(rootDecls, decl)
			}
		}
	}
	seen := map[string]bool{}
	sentinels := map[string]bool{}
	queue := rootDecls
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		ast.Inspect(node, func(n ast.Node) bool {
			var name string
			switch v := n.(type) {
			case *ast.SelectorExpr:
				if pkg, ok := v.X.(*ast.Ident); ok && (pkg.Name == "identityaccess" || pkg.Name == "domain") && strings.HasPrefix(v.Sel.Name, "Err") {
					sentinels[pkg.Name+"."+v.Sel.Name] = true
					return false
				}
				name = "AccountAuthentication." + v.Sel.Name
			case *ast.Ident:
				if errVars[v.Name] {
					sentinels[v.Name] = true
				}
				name = v.Name
			default:
				return true
			}
			if decls, ok := funcs[name]; ok && !seen[name] && !errorTranslations[name] {
				seen[name] = true
				for _, decl := range decls {
					queue = append(queue, decl)
				}
			}
			return true
		})
	}
	names := make([]string, 0, len(sentinels))
	for name := range sentinels {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
