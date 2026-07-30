package auth

import (
	"context"
	"database/sql"
	"errors"
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

// EffectiveAdminExistsSQL builds the SQL predicate that decides whether an
// account holds effective admin scope within one tenant: the literal admin
// role, or an admin:* / *:* permission granted either through a tenant role or
// directly to the account.
//
// It takes the caller's qualified account and tenant columns so the same
// predicate can be applied to auth.accounts and to any other table carrying an
// (account_id, tenant_id) pair. The arguments are SQL identifiers written by
// callers in this repository, never request input.
//
// There must stay exactly one definition of "effective admin" in SQL. It
// decides who receives admin-scoped data, so a second, drifting copy is a
// disclosure bug waiting to happen. The Go-side counterpart is
// authorize.HasEffectiveAdminScope.
func EffectiveAdminExistsSQL(accountColumn, tenantColumn string) string {
	return fmt.Sprintf(`EXISTS (
		SELECT 1
		FROM auth.account_roles AS "ar"
		INNER JOIN auth.roles AS "r" ON "r".id = "ar".role_id
		LEFT JOIN auth.role_permissions AS "rp" ON "rp".role_id = "ar".role_id
		LEFT JOIN auth.permissions AS "p" ON "p".id = "rp".permission_id
		WHERE "ar".account_id = %[1]s
		  AND "ar".tenant_id = %[2]s
		  AND (
		    LOWER("r".name) = 'admin'
		    OR ("p".resource = 'admin' AND "p".action = '*')
		    OR ("p".resource = '*' AND "p".action = '*')
		  )
	) OR EXISTS (
		SELECT 1
		FROM auth.account_permissions AS "ap"
		INNER JOIN auth.permissions AS "p" ON "p".id = "ap".permission_id
		WHERE "ap".account_id = %[1]s
		  AND "ap".tenant_id = %[2]s
		  AND "ap".granted = TRUE
		  AND (
		    ("p".resource = 'admin' AND "p".action = '*')
		    OR ("p".resource = '*' AND "p".action = '*')
		  )
	)`, accountColumn, tenantColumn)
}

// AccountRepository implements auth.AccountRepository interface
type AccountRepository struct {
	*base.Repository[*auth.Account]
	db *bun.DB
}

// FindByIDForUpdate retrieves an account with a row lock. Refresh uses this
// before locking token rows so login and refresh share one lock order.
func (r *AccountRepository) FindByIDForUpdate(ctx context.Context, id int64) (*auth.Account, error) {
	account := new(auth.Account)
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
		return nil, &modelBase.DatabaseError{Op: "find account by id for update", Err: err}
	}
	return account, nil
}

// ListEffectiveAdminAccountIDs returns the IDs of accounts holding effective
// admin scope in the current tenant, restricted to accounts that are active and
// whose tenant mapping is active.
//
// Callers that need to decide, for many people at once, whether someone sees
// tenant-wide data use this instead of asking per account. The predicate is
// shared with the push-subscription admin finder through
// EffectiveAdminExistsSQL, so both agree by construction.
func (r *AccountRepository) ListEffectiveAdminAccountIDs(ctx context.Context) ([]int64, error) {
	var ids []int64

	query := base.GetDB(ctx, r.db).NewSelect().
		Distinct().
		TableExpr(accountTableAlias).
		ColumnExpr(`"account".id`).
		Join(`INNER JOIN auth.account_tenants AS "account_tenant" ON "account_tenant".account_id = "account".id`).
		Where(`"account".active = ?`, true).
		Where(`"account_tenant".status = ?`, auth.AccountTenantStatusActive).
		Where(EffectiveAdminExistsSQL(`"account".id`, `"account_tenant".tenant_id`))

	// auth.accounts is cross-tenant, so the tenant predicate belongs on the
	// mapping table rather than on the account itself.
	query = base.WithTenantFilter(ctx, query, "account_tenant")

	if err := query.Scan(ctx, &ids); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list effective admin account IDs", Err: err}
	}

	return ids, nil
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
	err := base.GetDB(ctx, r.db).NewSelect().
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

// FindByCalendarFeedToken resolves the account owning an iCalendar subscription
// feed. The stored calendar_feed_token is the SHA-256 HASH of the raw token
// (the service hashes both on write and before this lookup), so callers pass
// the hash, not the raw token — a DB read exposes no replayable URL. Returns
// (nil, nil) when no account matches so the public feed endpoint can answer 404
// without leaking whether a token exists.
func (r *AccountRepository) FindByCalendarFeedToken(ctx context.Context, tokenHash string) (*auth.Account, error) {
	if tokenHash == "" {
		return nil, nil
	}
	account := new(auth.Account)
	err := base.GetDB(ctx, r.db).NewSelect().
		ModelTableExpr(accountTable).
		Where("calendar_feed_token = ?", tokenHash).
		Scan(ctx, account)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{Op: "find by calendar feed token", Err: err}
	}
	return account, nil
}

// EnsureCalendarFeedToken atomically claims newTokenHash for the account only if
// it has no feed token yet, then returns whatever hash is persisted. The value
// is the SHA-256 hash of the raw token (the service hashes before calling), not
// the raw token itself. The conditional UPDATE takes a row lock, so of two
// concurrent first-time callers exactly one wins the write; the loser matches no
// row and reads back the winner's hash via the follow-up SELECT. Nobody is ever
// handed a hash a later write overwrote.
func (r *AccountRepository) EnsureCalendarFeedToken(ctx context.Context, accountID int64, newTokenHash string) (string, error) {
	db := base.GetDB(ctx, r.db)
	res, err := db.NewUpdate().
		Model((*auth.Account)(nil)).
		ModelTableExpr(accountTable).
		Set("calendar_feed_token = ?", newTokenHash).
		Where(whereID, accountID).
		Where("(calendar_feed_token IS NULL OR calendar_feed_token = '')").
		Exec(ctx)
	if err != nil {
		return "", &modelBase.DatabaseError{Op: "ensure calendar feed token", Err: err}
	}
	if n, err := res.RowsAffected(); err == nil && n > 0 {
		return newTokenHash, nil
	}
	// Row already had a token, or a concurrent request won the write: return the
	// persisted value.
	account := new(auth.Account)
	if err := db.NewSelect().
		ModelTableExpr(accountTable).
		Where(whereID, accountID).
		Scan(ctx, account); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", &modelBase.DatabaseError{Op: "ensure calendar feed token", Err: err}
	}
	if account.CalendarFeedToken == nil {
		return "", nil
	}
	return *account.CalendarFeedToken, nil
}

// SetCalendarFeedToken sets or rotates the account's calendar feed token. The
// value stored is the SHA-256 hash of the raw token, not the raw token itself
// (the service hashes before calling).
func (r *AccountRepository) SetCalendarFeedToken(ctx context.Context, accountID int64, tokenHash string) error {
	_, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*auth.Account)(nil)).
		ModelTableExpr(accountTable).
		Set("calendar_feed_token = ?", tokenHash).
		Where(whereID, accountID).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "set calendar feed token", Err: err}
	}
	return nil
}

// FindByUsername retrieves an account by username
func (r *AccountRepository) FindByUsername(ctx context.Context, username string) (*auth.Account, error) {
	account := new(auth.Account)

	// Explicitly specify the schema and table
	err := base.GetDB(ctx, r.db).NewSelect().
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
	now := time.Now()
	account := &auth.Account{Model: modelBase.Model{ID: id}, LastLogin: &now}
	_, err := r.UpdateColumns(ctx, account, "last_login")
	return err
}

// UpdatePassword updates the password hash for an account and resets the
// OTP flag (a permanent password replaces any one-time password).
func (r *AccountRepository) UpdatePassword(ctx context.Context, id int64, passwordHash string) error {
	account := &auth.Account{Model: modelBase.Model{ID: id}, PasswordHash: &passwordHash, IsPasswordOTP: false}
	_, err := r.UpdateColumns(ctx, account, "password_hash", "is_password_otp")
	return err
}

// IncrementMFAAttempts atomically bumps mfa_attempts by one and sets
// mfa_locked_until = now() + lockoutDuration when the post-increment
// count is >= threshold. Returns the post-update counter and lock
// timestamp so the service can detect the lockout transition (exact
// threshold equality means *this* call crossed the line).
//
// The whole "read-mutate-write" pattern in the model layer was racy:
// two concurrent failed verifies both read mfa_attempts=N, both wrote
// N+1, and the counter only advanced by 1 — letting an attacker make
// 2N attempts before the threshold hit. Single SQL statement removes
// the race. (#1430 review item #6)
func (r *AccountRepository) IncrementMFAAttempts(ctx context.Context, id int64, threshold int, lockoutDuration time.Duration) (auth.MFAAttemptResult, error) {
	type incrementRow struct {
		MFAAttempts    int        `bun:"mfa_attempts"`
		MFALockedUntil *time.Time `bun:"mfa_locked_until"`
	}
	row := new(incrementRow)
	_, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*auth.Account)(nil)).
		ModelTableExpr(accountTable).
		Set("mfa_attempts = mfa_attempts + 1").
		Set(
			"mfa_locked_until = CASE WHEN mfa_attempts + 1 >= ? THEN now() + (? * interval '1 second') ELSE mfa_locked_until END",
			threshold, int64(lockoutDuration.Seconds()),
		).
		Where(whereID, id).
		Returning("mfa_attempts, mfa_locked_until").
		Exec(ctx, row)
	if err != nil {
		return auth.MFAAttemptResult{}, &modelBase.DatabaseError{
			Op:  "increment mfa attempts",
			Err: err,
		}
	}
	return auth.MFAAttemptResult{
		Attempts:    row.MFAAttempts,
		LockedUntil: row.MFALockedUntil,
	}, nil
}

// ResetMFAAttempts atomically clears mfa_attempts and mfa_locked_until.
// Used after a successful verify so a stale in-memory Account.Update
// can't accidentally re-set a concurrent racer's incremented counter.
func (r *AccountRepository) ResetMFAAttempts(ctx context.Context, id int64) error {
	_, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*auth.Account)(nil)).
		ModelTableExpr(accountTable).
		Set("mfa_attempts = 0").
		Set("mfa_locked_until = NULL").
		Where(whereID, id).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "reset mfa attempts",
			Err: err,
		}
	}
	return nil
}

// IncrementPINAttempts atomically bumps pin_attempts by one and sets
// pin_locked_until = now() + lockoutDuration when the post-increment count
// is >= threshold. Returns the post-update counter and lock timestamp so the
// caller can detect the lockout transition (exact threshold equality means
// *this* call crossed the line).
//
// This replaces the old model-level Account.IncrementPINAttempts() +
// accountRepo.Update() read-modify-write, which was racy: two concurrent
// failed PIN entries both read pin_attempts=N and both wrote N+1, advancing
// the counter by only 1 and letting an attacker double their attempt budget.
// A single SQL statement removes the race (issue #586, mirrors the MFA fix).
func (r *AccountRepository) IncrementPINAttempts(ctx context.Context, id int64, threshold int, lockoutDuration time.Duration) (auth.PINAttemptResult, error) {
	type incrementRow struct {
		PINAttempts    int        `bun:"pin_attempts"`
		PINLockedUntil *time.Time `bun:"pin_locked_until"`
	}
	row := new(incrementRow)
	_, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*auth.Account)(nil)).
		ModelTableExpr(accountTable).
		Set("pin_attempts = pin_attempts + 1").
		Set(
			"pin_locked_until = CASE WHEN pin_attempts + 1 >= ? THEN now() + (? * interval '1 second') ELSE pin_locked_until END",
			threshold, int64(lockoutDuration.Seconds()),
		).
		Where(whereID, id).
		Returning("pin_attempts, pin_locked_until").
		Exec(ctx, row)
	if err != nil {
		return auth.PINAttemptResult{}, &modelBase.DatabaseError{
			Op:  "increment pin attempts",
			Err: err,
		}
	}
	return auth.PINAttemptResult{
		Attempts:    row.PINAttempts,
		LockedUntil: row.PINLockedUntil,
	}, nil
}

// ResetPINAttempts atomically clears pin_attempts and pin_locked_until after a
// successful PIN verify so a stale in-memory Account.Update can't re-set a
// concurrent racer's incremented counter.
func (r *AccountRepository) ResetPINAttempts(ctx context.Context, id int64) error {
	_, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*auth.Account)(nil)).
		ModelTableExpr(accountTable).
		Set("pin_attempts = 0").
		Set("pin_locked_until = NULL").
		Where(whereID, id).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "reset pin attempts",
			Err: err,
		}
	}
	return nil
}

// SetActive toggles only the active flag for an account. Targeted update so
// it does not clobber password_hash or other in-memory stale fields.
func (r *AccountRepository) SetActive(ctx context.Context, id int64, active bool) error {
	_, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*auth.Account)(nil)).
		ModelTableExpr(accountTable).
		Set("active = ?", active).
		Where(whereID, id).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "set active",
			Err: err,
		}
	}

	return nil
}

// UpdateAvatar updates the global avatar path for an account.
func (r *AccountRepository) UpdateAvatar(ctx context.Context, id int64, avatar string) error {
	account := &auth.Account{Model: modelBase.Model{ID: id}, Avatar: avatar}
	_, err := r.UpdateColumns(ctx, account, "avatar")
	return err
}

// FindByRole retrieves accounts that have a specific role
func (r *AccountRepository) FindByRole(ctx context.Context, role string) ([]*auth.Account, error) {
	var accounts []*auth.Account

	// Use a SQL JOIN to retrieve accounts with the specified role
	// This assumes your account_roles table exists and has the proper foreign keys
	err := base.GetDB(ctx, r.db).NewSelect().
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
	query := base.GetDB(ctx, r.db).NewSelect().Model(&accounts).ModelTableExpr(accountTableAlias)

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

		// Batch-load roles and permissions for all accounts (3 queries instead of 3N)
		if err := r.loadAllAccountRolesAndPermissions(ctx, tx, accounts); err != nil {
			return err
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

// FindEmailsByAccountIDs batch-loads email addresses for the given account IDs.
// Returns a map of accountID → email.
func (r *AccountRepository) FindEmailsByAccountIDs(ctx context.Context, accountIDs []int64) (map[int64]string, error) {
	if len(accountIDs) == 0 {
		return make(map[int64]string), nil
	}

	type accountEmailRow struct {
		ID    int64  `bun:"id"`
		Email string `bun:"email"`
	}

	var rows []accountEmailRow
	err := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(accountTable).
		Column("id", "email").
		Where("id IN (?)", bun.List(accountIDs)).
		Scan(ctx, &rows)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find emails by account IDs",
			Err: err,
		}
	}

	result := make(map[int64]string, len(rows))
	for _, row := range rows {
		result[row.ID] = row.Email
	}

	return result, nil
}

// FindAvatarsByAccountIDs batch-loads avatar paths for the given account IDs.
// Returns a map of accountID → avatar path (only accounts with non-empty avatars).
func (r *AccountRepository) FindAvatarsByAccountIDs(ctx context.Context, accountIDs []int64) (map[int64]string, error) {
	if len(accountIDs) == 0 {
		return make(map[int64]string), nil
	}

	type accountAvatarRow struct {
		ID     int64  `bun:"id"`
		Avatar string `bun:"avatar"`
	}

	var rows []accountAvatarRow
	err := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(accountTable).
		Column("id", "avatar").
		Where("id IN (?)", bun.List(accountIDs)).
		Where("avatar IS NOT NULL").
		Where("avatar != ''").
		Scan(ctx, &rows)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find avatars by account IDs",
			Err: err,
		}
	}

	result := make(map[int64]string, len(rows))
	for _, row := range rows {
		result[row.ID] = row.Avatar
	}

	return result, nil
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

// accountRole is a result struct for batch-loading roles with their account association.
type accountRole struct {
	AccountID int64 `bun:"account_id"`
	auth.Role `bun:"role,extend"`
}

// accountPermission is a result struct for batch-loading permissions with their account association.
type accountPermission struct {
	AccountID       int64 `bun:"account_id"`
	auth.Permission `bun:"permission,extend"`
}

// loadAllAccountRolesAndPermissions batch-loads roles and permissions for all accounts
// using 3 queries instead of 3 per account.
func (r *AccountRepository) loadAllAccountRolesAndPermissions(ctx context.Context, tx bun.Tx, accounts []*auth.Account) error {
	if len(accounts) == 0 {
		return nil
	}

	// Collect all account IDs
	accountIDs := make([]int64, len(accounts))
	for i, a := range accounts {
		accountIDs[i] = a.ID
	}

	// 1. Batch-load roles for all accounts
	// Select only columns mapped in auth.Role to avoid scanning unmapped DB columns (e.g. metadata)
	var roleResults []accountRole
	err := tx.NewSelect().
		TableExpr(`auth.roles AS "role"`).
		ColumnExpr(`ar.account_id`).
		ColumnExpr(`"role".id, "role".created_at, "role".updated_at, "role".name, "role".description, "role".is_system`).
		Join(`JOIN auth.account_roles AS ar ON ar.role_id = "role".id`).
		Where("ar.account_id IN (?)", bun.List(accountIDs)).
		Scan(ctx, &roleResults)
	if err != nil {
		return err
	}

	rolesByAccount := make(map[int64][]*auth.Role, len(accounts))
	for i := range roleResults {
		ar := roleResults[i]
		role := ar.Role
		rolesByAccount[ar.AccountID] = append(rolesByAccount[ar.AccountID], &role)
	}

	// 2. Batch-load direct permissions for all accounts
	var directPermResults []accountPermission
	err = tx.NewSelect().
		TableExpr(`auth.permissions AS "permission"`).
		ColumnExpr(`ap.account_id`).
		ColumnExpr(`"permission".id, "permission".created_at, "permission".updated_at, "permission".name, "permission".description, "permission".resource, "permission".action`).
		Join(`JOIN auth.account_permissions AS ap ON ap.permission_id = "permission".id`).
		Where("ap.account_id IN (?)", bun.List(accountIDs)).
		Where("ap.granted = true").
		Scan(ctx, &directPermResults)
	if err != nil {
		return err
	}

	directPermsByAccount := make(map[int64][]*auth.Permission, len(accounts))
	for i := range directPermResults {
		p := directPermResults[i]
		perm := p.Permission
		directPermsByAccount[p.AccountID] = append(directPermsByAccount[p.AccountID], &perm)
	}

	// 3. Batch-load role-based permissions for all accounts
	var rolePermResults []accountPermission
	err = tx.NewSelect().
		TableExpr(`auth.permissions AS "permission"`).
		ColumnExpr(`ar.account_id`).
		ColumnExpr(`"permission".id, "permission".created_at, "permission".updated_at, "permission".name, "permission".description, "permission".resource, "permission".action`).
		Join(`JOIN auth.role_permissions AS rp ON rp.permission_id = "permission".id`).
		Join(`JOIN auth.account_roles AS ar ON ar.role_id = rp.role_id`).
		Where("ar.account_id IN (?)", bun.List(accountIDs)).
		Scan(ctx, &rolePermResults)
	if err != nil {
		return err
	}

	rolePermsByAccount := make(map[int64][]*auth.Permission, len(accounts))
	for i := range rolePermResults {
		p := rolePermResults[i]
		perm := p.Permission
		rolePermsByAccount[p.AccountID] = append(rolePermsByAccount[p.AccountID], &perm)
	}

	// 4. Assign results to each account
	for _, account := range accounts {
		account.Roles = rolesByAccount[account.ID]
		account.Permissions = r.mergePermissions(
			directPermsByAccount[account.ID],
			rolePermsByAccount[account.ID],
		)
	}

	return nil
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

// Update overrides the base Update method to handle email normalization
func (r *AccountRepository) Update(ctx context.Context, account *auth.Account) error {
	if account == nil {
		return fmt.Errorf("account cannot be nil")
	}

	// Validate account - this will also normalize the email
	if err := account.Validate(); err != nil {
		return err
	}

	// Execute the query using GetDB for transaction support
	_, err := base.GetDB(ctx, r.db).NewUpdate().
		Model(account).
		Where(whereID, account.ID).
		ModelTableExpr(accountTable).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update",
			Err: err,
		}
	}

	return nil
}

// AnonymizeForDeletion overwrites the account's email with the given
// anonymized placeholder and clears the username. Custom method
// (backend-conventions Rule 2): GDPR person-deletion step that pairs two
// column writes into one statement; used by operator SoftDeletePerson.
func (r *AccountRepository) AnonymizeForDeletion(ctx context.Context, accountID int64, anonymizedEmail string) error {
	_, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*auth.Account)(nil)).
		ModelTableExpr(accountTableAlias).
		Set(`email = ?`, anonymizedEmail).
		Set(`username = NULL`).
		Where(`"account".id = ?`, accountID).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "anonymize account for deletion",
			Err: err,
		}
	}
	return nil
}
