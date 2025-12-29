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
		Where("id = ?", id).
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
		Where("id = ?", id).
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
		Join("JOIN auth.account_roles ar ON ar.account_id = account.id").
		Join("JOIN auth.roles r ON ar.role_id = r.id").
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
			switch field {
			case "email":
				// Case-insensitive email search
				if strValue, ok := value.(string); ok {
					query = query.Where("LOWER(email) = LOWER(?)", strValue)
				} else {
					query = query.Where("email = ?", value)
				}
			case "username":
				// Case-insensitive username search
				if strValue, ok := value.(string); ok {
					query = query.Where("LOWER(username) = LOWER(?)", strValue)
				} else {
					query = query.Where("username = ?", value)
				}
			case "email_like":
				// Case-insensitive email pattern search
				if strValue, ok := value.(string); ok {
					query = query.Where("LOWER(email) LIKE LOWER(?)", "%"+strValue+"%")
				}
			case "username_like":
				// Case-insensitive username pattern search
				if strValue, ok := value.(string); ok {
					query = query.Where("LOWER(username) LIKE LOWER(?)", "%"+strValue+"%")
				}
			case "active":
				query = query.Where("active = ?", value)
			case "role":
				// Role-based filtering
				if strValue, ok := value.(string); ok {
					query = query.
						Join("JOIN auth.account_roles ar ON ar.account_id = account.id").
						Join("JOIN auth.roles r ON ar.role_id = r.id").
						Where("LOWER(r.name) = LOWER(?)", strValue)
				}
			default:
				// Default to exact match for other fields
				query = query.Where("? = ?", bun.Ident(field), value)
			}
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

// FindAccountsWithRolesAndPermissions retrieves accounts with their associated roles and permissions
func (r *AccountRepository) FindAccountsWithRolesAndPermissions(ctx context.Context, filters map[string]interface{}) ([]*auth.Account, error) {
	var accounts []*auth.Account
	err := r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// First get the accounts based on filters
		query := tx.NewSelect().Model(&accounts)
		for field, value := range filters {
			if value != nil {
				query = query.Where("? = ?", bun.Ident(field), value)
			}
		}

		if err := query.Scan(ctx); err != nil {
			return err
		}

		// For each account, load roles and permissions
		for _, account := range accounts {
			// Load roles
			var roles []*auth.Role
			err := tx.NewSelect().
				Model(&roles).
				Join("JOIN auth.account_roles ar ON ar.role_id = role.id").
				Where("ar.account_id = ?", account.ID).
				Scan(ctx)

			if err != nil {
				return err
			}
			account.Roles = roles

			// Load direct permissions
			var permissions []*auth.Permission
			err = tx.NewSelect().
				Model(&permissions).
				Join("JOIN auth.account_permissions ap ON ap.permission_id = permission.id").
				Where("ap.account_id = ? AND ap.granted = true", account.ID).
				Scan(ctx)

			if err != nil {
				return err
			}

			// Load role-based permissions
			var rolePermissions []*auth.Permission
			err = tx.NewSelect().
				Model(&rolePermissions).
				Join("JOIN auth.role_permissions rp ON rp.permission_id = permission.id").
				Join("JOIN auth.account_roles ar ON ar.role_id = rp.role_id").
				Where("ar.account_id = ?", account.ID).
				Scan(ctx)

			if err != nil {
				return err
			}

			// Combine direct and role-based permissions (avoid duplicates)
			permMap := make(map[int64]*auth.Permission)
			for _, p := range permissions {
				permMap[p.ID] = p
			}
			for _, p := range rolePermissions {
				if _, exists := permMap[p.ID]; !exists {
					permMap[p.ID] = p
				}
			}

			// Convert map back to slice
			allPermissions := make([]*auth.Permission, 0, len(permMap))
			for _, p := range permMap {
				allPermissions = append(allPermissions, p)
			}
			account.Permissions = allPermissions
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
		Where("id = ?", account.ID).
		ModelTableExpr(accountTable)

	// Extract transaction from context if it exists
	if tx, ok := ctx.Value("tx").(*bun.Tx); ok && tx != nil {
		// Use the transaction if available
		query = tx.NewUpdate().
			Model(account).
			Where("id = ?", account.ID).
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
