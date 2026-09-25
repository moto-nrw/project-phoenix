package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/guardianlinkview"
	"github.com/moto-nrw/project-phoenix/modules/studentdirectoryview"
	"github.com/moto-nrw/project-phoenix/tenant"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	repoBase "github.com/moto-nrw/project-phoenix/database/repositories/base"
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
	db                *bun.DB
	portalMemberships PortalMembershipQuery
}

// GuardianProfileOption configures a GuardianProfileRepository at construction.
type GuardianProfileOption func(*GuardianProfileRepository)

// WithPortalMemberships binds account reachability without exposing owner SQL.
func WithPortalMemberships(query PortalMembershipQuery) GuardianProfileOption {
	return func(r *GuardianProfileRepository) { r.portalMemberships = query }
}

// NewGuardianProfileRepository creates a new GuardianProfileRepository instance
func NewGuardianProfileRepository(db *bun.DB, options ...GuardianProfileOption) users.GuardianProfileRepository {
	repository := &GuardianProfileRepository{
		Repository: repoBase.NewRepository[*users.GuardianProfile](db, "users.guardian_profiles", "GuardianProfile"),
		db:         db,
	}
	for _, option := range options {
		option(repository)
	}
	return repository
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

	query = repoBase.WithTenantFilter(ctx, query, "guardian_profile")

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
// The owner projection opens no transaction of its own: its store resolves the
// caller's ambient transaction from the context and runs on the root connection
// when there is none. Isolation therefore does not rest on it. Its result may
// include several schools for one account, so each profile is matched against
// its own school, never against membership in any school.
func (r *GuardianProfileRepository) FindActivePortalProfilesByIDs(ctx context.Context, ids []int64) (map[int64]*users.GuardianProfile, error) {
	if len(ids) == 0 {
		return make(map[int64]*users.GuardianProfile), nil
	}
	if r.portalMemberships == nil {
		return nil, errors.New("find active portal guardian profiles: portal membership query is required")
	}

	var profiles []*users.GuardianProfile
	query := repoBase.GetDB(ctx, r.db).NewSelect().
		Model(&profiles).
		ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`).
		Where(`"guardian_profile".id IN (?)`, bun.List(ids)).
		Where(`"guardian_profile".account_id IS NOT NULL`)

	query = repoBase.WithTenantFilter(ctx, query, "guardian_profile")

	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("failed to find active portal guardian profiles by ids: %w", err)
	}

	result := make(map[int64]*users.GuardianProfile, len(profiles))
	if len(profiles) == 0 {
		return result, nil
	}
	accountIDs := make([]int64, 0, len(profiles))
	for _, profile := range profiles {
		accountIDs = append(accountIDs, *profile.AccountID)
	}
	memberships, err := r.portalMemberships(ctx, accountIDs)
	if err != nil {
		return nil, fmt.Errorf("find active portal guardian memberships: %w", err)
	}
	for _, profile := range profiles {
		if slices.Contains(memberships[*profile.AccountID], profile.GetTenantID()) {
			result[profile.ID] = profile
		}
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

	query = repoBase.WithTenantFilter(ctx, query, "guardian_profile")

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
// FindByEmails retrieves the tenant's guardian profiles whose trimmed email
// is one of the given addresses, through the generic list filter.
func (r *GuardianProfileRepository) FindByEmails(ctx context.Context, emails []string) ([]*users.GuardianProfile, error) {
	return r.ListWithOptions(ctx, &base.QueryOptions{
		Filter: base.NewFilter().TrimIn("email", emails...),
	})
}

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

// LinkAccount links a guardian profile to a parent account. A removed contact
// may leave that account attached to a childless profile (#3477). Move only
// that orphaned association; retain both contacts and all relationship data.
func (r *GuardianProfileRepository) LinkAccount(ctx context.Context, profileID int64, accountID int64) error {
	if profileID <= 0 || accountID <= 0 {
		return users.ErrGuardianAccountConflict
	}
	return tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		return tenant.WithSavepoint(txCtx, func(linkCtx context.Context) error {
			return r.linkAccount(linkCtx, profileID, accountID)
		})
	})
}

func (r *GuardianProfileRepository) linkAccount(ctx context.Context, profileID, accountID int64) error {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return err
	}
	db := repoBase.GetDB(ctx, r.db)
	// Serialize invitations for the same account, including when it has no
	// profile yet. Row locks also stop concurrent child FK inserts until commit.
	lockKey := fmt.Sprintf("guardian-account:%d:%d", tenantID.Int64(), accountID)
	if _, err := db.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", lockKey); err != nil {
		return fmt.Errorf("lock guardian account link: %w", err)
	}
	var profiles []*users.GuardianProfile
	if err := db.NewSelect().Model(&profiles).
		ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`).
		Where(`"guardian_profile".tenant_id = ?`, tenantID.Int64()).
		Where(`("guardian_profile".id = ? OR "guardian_profile".account_id = ?)`, profileID, accountID).
		OrderExpr(`"guardian_profile".id`).For("UPDATE").Scan(ctx); err != nil {
		return fmt.Errorf("lock guardian profiles: %w", err)
	}
	var target, previous *users.GuardianProfile
	for _, profile := range profiles {
		if profile.ID == profileID {
			target = profile
		} else {
			previous = profile
		}
	}
	if target == nil {
		return users.ErrGuardianProfileNotFound
	}
	if target.AccountID != nil && *target.AccountID != accountID {
		return users.ErrGuardianAccountConflict
	}
	if previous != nil {
		linked, err := db.NewSelect().TableExpr("users.student_guardian_relationships").
			Where("tenant_id = ?", tenantID.Int64()).
			Where("guardian_profile_id = ?", previous.ID).Exists(ctx)
		if err != nil {
			return fmt.Errorf("check previous guardian children: %w", err)
		}
		if linked {
			return users.ErrGuardianAccountConflict
		}
		if _, err := db.NewUpdate().Model((*users.GuardianProfile)(nil)).
			ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`).
			Set("account_id = NULL").Set("has_account = ?", false).
			Where(`"guardian_profile".tenant_id = ?`, tenantID.Int64()).
			Where(`"guardian_profile".id = ?`, previous.ID).Exec(ctx); err != nil {
			return fmt.Errorf("unlink childless guardian profile: %w", err)
		}
	}
	result, err := db.NewUpdate().
		Model((*users.GuardianProfile)(nil)).
		ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`).
		Set("account_id = ?", accountID).
		Set("has_account = ?", true).
		Where(`"guardian_profile".tenant_id = ?`, tenantID.Int64()).
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
// schema join (guardian relationships → users.students →
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
	childErr := guardianlinkview.Query(repoBase.GetDB(ctx, r.db), tenant.FromContext(ctx)).
		ColumnExpr(`"s".id AS student_id`).
		ColumnExpr(`"p".first_name`).
		ColumnExpr(`"p".last_name`).
		ColumnExpr(`"s".school_class`).
		// Lifecycle status travels with the summary so the reuse picker can
		// drop a child the enrolled-student gate would reject anyway (#1663).
		ColumnExpr(`"s".status`).
		// Per-relationship enrollment-submit permission, so the form can offer
		// reuse only for children this guardian may actually re-enroll (#1663).
		ColumnExpr(`COALESCE(("student_guardian".permissions ->> ?)::boolean, false) AS enrollment_submit`, authorize.GuardianPermissionEnrollmentSubmit).
		Join(`INNER JOIN (?) AS "s" ON "s".id = "student_guardian".student_id`, studentdirectoryview.Query(repoBase.GetDB(ctx, r.db), tenant.FromContext(ctx))).
		Join(`INNER JOIN users.persons AS "p" ON "p".id = "s".person_id`).
		Where(`"student_guardian".guardian_profile_id = ?`, profile.ID).
		Where(`COALESCE(("student_guardian".permissions ->> ?)::boolean, false) = TRUE`, authorize.GuardianPermissionPortalAccess).
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
