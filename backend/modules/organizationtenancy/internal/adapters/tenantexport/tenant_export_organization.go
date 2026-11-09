package tenantexport

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

// A Träger package (#3838) returns what belongs to the Träger: its own
// tables, the master rows of its schools and the accounts of its Träger
// office, which no school package carries. What a school holds goes in that
// school's package, so the Träger package names those tables without rows.

func (r *exportRun) subjectID() int64 {
	if r.options.OrganizationID > 0 {
		return r.options.OrganizationID
	}
	return r.options.SchoolID
}

func (r *exportRun) requireSubject(ctx context.Context) error {
	query, missing := `SELECT EXISTS (SELECT 1 FROM platform.schools WHERE id = ?)`, "tenant export: school unavailable"
	if r.options.OrganizationID > 0 {
		query, missing = `SELECT EXISTS (SELECT 1 FROM platform.organizations WHERE id = ?)`, "tenant export: Träger unavailable"
		r.manifest.Package = "organization"
	}
	var exists bool
	if err := r.tx.NewRaw(query, r.subjectID()).Scan(ctx, &exists); err != nil || !exists {
		return errors.New(missing)
	}
	return nil
}

// inSchoolPackages reports a table a Träger package leaves to its schools.
func (r *exportRun) inSchoolPackages(table organizationtenancy.LifecycleTable) bool {
	if r.options.OrganizationID <= 0 {
		return false
	}
	_, err := r.organizationScope(table, "t", 0)
	return errors.Is(err, errInSchoolPackage)
}

var errInSchoolPackage = errors.New("tenant export: held by a school")

func (r *exportRun) organizationScope(table organizationtenancy.LifecycleTable, alias string, depth int) (string, error) {
	switch table.Scope {
	case "organization":
		return alias + "." + quoteExportIdentifier(table.ScopeColumn) + " = ?0", nil
	case "school":
		// The master rows of the Träger's schools say which school ID the
		// school packages carry.
		return alias + ".organization_id = ?0", nil
	case "tenant", "tenantReference":
		return "", errInSchoolPackage
	case "tenantChild":
		return r.childScope(table, alias, depth)
	case "account":
		return organizationAccountScope(table.Name, alias)
	default:
		return "", exportTableError(table.Name)
	}
}

// The Träger office's accounts, including pending and deactivated ones.
// School accounts are returned with their schools.
const organizationAccounts = `(SELECT account_id FROM platform.organization_accounts WHERE organization_id = ?0)`

func organizationAccountScope(table, alias string) (string, error) {
	switch table {
	case "auth.accounts":
		return alias + ".id IN " + organizationAccounts, nil
	case "auth.mfa_credentials", "auth.passkey_credentials", "auth.password_reset_tokens":
		return alias + ".account_id IN " + organizationAccounts, nil
	case "auth.password_reset_rate_limits":
		return alias + ".email IN (SELECT email FROM auth.accounts WHERE id IN " + organizationAccounts + ")", nil
	case "platform.announcement_views":
		return alias + ".user_id IN " + organizationAccounts, nil
	default:
		return "", exportTableError(table)
	}
}
