package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	repoBase "github.com/moto-nrw/project-phoenix/database/repositories/base"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// Error messages (S1192 - avoid duplicate string literals)
const (
	errRowsAffected = "failed to get rows affected: %w"
)

// GuardianProfileRepository implements the users.GuardianProfileRepository interface
type GuardianProfileRepository struct {
	*repoBase.Repository[*users.GuardianProfile]
	db *bun.DB
}

// NewGuardianProfileRepository creates a new GuardianProfileRepository instance
func NewGuardianProfileRepository(db *bun.DB) users.GuardianProfileRepository {
	return &GuardianProfileRepository{
		Repository: repoBase.NewRepository[*users.GuardianProfile](db, "users.guardian_profiles", "GuardianProfile"),
		db:         db,
	}
}

// Create inserts a new guardian profile into the database
func (r *GuardianProfileRepository) Create(ctx context.Context, profile *users.GuardianProfile) error {
	if err := profile.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	repoBase.EnsureTenantID(ctx, profile)

	// Get the database connection (or transaction if in context)
	db := repoBase.GetDB(ctx, r.db)

	_, err := db.NewInsert().
		Model(profile).
		ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to create guardian profile: %w", err)
	}

	return nil
}

// FindByID retrieves a guardian profile by their ID
func (r *GuardianProfileRepository) FindByID(ctx context.Context, id int64) (*users.GuardianProfile, error) {
	profile := new(users.GuardianProfile)

	err := repoBase.GetDB(ctx, r.db).NewSelect().
		Model(profile).
		ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`).
		Where(`"guardian_profile".id = ?`, id).
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, users.ErrGuardianProfileNotFound
		}
		return nil, fmt.Errorf("failed to find guardian profile: %w", err)
	}

	return profile, nil
}

// FindByIDs retrieves guardian profiles for the given ids in a single query,
// keyed by id for O(1) lookup. Missing ids are simply absent from the map.
// Tenant-scoped via RLS / TenantWhere, mirroring FindByIDs on the other repos.
func (r *GuardianProfileRepository) FindByIDs(ctx context.Context, ids []int64) (map[int64]*users.GuardianProfile, error) {
	if len(ids) == 0 {
		return make(map[int64]*users.GuardianProfile), nil
	}

	var profiles []*users.GuardianProfile
	query := repoBase.GetDB(ctx, r.db).NewSelect().
		Model(&profiles).
		ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`).
		Where(`"guardian_profile".id IN (?)`, bun.List(ids))

	if where, val, ok := repoBase.TenantWhere(ctx, "guardian_profile"); ok {
		query = query.Where(where, val)
	}

	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("failed to find guardian profiles by ids: %w", err)
	}

	result := make(map[int64]*users.GuardianProfile, len(profiles))
	for _, profile := range profiles {
		result[profile.ID] = profile
	}
	return result, nil
}

// FindActivePortalProfilesByIDs returns only guardian profiles that are linked
// to an account that can actually sign in to the parent portal for the current
// tenant. Reachability mirrors the parent login flow (services/auth): the
// tenant mapping (account_tenants.status) and the account itself
// (accounts.active) must be active, AND the account must hold the guardian role
// on that tenant. Parent login rejects accounts without the guardian role
// (ErrAccountNoGuardianRole), so a profile whose account lacks it must not be
// treated as reachable — otherwise staff could target a parent who can never
// see or answer the invitation.
//
// Runtime-role note: this runs under phoenix_tenant (the staff calendar routes
// use TenantTxMiddleware). That role has SELECT on every auth table via the
// "GRANT SELECT ON ALL TABLES IN SCHEMA auth TO phoenix_tenant" in migration
// 1.14.1 (no later revoke), and RLS permits the reads with app.current_tenant_id
// set: auth.roles allows tenant_id IS NULL (the guardian base role), and
// auth.account_roles / auth.account_tenants rows match the current tenant.
// It is NOT limited to phoenix_auth — these joins do not need an admin tx.
func (r *GuardianProfileRepository) FindActivePortalProfilesByIDs(ctx context.Context, ids []int64) (map[int64]*users.GuardianProfile, error) {
	if len(ids) == 0 {
		return make(map[int64]*users.GuardianProfile), nil
	}

	var profiles []*users.GuardianProfile
	query := repoBase.GetDB(ctx, r.db).NewSelect().
		Model(&profiles).
		ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`).
		Join(`INNER JOIN auth.account_tenants AS "account_tenant" ON "account_tenant".account_id = "guardian_profile".account_id AND "account_tenant".tenant_id = "guardian_profile".tenant_id`).
		Join(`INNER JOIN auth.accounts AS "account" ON "account".id = "guardian_profile".account_id`).
		Join(`INNER JOIN auth.account_roles AS "account_role" ON "account_role".account_id = "guardian_profile".account_id AND "account_role".tenant_id = "guardian_profile".tenant_id`).
		Join(`INNER JOIN auth.roles AS "role" ON "role".id = "account_role".role_id`).
		Where(`"guardian_profile".id IN (?)`, bun.List(ids)).
		Where(`"guardian_profile".account_id IS NOT NULL`).
		Where(`"account_tenant".status = ?`, authModels.AccountTenantStatusActive).
		Where(`"account".active = ?`, true).
		// Match the parent login guardian-role check (case-insensitive, mirrors
		// strings.EqualFold in services/auth). Deduped by the result map below.
		Where(`LOWER("role".name) = ?`, strings.ToLower(authModels.BaseRoleGuardian))

	if where, val, ok := repoBase.TenantWhere(ctx, "guardian_profile"); ok {
		query = query.Where(where, val)
	}

	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("failed to find active portal guardian profiles by ids: %w", err)
	}

	result := make(map[int64]*users.GuardianProfile, len(profiles))
	for _, profile := range profiles {
		result[profile.ID] = profile
	}
	return result, nil
}

// LockByIDForUpdate locks a guardian profile row for the current transaction.
func (r *GuardianProfileRepository) LockByIDForUpdate(ctx context.Context, id int64) error {
	var profileID int64
	query := repoBase.GetDB(ctx, r.db).NewSelect().
		TableExpr(`users.guardian_profiles AS "guardian_profile"`).
		ColumnExpr(`"guardian_profile".id`).
		Where(`"guardian_profile".id = ?`, id).
		For("UPDATE")

	if where, tenantID, ok := repoBase.TenantWhere(ctx, "guardian_profile"); ok {
		query.Where(where, tenantID)
	}

	if err := query.Scan(ctx, &profileID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return users.ErrGuardianProfileNotFound
		}
		return fmt.Errorf("failed to lock guardian profile: %w", err)
	}
	return nil
}

// FindByEmail retrieves a guardian profile by their email address
func (r *GuardianProfileRepository) FindByEmail(ctx context.Context, email string) (*users.GuardianProfile, error) {
	profile := new(users.GuardianProfile)

	err := repoBase.GetDB(ctx, r.db).NewSelect().
		Model(profile).
		ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`).
		Where(`LOWER("guardian_profile".email) = LOWER(?)`, email).
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, users.ErrGuardianProfileNotFound
		}
		return nil, fmt.Errorf("failed to find guardian profile by email: %w", err)
	}

	return profile, nil
}

// FindByEmails retrieves profiles keyed by trimmed, lower-case email.
func (r *GuardianProfileRepository) FindByEmails(ctx context.Context, emails []string) (map[string]*users.GuardianProfile, error) {
	normalized := make([]string, 0, len(emails))
	seen := make(map[string]bool, len(emails))
	for _, email := range emails {
		email = strings.ToLower(strings.TrimSpace(email))
		if email != "" && !seen[email] {
			seen[email] = true
			normalized = append(normalized, email)
		}
	}
	result := make(map[string]*users.GuardianProfile, len(normalized))
	if len(normalized) == 0 {
		return result, nil
	}
	rows := make([]*users.GuardianProfile, 0, len(normalized))
	if err := repoBase.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`).
		Where(`LOWER("guardian_profile".email) IN (?)`, bun.List(normalized)).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("failed to find guardian profiles by emails: %w", err)
	}
	for _, profile := range rows {
		if profile.Email != nil {
			result[strings.ToLower(strings.TrimSpace(*profile.Email))] = profile
		}
	}
	return result, nil
}

// FindByAccountID retrieves a guardian profile by their account ID
func (r *GuardianProfileRepository) FindByAccountID(ctx context.Context, accountID int64) (*users.GuardianProfile, error) {
	profile := new(users.GuardianProfile)

	// A parent account is cross-tenant and can own one guardian_profiles row
	// per school, so several rows may share account_id. Order deterministically:
	// rows with an explicit portal_locale first (NULLs last), then by id. This
	// makes a returning parent's saved portal language win even when a newer
	// enrollment inserted a fresh row whose portal_locale is still NULL, instead
	// of the previous nondeterministic row pick. Limit(1) keeps the single-row
	// Scan intentional.
	err := repoBase.GetDB(ctx, r.db).NewSelect().
		Model(profile).
		ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`).
		Where(`"guardian_profile".account_id = ?`, accountID).
		OrderExpr(`"guardian_profile".portal_locale IS NULL, "guardian_profile".id`).
		Limit(1).
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, users.ErrGuardianProfileNotFound
		}
		return nil, fmt.Errorf("failed to find guardian profile by account ID: %w", err)
	}

	return profile, nil
}

// FindWithoutAccount retrieves guardian profiles without portal accounts
func (r *GuardianProfileRepository) FindWithoutAccount(ctx context.Context) ([]*users.GuardianProfile, error) {
	var profiles []*users.GuardianProfile

	err := repoBase.GetDB(ctx, r.db).NewSelect().
		Model(&profiles).
		ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`).
		Where(`"guardian_profile".account_id IS NULL`).
		Where(`"guardian_profile".has_account = ?`, false).
		Order(`last_name ASC`, `first_name ASC`).
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to find guardians without account: %w", err)
	}

	return profiles, nil
}

// FindInvitable retrieves guardians who can be invited (has email, no account)
func (r *GuardianProfileRepository) FindInvitable(ctx context.Context) ([]*users.GuardianProfile, error) {
	var profiles []*users.GuardianProfile

	err := repoBase.GetDB(ctx, r.db).NewSelect().
		Model(&profiles).
		ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`).
		Where(`"guardian_profile".email IS NOT NULL`).
		Where(`"guardian_profile".email != ''`).
		Where(`"guardian_profile".account_id IS NULL`).
		Where(`"guardian_profile".has_account = ?`, false).
		Order(`last_name ASC`, `first_name ASC`).
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to find invitable guardians: %w", err)
	}

	return profiles, nil
}

// ListWithOptions retrieves guardian profiles with pagination and filters
func (r *GuardianProfileRepository) ListWithOptions(ctx context.Context, options *base.QueryOptions) ([]*users.GuardianProfile, error) {
	listOptions := &base.QueryOptions{}
	if options != nil {
		*listOptions = *options
	}
	fields := make([]base.SortField, 0, 2)
	if options != nil && options.Sorting != nil {
		fields = append(fields, options.Sorting.Fields...)
	}
	fields = append(fields,
		base.SortField{Field: "last_name", Direction: base.SortAsc},
		base.SortField{Field: "first_name", Direction: base.SortAsc},
	)
	listOptions.Sorting = &base.Sorting{Fields: fields}

	profiles, err := r.Repository.ListWithOptions(ctx, listOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to list guardian profiles: %w", repoBase.DatabaseErrorCause(err))
	}
	if len(profiles) == 0 {
		return nil, nil
	}

	return profiles, nil
}

// escapeLikePattern escapes the LIKE metacharacters (\, %, _) in user-supplied
// search text so they match literally instead of acting as wildcards. Without
// this a caller could pass "%" or "___" to defeat the picker's minimum-query-
// length guard and match the whole guardian pool (the picker is open to all
// staff, not just admins — see searchGuardiansForPicker). The query pairs this
// with an explicit ESCAPE '\' clause. Backslash is escaped first so the escapes
// added for % and _ are not themselves re-escaped.
func escapeLikePattern(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "%", `\%`)
	s = strings.ReplaceAll(s, "_", `\_`)
	return s
}

// maxSearchTokens bounds how many whitespace-separated terms a single picker
// search expands into, so a pathological query ("a b c d e …") can't build an
// unbounded WHERE clause. A real name search never needs more than a handful.
const maxSearchTokens = 10

// SearchByText retrieves guardian profiles matching the search text
// (case-insensitive substring across first name, last name, and email). Tenant
// isolation is enforced by RLS on the ambient tenant transaction (the endpoint
// runs under TenantTxMiddleware), mirroring the other read methods on this
// repository. Results are capped by limit so the guardian picker payload stays
// small.
//
// The search text is split into whitespace-separated tokens; EACH token must
// match at least one column, and ALL tokens must match (AND). This is what makes
// a full-name query like "Andrea Bauer" work even though "Andrea" lives in
// first_name and "Bauer" in last_name — a single full-string LIKE would match
// neither column and return nothing. Token order is irrelevant, so "Bauer
// Andrea" matches the same person.
//
// LIKE metacharacters in each token are escaped (see escapeLikePattern) so they
// match literally — the picker's enumeration guard relies on a real minimum
// query length, which raw "%"/"_" input would otherwise bypass.
func (r *GuardianProfileRepository) SearchByText(ctx context.Context, searchText string, limit int) ([]*users.GuardianProfile, error) {
	// strings.Fields splits on any run of whitespace and drops empties, so a
	// query that is only spaces yields no tokens → empty result.
	tokens := strings.Fields(strings.ToLower(searchText))
	if len(tokens) == 0 {
		return []*users.GuardianProfile{}, nil
	}
	if len(tokens) > maxSearchTokens {
		tokens = tokens[:maxSearchTokens]
	}
	if limit <= 0 {
		limit = 20
	}

	var profiles []*users.GuardianProfile

	query := repoBase.GetDB(ctx, r.db).NewSelect().
		Model(&profiles).
		ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`)

	// One AND-ed WHERE group per token; within a group the token may match any of
	// the three columns. The explicit parentheses keep each token's OR set
	// isolated so the groups combine as (t1col OR …) AND (t2col OR …).
	for _, tok := range tokens {
		pattern := "%" + escapeLikePattern(tok) + "%"
		query = query.Where(
			`(LOWER("guardian_profile".first_name) LIKE ? ESCAPE '\' OR LOWER("guardian_profile".last_name) LIKE ? ESCAPE '\' OR LOWER("guardian_profile".email) LIKE ? ESCAPE '\')`,
			pattern, pattern, pattern,
		)
	}

	if err := query.
		Order(`last_name ASC`, `first_name ASC`).
		Limit(limit).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("failed to search guardian profiles: %w", err)
	}

	return profiles, nil
}

// Update updates an existing guardian profile
func (r *GuardianProfileRepository) Update(ctx context.Context, profile *users.GuardianProfile) error {
	if err := profile.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	result, err := repoBase.GetDB(ctx, r.db).NewUpdate().
		Model(profile).
		ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`).
		Where(`"guardian_profile".id = ?`, profile.ID).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to update guardian profile: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf(errRowsAffected, err)
	}

	if rowsAffected == 0 {
		return users.ErrGuardianProfileNotFound
	}

	return nil
}

// UpdatePortalLocaleByAccountID updates portal_locale for every profile linked
// to the parent account. Parent accounts are cross-tenant, so callers run this
// under an admin transaction.
func (r *GuardianProfileRepository) UpdatePortalLocaleByAccountID(ctx context.Context, accountID int64, locale string) error {
	result, err := repoBase.GetDB(ctx, r.db).NewUpdate().
		Model((*users.GuardianProfile)(nil)).
		ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`).
		Set(`portal_locale = ?`, locale).
		Set(`updated_at = NOW()`).
		Where(`"guardian_profile".account_id = ?`, accountID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update guardian portal locale: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf(errRowsAffected, err)
	}
	// Fail loud when the account has no guardian_profiles row. Without this the
	// UPDATE matches zero rows, silently succeeds, and the caller reports the
	// preference as saved — but the next read returns NULL again. Surface it so
	// a "saved" choice that never persisted can't masquerade as success.
	if rowsAffected == 0 {
		return users.ErrGuardianProfileNotFound
	}
	return nil
}

// Delete removes a guardian profile
func (r *GuardianProfileRepository) Delete(ctx context.Context, id int64) error {
	result, err := repoBase.GetDB(ctx, r.db).NewDelete().
		Model((*users.GuardianProfile)(nil)).
		ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`).
		Where(`"guardian_profile".id = ?`, id).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to delete guardian profile: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf(errRowsAffected, err)
	}

	if rowsAffected == 0 {
		return users.ErrGuardianProfileNotFound
	}

	return nil
}

// LinkAccount links a guardian profile to a parent account
func (r *GuardianProfileRepository) LinkAccount(ctx context.Context, profileID int64, accountID int64) error {
	result, err := repoBase.GetDB(ctx, r.db).NewUpdate().
		Model((*users.GuardianProfile)(nil)).
		ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`).
		Set("account_id = ?", accountID).
		Set("has_account = ?", true).
		Where(`"guardian_profile".id = ?`, profileID).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to link account: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf(errRowsAffected, err)
	}

	if rowsAffected == 0 {
		return users.ErrGuardianProfileNotFound
	}

	return nil
}

// LoadProfileWithChildren returns the guardian profile linked to the
// account plus the primary phone + active-student summaries. Multi-
// schema join (users.students_guardians → users.students →
// users.persons) lives here so handlers/services don't reach into the
// schema directly. Returns (nil, nil) when no profile is linked under
// the current tenant context.
func (r *GuardianProfileRepository) LoadProfileWithChildren(ctx context.Context, accountID int64) (*users.GuardianProfileWithChildren, error) {
	profile := new(users.GuardianProfile)
	err := repoBase.GetDB(ctx, r.db).NewSelect().
		Model(profile).
		ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`).
		Where(`"guardian_profile".account_id = ?`, accountID).
		Limit(1).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to load guardian profile: %w", err)
	}

	result := &users.GuardianProfileWithChildren{Profile: profile}

	var primaryPhone users.GuardianPhoneNumber
	phoneErr := repoBase.GetDB(ctx, r.db).NewSelect().
		Model(&primaryPhone).
		ModelTableExpr(`users.guardian_phone_numbers AS "guardian_phone_number"`).
		Where(`"guardian_phone_number".guardian_profile_id = ?`, profile.ID).
		OrderExpr(`"guardian_phone_number".is_primary DESC, "guardian_phone_number".id ASC`).
		Limit(1).
		Scan(ctx)
	if phoneErr != nil && !errors.Is(phoneErr, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to load primary phone: %w", phoneErr)
	}
	if phoneErr == nil && primaryPhone.PhoneNumber != "" {
		result.PrimaryPhone = primaryPhone.PhoneNumber
	}

	type childRow struct {
		StudentID        int64  `bun:"student_id"`
		FirstName        string `bun:"first_name"`
		LastName         string `bun:"last_name"`
		SchoolClass      string `bun:"school_class"`
		Status           string `bun:"status"`
		EnrollmentSubmit bool   `bun:"enrollment_submit"`
	}
	var rows []childRow
	childErr := repoBase.GetDB(ctx, r.db).NewSelect().
		TableExpr(`users.students_guardians AS "sg"`).
		ColumnExpr(`"s".id AS student_id`).
		ColumnExpr(`"p".first_name`).
		ColumnExpr(`"p".last_name`).
		ColumnExpr(`"s".school_class`).
		// Lifecycle status travels with the summary so the reuse picker can
		// drop a child the enrolled-student gate would reject anyway (#1663).
		ColumnExpr(`"s".status`).
		// Per-relationship enrollment-submit permission, so the form can offer
		// reuse only for children this guardian may actually re-enroll (#1663).
		ColumnExpr(`COALESCE(("sg".permissions ->> ?)::boolean, false) AS enrollment_submit`, authorize.GuardianPermissionEnrollmentSubmit).
		Join(`INNER JOIN users.students AS "s" ON "s".id = "sg".student_id`).
		Join(`INNER JOIN users.persons AS "p" ON "p".id = "s".person_id`).
		Where(`"sg".guardian_profile_id = ?`, profile.ID).
		Where(`COALESCE(("sg".permissions ->> ?)::boolean, false) = TRUE`, authorize.GuardianPermissionPortalAccess).
		Where(`"s".status <> ?`, "alumnus").
		OrderExpr(`"p".last_name ASC, "p".first_name ASC`).
		Scan(ctx, &rows)
	if childErr != nil && !errors.Is(childErr, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to load children: %w", childErr)
	}
	for _, row := range rows {
		result.Children = append(result.Children, users.GuardianChildSummary{
			StudentID:        row.StudentID,
			FirstName:        row.FirstName,
			LastName:         row.LastName,
			SchoolClass:      row.SchoolClass,
			Status:           row.Status,
			EnrollmentSubmit: row.EnrollmentSubmit,
		})
	}

	return result, nil
}
