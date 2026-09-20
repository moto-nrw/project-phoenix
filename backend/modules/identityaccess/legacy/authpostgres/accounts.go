package authpostgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/authmodels"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const (
	accountTable      = "auth.accounts"
	accountTableAlias = `auth.accounts AS "account"`
	whereID           = "id = ?"
)

type manageableSchoolIDsKey struct{}

// WithManageableSchoolIDs attaches the active school set resolved by the
// Organization & Tenancy capability for an organization-scoped operation.
func WithManageableSchoolIDs(ctx context.Context, ids []int64) context.Context {
	return context.WithValue(ctx, manageableSchoolIDsKey{}, append([]int64(nil), ids...))
}

func manageableSchoolIDs(ctx context.Context) []int64 {
	ids, _ := ctx.Value(manageableSchoolIDsKey{}).([]int64)
	return ids
}

// OrganizationScope reports the organization selected by the caller context.
func OrganizationScope(ctx context.Context) (int64, bool) {
	id := tenant.OrgFromContext(ctx)
	return id, tenant.ScopeFromContext(ctx) == tenant.ScopeOrg && id > 0
}

// membershipScopeKind names the account-visibility predicate a caller's
// context selects. The predicates are compile-time SQL so the architecture
// evaluator resolves the tables they read.
type membershipScopeKind int

const (
	// membershipScopeGlobal keeps the global lookup (platform and admin
	// contexts).
	membershipScopeGlobal membershipScopeKind = iota
	// membershipScopeDenied matches no account: an organization or tenant
	// context without a resolvable scope.
	membershipScopeDenied
	// membershipScopeOrganization restricts to active memberships in the
	// organization's manageable schools.
	membershipScopeOrganization
	// membershipScopeTenant restricts to an active membership in the tenant
	// from the caller's context.
	membershipScopeTenant
)

const (
	membershipScopeDeniedSQL       = "FALSE"
	membershipScopeOrganizationSQL = `EXISTS (
			SELECT 1
			FROM auth.account_tenants AS "account_tenant"
			WHERE "account_tenant".account_id = "account".id
			  AND "account_tenant".status = ?
			  AND "account_tenant".tenant_id IN (?)
		)`
	membershipScopeTenantSQL = `EXISTS (
		SELECT 1
		FROM auth.account_tenants AS "account_tenant"
		WHERE "account_tenant".account_id = "account".id
		  AND "account_tenant".tenant_id = ?
		  AND "account_tenant".status = ?
	)`
)

func accountMembershipScope(ctx context.Context) (membershipScopeKind, []any) {
	if tenant.ScopeFromContext(ctx) == tenant.ScopeOrg {
		schoolIDs := manageableSchoolIDs(ctx)
		if tenant.OrgFromContext(ctx) == 0 || len(schoolIDs) == 0 {
			return membershipScopeDenied, nil
		}
		return membershipScopeOrganization, []any{authmodels.AccountTenantStatusActive, bun.List(schoolIDs)}
	}
	if tenant.IsAdminTx(ctx) || tenant.ScopeFromContext(ctx) == tenant.ScopePlatform {
		return membershipScopeGlobal, nil
	}

	tenantID := tenant.FromContext(ctx)
	if tenantID == 0 {
		return membershipScopeDenied, nil
	}
	return membershipScopeTenant, []any{tenantID, authmodels.AccountTenantStatusActive}
}

// scopeToMembership applies the caller's membership scope to an account read
// or write; the global scope leaves the query untouched. It is generic over
// the query kind for the same reason base.WithTenantFilter is: selects and
// updates must apply the identical predicate, and one definition is what
// keeps them from drifting apart.
func scopeToMembership[Q interface{ Where(string, ...any) Q }](ctx context.Context, query Q) Q {
	switch kind, args := accountMembershipScope(ctx); kind {
	case membershipScopeDenied:
		return query.Where(membershipScopeDeniedSQL)
	case membershipScopeOrganization:
		return query.Where(membershipScopeOrganizationSQL, args...)
	case membershipScopeTenant:
		return query.Where(membershipScopeTenantSQL, args...)
	default:
		return query
	}
}

// effectiveAdminExistsSQL decides whether an account holds effective admin
// scope within one tenant: the literal admin role, or an admin:* / *:*
// permission granted either through a tenant role or directly to the account.
//
// The four placeholders take the caller's qualified account and tenant columns
// (account, tenant, account, tenant) as bun.Safe identifiers written in this
// repository, never request input.
//
// There must stay exactly one definition of "effective admin" in SQL. It
// decides who receives admin-scoped data, so a second, drifting copy is a
// disclosure bug waiting to happen. The Go-side counterpart is
// authorize.HasEffectiveAdminScope.
const effectiveAdminExistsSQL = `EXISTS (
		SELECT 1
		FROM auth.account_roles AS "ar"
		INNER JOIN auth.roles AS "r" ON "r".id = "ar".role_id
		LEFT JOIN auth.role_permissions AS "rp" ON "rp".role_id = "ar".role_id
		LEFT JOIN auth.permissions AS "p" ON "p".id = "rp".permission_id
		WHERE "ar".account_id = ?
		  AND "ar".tenant_id = ?
		  AND (
		    LOWER("r".name) = 'admin'
		    OR ("p".resource = 'admin' AND "p".action = '*')
		    OR ("p".resource = '*' AND "p".action = '*')
		  )
	) OR EXISTS (
		SELECT 1
		FROM auth.account_permissions AS "ap"
		INNER JOIN auth.permissions AS "p" ON "p".id = "ap".permission_id
		WHERE "ap".account_id = ?
		  AND "ap".tenant_id = ?
		  AND "ap".granted = TRUE
		  AND (
		    ("p".resource = 'admin' AND "p".action = '*')
		    OR ("p".resource = '*' AND "p".action = '*')
		  )
	)`

// AccountRepository implements auth.AccountRepository interface
type AccountRepository struct {
	*base.Repository[*authmodels.Account]
	db *bun.DB
}

// FindByIDForUpdate retrieves an account with a row lock. Refresh uses this
// before locking token rows so login and refresh share one lock order.
func (r *AccountRepository) FindByIDForUpdate(ctx context.Context, id int64) (*authmodels.Account, error) {
	account := new(authmodels.Account)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(account).
		ModelTableExpr(accountTableAlias).
		Where(`"account".id = ?`, id).
		For("UPDATE").
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, &modelBase.DatabaseError{Op: "find account by id for update", Err: base.TranslateNotFound(err)}
	}
	return account, nil
}

// ListEffectiveAdminAccountIDs returns the IDs of accounts holding effective
// admin scope in the current tenant, restricted to accounts that are active and
// whose tenant mapping is active.
//
// Callers that need to decide, for many people at once, whether someone sees
// tenant-wide data use this instead of asking per account. The predicate is
// the one effectiveAdminExistsSQL definition, so every reader agrees by
// construction.
func (r *AccountRepository) ListEffectiveAdminAccountIDs(ctx context.Context) ([]int64, error) {
	var ids []int64

	accountColumn := bun.Safe(`"account".id`)
	tenantColumn := bun.Safe(`"account_tenant".tenant_id`)
	query := base.GetDB(ctx, r.db).NewSelect().
		Distinct().
		TableExpr(accountTableAlias).
		ColumnExpr(`"account".id`).
		Join(`INNER JOIN auth.account_tenants AS "account_tenant" ON "account_tenant".account_id = "account".id`).
		Where(`"account".active = ?`, true).
		Where(`"account_tenant".status = ?`, authmodels.AccountTenantStatusActive).
		Where(effectiveAdminExistsSQL, accountColumn, tenantColumn, accountColumn, tenantColumn)

	// auth.accounts is cross-tenant, so the tenant predicate belongs on the
	// mapping table rather than on the account itself.
	query = base.WithTenantFilter(ctx, query, "account_tenant")

	if err := query.Scan(ctx, &ids); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list effective admin account IDs", Err: base.TranslateNotFound(err)}
	}

	return ids, nil
}

// ActiveAccountIDs is the owner query "every active platform account". Other
// owners that must not read auth.accounts themselves join it as a subquery
// (guardian portal reachability, staff messaging) so their statements stay
// single round trips.
func (r *AccountRepository) ActiveAccountIDs(ctx context.Context) *bun.SelectQuery {
	return base.GetDB(ctx, r.db).NewSelect().
		TableExpr(accountTableAlias).
		ColumnExpr(`"account".id`).
		Where(`"account".active = ?`, true)
}

// NewAccountRepository creates a new AccountRepository
func NewAccountRepository(db *bun.DB) authmodels.AccountRepository {
	return &AccountRepository{
		Repository: base.NewRepository[*authmodels.Account](db, accountTable, "Account"),
		db:         db,
	}
}

// FindManageableByID restricts account administration to active memberships
// in the tenant or organization from the caller's context. Platform and admin
// contexts keep the global lookup used by operator flows.
func (r *AccountRepository) FindManageableByID(ctx context.Context, id int64) (*authmodels.Account, error) {
	if kind, _ := accountMembershipScope(ctx); kind == membershipScopeGlobal {
		return r.FindByID(ctx, id)
	}

	account := new(authmodels.Account)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(account).
		ModelTableExpr(accountTableAlias).
		Where(`"account".id = ?`, id)
	err := scopeToMembership(ctx, query).Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find by id", Err: base.TranslateNotFound(err)}
	}
	return account, nil
}

// FindByEmail retrieves an account by email address
func (r *AccountRepository) FindByEmail(ctx context.Context, email string) (*authmodels.Account, error) {
	account := new(authmodels.Account)

	// Explicitly specify the schema and table
	err := base.GetDB(ctx, r.db).NewSelect().
		ModelTableExpr(accountTable).
		Where("LOWER(email) = LOWER(?)", email).
		Scan(ctx, account)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by email",
			Err: base.TranslateNotFound(err),
		}
	}

	return account, nil
}

// List retrieves accounts matching the provided filters without applying an
// account-management boundary. Internal authentication flows remain global.
func (r *AccountRepository) List(ctx context.Context, filters map[string]interface{}) ([]*authmodels.Account, error) {
	return r.list(ctx, filters)
}

func (r *AccountRepository) list(ctx context.Context, filters map[string]interface{}) ([]*authmodels.Account, error) {
	var accounts []*authmodels.Account
	query := base.GetDB(ctx, r.db).NewSelect().Model(&accounts).ModelTableExpr(accountTableAlias)

	// Apply filters
	for field, value := range filters {
		if value != nil {
			query = r.applyAccountFilter(ctx, query, field, value)
		}
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list",
			Err: base.TranslateNotFound(err),
		}
	}

	return accounts, nil
}

// applyAccountFilter applies a single filter to the query
func (r *AccountRepository) applyAccountFilter(ctx context.Context, query *bun.SelectQuery, field string, value interface{}) *bun.SelectQuery {
	switch field {
	case "email":
		return r.applyStringEqualFilter(query, bun.Safe("email"), value)
	case "username":
		return r.applyStringEqualFilter(query, bun.Safe("username"), value)
	case "email_like":
		return r.applyStringLikeFilter(query, bun.Safe("email"), value)
	case "username_like":
		return r.applyStringLikeFilter(query, bun.Safe("username"), value)
	case "active":
		return query.Where("active = ?", value)
	default:
		return query.Where("? = ?", bun.Ident(field), value)
	}
}

// applyStringEqualFilter applies case-insensitive equality filter for string fields
// The field is a column written in this file, never request input.
func (r *AccountRepository) applyStringEqualFilter(query *bun.SelectQuery, field bun.Safe, value interface{}) *bun.SelectQuery {
	if strValue, ok := value.(string); ok {
		return query.Where("LOWER(?) = LOWER(?)", field, strValue)
	}
	return query.Where("? = ?", field, value)
}

// applyStringLikeFilter applies case-insensitive LIKE filter for string fields
func (r *AccountRepository) applyStringLikeFilter(query *bun.SelectQuery, field bun.Safe, value interface{}) *bun.SelectQuery {
	if strValue, ok := value.(string); ok {
		return query.Where("LOWER(?) LIKE LOWER(?)", field, "%"+strValue+"%")
	}
	return query
}

// Update overrides the base Update method to handle email normalization.
func (r *AccountRepository) Update(ctx context.Context, account *authmodels.Account) error {
	return r.update(ctx, account)
}

func (r *AccountRepository) update(ctx context.Context, account *authmodels.Account) error {
	if account == nil {
		return fmt.Errorf("account cannot be nil")
	}

	// Validate account - this will also normalize the email
	if err := account.Validate(); err != nil {
		return err
	}

	// Execute the query using GetDB for transaction support
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model(account).
		ModelTableExpr(accountTableAlias).
		Where(`"account".id = ?`, account.ID)

	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update",
			Err: base.TranslateNotFound(err),
		}
	}
	return base.AssertRowsAffected(result, 1, "update account")
}

// AnonymizeForDeletion overwrites the account's email with the given
// anonymized placeholder and clears the username. Custom method
// (backend-conventions Rule 2): GDPR person-deletion step that pairs two
// column writes into one statement; used by operator SoftDeletePerson.
func (r *AccountRepository) AnonymizeForDeletion(ctx context.Context, accountID int64, anonymizedEmail string) error {
	_, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*authmodels.Account)(nil)).
		ModelTableExpr(accountTableAlias).
		Set(`email = ?`, anonymizedEmail).
		Set(`username = NULL`).
		Where(`"account".id = ?`, accountID).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "anonymize account for deletion",
			Err: base.TranslateNotFound(err),
		}
	}
	return nil
}
