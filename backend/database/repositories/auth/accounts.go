package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

const (
	accountTable      = "auth.accounts"
	accountTableAlias = `auth.accounts AS "account"`
)

// AccountRepository implements auth.AccountRepository interface
type AccountRepository struct {
	*base.Repository[*auth.Account]
	db *bun.DB
}

// NewAccountRepository creates a new AccountRepository
func NewAccountRepository(db *bun.DB) auth.AccountRepository {
	return &AccountRepository{
		Repository: base.NewRepository[*auth.Account](db, accountTable, "Account"),
		db:         db,
	}
}

// FindByEmail retrieves an account by email address
func (r *AccountRepository) FindByEmail(ctx context.Context, email string) (*auth.Account, error) {
	account := new(auth.Account)

	// Explicitly specify the schema and table
	err := r.db.NewSelect().
		ModelTableExpr(accountTable).
		Where("LOWER(email) = LOWER(?)", email).
		Scan(ctx, account)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by email",
			Err: err,
		}
	}

	return account, nil
}

// FindByUsername retrieves an account by username
func (r *AccountRepository) FindByUsername(ctx context.Context, username string) (*auth.Account, error) {
	account := new(auth.Account)

	// Explicitly specify the schema and table
	err := r.db.NewSelect().
		ModelTableExpr(accountTable).
		Where("LOWER(username) = LOWER(?)", username).
		Scan(ctx, account)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by username",
			Err: err,
		}
	}

	return account, nil
}

// UpdateLastLogin updates the last login timestamp for an account
func (r *AccountRepository) UpdateLastLogin(ctx context.Context, id int64) error {
	_, err := r.db.NewUpdate().
		Model((*auth.Account)(nil)).
		ModelTableExpr(accountTable).
		Set("last_login = ?", time.Now()).
		Where(whereID, id).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update last login",
			Err: err,
		}
	}

	return nil
}

// UpdatePassword updates the password hash for an account
func (r *AccountRepository) UpdatePassword(ctx context.Context, id int64, passwordHash string) error {
	_, err := r.db.NewUpdate().
		Model((*auth.Account)(nil)).
		ModelTableExpr(accountTable).
		Set("password_hash = ?", passwordHash).
		Set("is_password_otp = ?", false). // Reset OTP flag when setting a permanent password
		Where(whereID, id).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update password",
			Err: err,
		}
	}

	return nil
}

// FindByRole retrieves accounts that have a specific role
func (r *AccountRepository) FindByRole(ctx context.Context, role string) ([]*auth.Account, error) {
	var accounts []*auth.Account

	// Use a SQL JOIN to retrieve accounts with the specified role
	// This assumes your account_roles table exists and has the proper foreign keys
	err := r.db.NewSelect().
		Model(&accounts).
		ModelTableExpr(accountTableAlias).
		Join(`JOIN auth.account_roles ar ON ar.account_id = "account".id`).
		Join(`JOIN auth.roles r ON ar.role_id = r.id`).
		Where("LOWER(r.name) = LOWER(?)", role).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by role",
			Err: err,
		}
	}

	return accounts, nil
}

// List retrieves accounts matching the provided filters
func (r *AccountRepository) List(ctx context.Context, filters map[string]interface{}) ([]*auth.Account, error) {
	var accounts []*auth.Account
	query := r.db.NewSelect().Model(&accounts).ModelTableExpr(accountTableAlias)

	// Apply filters
	for field, value := range filters {
		if value != nil {
			query = r.applyAccountFilter(query, field, value)
		}
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list",
			Err: err,
		}
	}

	return accounts, nil
}

// applyAccountFilter applies a single filter to the query
func (r *AccountRepository) applyAccountFilter(query *bun.SelectQuery, field string, value interface{}) *bun.SelectQuery {
	switch field {
	case "email":
		return r.applyStringEqualFilter(query, "email", value)
	case "username":
		return r.applyStringEqualFilter(query, "username", value)
	case "email_like":
		return r.applyStringLikeFilter(query, "email", value)
	case "username_like":
		return r.applyStringLikeFilter(query, "username", value)
	case "active":
		return query.Where("active = ?", value)
	case "role":
		return r.applyRoleFilter(query, value)
	default:
		return query.Where("? = ?", bun.Ident(field), value)
	}
}

// applyStringEqualFilter applies case-insensitive equality filter for string fields
func (r *AccountRepository) applyStringEqualFilter(query *bun.SelectQuery, field string, value interface{}) *bun.SelectQuery {
	if strValue, ok := value.(string); ok {
		return query.Where("LOWER("+field+") = LOWER(?)", strValue)
	}
	return query.Where(field+" = ?", value)
}

// applyStringLikeFilter applies case-insensitive LIKE filter for string fields
func (r *AccountRepository) applyStringLikeFilter(query *bun.SelectQuery, field string, value interface{}) *bun.SelectQuery {
	if strValue, ok := value.(string); ok {
		return query.Where("LOWER("+field+") LIKE LOWER(?)", "%"+strValue+"%")
	}
	return query
}

// applyRoleFilter applies role-based filtering
func (r *AccountRepository) applyRoleFilter(query *bun.SelectQuery, value interface{}) *bun.SelectQuery {
	if strValue, ok := value.(string); ok {
		return query.
			Join("JOIN auth.account_roles ar ON ar.account_id = account.id").
			Join("JOIN auth.roles r ON ar.role_id = r.id").
			Where("LOWER(r.name) = LOWER(?)", strValue)
	}
	return query
}

// FindAccountsWithRolesAndPermissions retrieves accounts with their associated roles and permissions
func (r *AccountRepository) FindAccountsWithRolesAndPermissions(ctx context.Context, filters map[string]interface{}) ([]*auth.Account, error) {
	var accounts []*auth.Account
	err := r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// First get the accounts based on filters
		if err := r.loadAccountsByFilters(ctx, tx, &accounts, filters); err != nil {
			return err
		}

		// For each account, load roles and permissions
		for _, account := range accounts {
			if err := r.loadAccountRolesAndPermissions(ctx, tx, account); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find accounts with roles and permissions",
			Err: err,
		}
	}

	return accounts, nil
}

// loadAccountsByFilters loads accounts based on provided filters
func (r *AccountRepository) loadAccountsByFilters(ctx context.Context, tx bun.Tx, accounts *[]*auth.Account, filters map[string]interface{}) error {
	query := tx.NewSelect().
		Model(accounts).
		ModelTableExpr(accountTableAlias)
	for field, value := range filters {
		if value != nil {
			query = query.Where("? = ?", bun.Ident(field), value)
		}
	}
	return query.Scan(ctx)
}

// loadAccountRolesAndPermissions loads roles and permissions for a single account
func (r *AccountRepository) loadAccountRolesAndPermissions(ctx context.Context, tx bun.Tx, account *auth.Account) error {
	// Load roles
	if err := r.loadAccountRoles(ctx, tx, account); err != nil {
		return err
	}

	// Load and merge permissions
	return r.loadAccountPermissions(ctx, tx, account)
}

// loadAccountRoles loads roles for an account
func (r *AccountRepository) loadAccountRoles(ctx context.Context, tx bun.Tx, account *auth.Account) error {
	var roles []*auth.Role
	err := tx.NewSelect().
		Model(&roles).
		ModelTableExpr(`auth.roles AS "role"`).
		Join(`JOIN auth.account_roles ar ON ar.role_id = "role".id`).
		Where("ar.account_id = ?", account.ID).
		Scan(ctx)

	if err != nil {
		return err
	}
	account.Roles = roles
	return nil
}

// loadAccountPermissions loads and merges direct and role-based permissions for an account
func (r *AccountRepository) loadAccountPermissions(ctx context.Context, tx bun.Tx, account *auth.Account) error {
	// Load direct permissions
	directPermissions, err := r.loadDirectPermissions(ctx, tx, account.ID)
	if err != nil {
		return err
	}

	// Load role-based permissions
	rolePermissions, err := r.loadRoleBasedPermissions(ctx, tx, account.ID)
	if err != nil {
		return err
	}

	// Merge permissions (avoid duplicates)
	account.Permissions = r.mergePermissions(directPermissions, rolePermissions)
	return nil
}

// loadDirectPermissions loads direct permissions for an account
func (r *AccountRepository) loadDirectPermissions(ctx context.Context, tx bun.Tx, accountID int64) ([]*auth.Permission, error) {
	var permissions []*auth.Permission
	err := tx.NewSelect().
		Model(&permissions).
		ModelTableExpr(`auth.permissions AS "permission"`).
		Join(`JOIN auth.account_permissions ap ON ap.permission_id = "permission".id`).
		Where("ap.account_id = ? AND ap.granted = true", accountID).
		Scan(ctx)
	return permissions, err
}

// loadRoleBasedPermissions loads role-based permissions for an account
func (r *AccountRepository) loadRoleBasedPermissions(ctx context.Context, tx bun.Tx, accountID int64) ([]*auth.Permission, error) {
	var permissions []*auth.Permission
	err := tx.NewSelect().
		Model(&permissions).
		ModelTableExpr(`auth.permissions AS "permission"`).
		Join(`JOIN auth.role_permissions rp ON rp.permission_id = "permission".id`).
		Join("JOIN auth.account_roles ar ON ar.role_id = rp.role_id").
		Where("ar.account_id = ?", accountID).
		Scan(ctx)
	return permissions, err
}

// mergePermissions combines direct and role-based permissions, avoiding duplicates
func (r *AccountRepository) mergePermissions(directPermissions, rolePermissions []*auth.Permission) []*auth.Permission {
	permMap := make(map[int64]*auth.Permission)
	for _, p := range directPermissions {
		permMap[p.ID] = p
	}
	for _, p := range rolePermissions {
		if _, exists := permMap[p.ID]; !exists {
			permMap[p.ID] = p
		}
	}

	allPermissions := make([]*auth.Permission, 0, len(permMap))
	for _, p := range permMap {
		allPermissions = append(allPermissions, p)
	}
	return allPermissions
}

// Create overrides the base Create method for validation
func (r *AccountRepository) Create(ctx context.Context, account *auth.Account) error {
	if account == nil {
		return fmt.Errorf("account cannot be nil")
	}

	// Validate account - this will also normalize the email
	if err := account.Validate(); err != nil {
		return err
	}

	// Use the base Create method which now uses ModelTableExpr
	return r.Repository.Create(ctx, account)
}

// Update overrides the base Update method to handle email normalization
func (r *AccountRepository) Update(ctx context.Context, account *auth.Account) error {
	if account == nil {
		return fmt.Errorf("account cannot be nil")
	}

	// Validate account - this will also normalize the email
	if err := account.Validate(); err != nil {
		return err
	}

	// Get the query builder - detect if we're in a transaction
	query := r.db.NewUpdate().
		Model(account).
		Where(whereID, account.ID).
		ModelTableExpr(accountTable)

	// Extract transaction from context if it exists
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		// Use the transaction if available
		query = tx.NewUpdate().
			Model(account).
			Where(whereID, account.ID).
			ModelTableExpr(accountTable)
	}

	// Execute the query
	_, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update",
			Err: err,
		}
	}

	return nil
}
