package architecture

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/packages"
)

func (a semanticAnalyzer) rawSQLViolations(pkg *packages.Package, classification Package, functionName string, receiver types.Type, selection *types.Selection, method string, call *ast.CallExpr) []Violation {
	args := call.Args
	if isSQLStmtCall(receiver, selection) {
		if query, ok := preparedStatementSQL(pkg, call); ok {
			return a.sqlStringViolations(pkg.PkgPath, classification.Owner, functionName, method, query, true)
		}
		return []Violation{unresolvedTableViolation(pkg.PkgPath, functionName, method)}
	}
	queryIndex := 0
	if strings.HasSuffix(method, "Context") {
		queryIndex = 1
	}
	if len(args) <= queryIndex {
		return []Violation{unresolvedTableViolation(pkg.PkgPath, functionName, method)}
	}
	query, ok := constantString(pkg.TypesInfo, args[queryIndex])
	if !ok {
		// ADR 0092: this one read is built from the embedded lifecycle
		// catalog. Its NOLOGIN reader and read-only transaction reject writes;
		// every catalog table is covered by the two-school behavior fixture.
		if len(a.projections[pkg.PkgPath]) > 0 && tenantLifecycleDynamicRead(pkg.PkgPath, classification, functionName, method) {
			return nil
		}
		return []Violation{unresolvedTableViolation(pkg.PkgPath, functionName, method)}
	}
	return a.sqlStringViolations(pkg.PkgPath, classification.Owner, functionName, method, query, true)
}

func tenantLifecycleDynamicRead(source string, classification Package, functionName, method string) bool {
	return source == "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/adapters/tenantexport" &&
		classification.Owner == "organization-tenancy" && classification.Role == "postgres" &&
		functionName == "exportRun.exportTable" && method == "QueryContext"
}

// ADR 0092 explicitly keeps the administrative lifecycle with the existing
// domain owner, without creating a projection owner. This exact read-only
// grant is not a general exemption for domain persistence packages.
func (p *Policy) isTenantLifecycleRead(projection ReadProjection, pkg Package) bool {
	return p.ModulePath == "github.com/moto-nrw/project-phoenix" &&
		projection.ID == "organization-tenancy.tenant-data-lifecycle" &&
		projection.Package == "modules/organizationtenancy/internal/adapters/tenantexport" &&
		pkg.Owner == "organization-tenancy" && pkg.Role == "postgres"
}

func (p *Policy) readProjectionOwnerAllowed(projection ReadProjection, pkg Package, owners map[string]Owner) bool {
	return owners[pkg.Owner].Kind == "projection" || p.isTenantLifecycleRead(projection, pkg)
}
