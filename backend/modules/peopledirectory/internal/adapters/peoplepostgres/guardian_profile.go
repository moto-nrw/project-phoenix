package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/uptrace/bun"

	users "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/rows"
	ports "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

// Error messages (S1192 - avoid duplicate string literals)
const (
	errRowsAffected = "failed to get rows affected: %w"
)

// The parent-portal permissions the guardian reads filter on, by their stored
// key (auth/authorize.GuardianPermission*). This package may not import the
// authorization runtime; the People Directory domain keeps the same copy.
const (
	guardianPermissionPortalAccess     = "parent_portal.access"
	guardianPermissionEnrollmentSubmit = "parent_portal.enrollment.submit"
)

// guardianProfileRepository implements the GuardianProfileRepository interface.
type guardianProfileRepository struct {
	runtime                Runtime
	students               StudentDirectoryRows
	links                  GuardianLinkRows
	portalMemberships      PortalMembershipQuery
	portalLoginMemberships PortalLoginMembershipQuery
}

// GuardianProfileOption configures a GuardianProfileRepository at construction.
type GuardianProfileOption func(*guardianProfileRepository)

// WithPortalMemberships binds account reachability without exposing owner SQL.
func WithPortalMemberships(query PortalMembershipQuery) GuardianProfileOption {
	return func(r *guardianProfileRepository) { r.portalMemberships = query }
}

// WithPortalLoginMemberships binds parent-login reachability without exposing
// owner SQL.
func WithPortalLoginMemberships(query PortalLoginMembershipQuery) GuardianProfileOption {
	return func(r *guardianProfileRepository) { r.portalLoginMemberships = query }
}

// newRawGuardianProfileRepository creates a new GuardianProfileRepository instance
func newRawGuardianProfileRepository(runtime Runtime, students StudentDirectoryRows, links GuardianLinkRows, options ...GuardianProfileOption) rawGuardianProfileRepository {
	repository := &guardianProfileRepository{
		runtime:  requireRuntime(runtime),
		students: requireStudentRows(students),
		links:    requireGuardianLinkRows(links),
	}
	for _, option := range options {
		option(repository)
	}
	return repository
}

// Create inserts a new guardian profile into the database
func (r *guardianProfileRepository) Create(ctx context.Context, profile *users.GuardianProfile) error {
	if err := profile.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	ensureTenantID(ctx, r.runtime, profile)

	// Get the database connection (or transaction if in context)
	ambientDB1, databaseErr1 := r.runtime.DB(ctx)
	if databaseErr1 != nil {
		return databaseErr1
	}
	db := ambientDB1

	_, err := db.NewInsert().
		Model(profile).
		ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`).
		Exec(ctx)

	if err != nil {
		return guardianWriteError("failed to create guardian profile", err)
	}

	return nil
}

// FindByID retrieves a guardian profile by their ID
func (r *guardianProfileRepository) FindByID(ctx context.Context, id int64) (*users.GuardianProfile, error) {
	profile := new(users.GuardianProfile)

	ambientDB2, databaseErr2 := r.runtime.DB(ctx)
	if databaseErr2 != nil {
		return *new(*users.GuardianProfile), databaseErr2
	}
	err := ambientDB2.NewSelect().
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
func (r *guardianProfileRepository) FindByIDs(ctx context.Context, ids []int64) (map[int64]*users.GuardianProfile, error) {
	if len(ids) == 0 {
		return make(map[int64]*users.GuardianProfile), nil
	}

	var profiles []*users.GuardianProfile
	ambientDB3, databaseErr3 := r.runtime.DB(ctx)
	if databaseErr3 != nil {
		return *new(map[int64]*users.GuardianProfile), databaseErr3
	}
	query := ambientDB3.NewSelect().
		Model(&profiles).
		ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`).
		Where(`"guardian_profile".id IN (?)`, bun.List(ids))

	query = withTenantFilter(ctx, r.runtime, query, "guardian_profile")

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
// to an active account with active membership and guardian role at the current
// tenant. It identifies portal participants for recipient and sharing flows;
// it does not assert that the account can authenticate with a password.
//
// The owner projection opens no transaction of its own: its store resolves the
// caller's ambient transaction from the context and fails when there is none. Its result may
// include several schools for one account, so each profile is matched against
// its own school, never against membership in any school.
func (r *guardianProfileRepository) FindActivePortalProfilesByIDs(ctx context.Context, ids []int64) (map[int64]*users.GuardianProfile, error) {
	if len(ids) == 0 {
		return make(map[int64]*users.GuardianProfile), nil
	}
	if r.portalMemberships == nil {
		return nil, errors.New("find active portal guardian profiles: portal membership query is required")
	}
	return r.findPortalProfilesByIDs(ctx, ids, r.portalMemberships, "find active portal guardian memberships")
}

// FindLoginReadyPortalProfilesByIDs returns guardian profiles whose linked
// account can authenticate with a password at the current tenant.
func (r *guardianProfileRepository) FindLoginReadyPortalProfilesByIDs(ctx context.Context, ids []int64) (map[int64]*users.GuardianProfile, error) {
	if len(ids) == 0 {
		return make(map[int64]*users.GuardianProfile), nil
	}
	if r.portalLoginMemberships == nil {
		return nil, errors.New("find login-ready portal guardian profiles: portal login membership query is required")
	}
	return r.findPortalProfilesByIDs(ctx, ids, r.portalLoginMemberships, "find login-ready portal guardian memberships")
}

func (r *guardianProfileRepository) findPortalProfilesByIDs(
	ctx context.Context,
	ids []int64,
	membershipsQuery func(context.Context, []int64) (map[int64][]int64, error),
	errorPrefix string,
) (map[int64]*users.GuardianProfile, error) {
	if len(ids) == 0 {
		return make(map[int64]*users.GuardianProfile), nil
	}

	var profiles []*users.GuardianProfile
	ambientDB4, databaseErr4 := r.runtime.DB(ctx)
	if databaseErr4 != nil {
		return *new(map[int64]*users.GuardianProfile), databaseErr4
	}
	query := ambientDB4.NewSelect().
		Model(&profiles).
		ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`).
		Where(`"guardian_profile".id IN (?)`, bun.List(ids)).
		Where(`"guardian_profile".account_id IS NOT NULL`)

	query = withTenantFilter(ctx, r.runtime, query, "guardian_profile")

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
	memberships, err := membershipsQuery(ctx, accountIDs)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errorPrefix, err)
	}
	for _, profile := range profiles {
		if slices.Contains(memberships[*profile.AccountID], profile.GetTenantID()) {
			result[profile.ID] = profile
		}
	}
	return result, nil
}

// LockByIDForUpdate locks a guardian profile row for the current transaction.
func (r *guardianProfileRepository) LockByIDForUpdate(ctx context.Context, id int64) error {
	var profileID int64
	ambientDB5, databaseErr5 := r.runtime.DB(ctx)
	if databaseErr5 != nil {
		return databaseErr5
	}
	query := ambientDB5.NewSelect().
		TableExpr(`users.guardian_profiles AS "guardian_profile"`).
		ColumnExpr(`"guardian_profile".id`).
		Where(`"guardian_profile".id = ?`, id).
		For("UPDATE")

	query = withTenantFilter(ctx, r.runtime, query, "guardian_profile")

	if err := query.Scan(ctx, &profileID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return users.ErrGuardianProfileNotFound
		}
		return fmt.Errorf("failed to lock guardian profile: %w", err)
	}
	return nil
}

// FindByEmail retrieves a guardian profile by their email address
func (r *guardianProfileRepository) FindByEmail(ctx context.Context, email string) (*users.GuardianProfile, error) {
	profile := new(users.GuardianProfile)

	ambientDB6, databaseErr6 := r.runtime.DB(ctx)
	if databaseErr6 != nil {
		return *new(*users.GuardianProfile), databaseErr6
	}
	err := ambientDB6.NewSelect().
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
func (r *guardianProfileRepository) FindByAccountID(ctx context.Context, accountID int64) (*users.GuardianProfile, error) {
	profile := new(users.GuardianProfile)

	// A parent account is cross-tenant and can own one guardian_profiles row
	// per school, so several rows may share account_id. Order deterministically:
	// rows with an explicit portal_locale first (NULLs last), then by id. This
	// makes a returning parent's saved portal language win even when a newer
	// enrollment inserted a fresh row whose portal_locale is still NULL, instead
	// of the previous nondeterministic row pick. Limit(1) keeps the single-row
	// Scan intentional.
	ambientDB7, databaseErr7 := r.runtime.DB(ctx)
	if databaseErr7 != nil {
		return *new(*users.GuardianProfile), databaseErr7
	}
	err := ambientDB7.NewSelect().
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
func (r *guardianProfileRepository) FindWithoutAccount(ctx context.Context) ([]*users.GuardianProfile, error) {
	var profiles []*users.GuardianProfile

	ambientDB8, databaseErr8 := r.runtime.DB(ctx)
	if databaseErr8 != nil {
		return *new([]*users.GuardianProfile), databaseErr8
	}
	err := ambientDB8.NewSelect().
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
func (r *guardianProfileRepository) FindInvitable(ctx context.Context) ([]*users.GuardianProfile, error) {
	var profiles []*users.GuardianProfile

	ambientDB9, databaseErr9 := r.runtime.DB(ctx)
	if databaseErr9 != nil {
		return *new([]*users.GuardianProfile), databaseErr9
	}
	err := ambientDB9.NewSelect().
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
func (r *guardianProfileRepository) FindByEmails(ctx context.Context, emails []string) ([]*users.GuardianProfile, error) {
	return r.ListWithOptions(ctx, &ports.QueryOptions{
		Filter: ports.NewQueryFilter().TrimIn("email", emails...),
	})
}

func (r *guardianProfileRepository) ListWithOptions(ctx context.Context, options *ports.QueryOptions) ([]*users.GuardianProfile, error) {
	listOptions := &ports.QueryOptions{}
	if options != nil {
		*listOptions = *options
	}
	fields := make([]ports.QuerySortField, 0, 2)
	if options != nil && options.Sorting != nil {
		fields = append(fields, options.Sorting.Fields...)
	}
	fields = append(fields,
		ports.QuerySortField{Field: "last_name", Direction: ports.SortAsc},
		ports.QuerySortField{Field: "first_name", Direction: ports.SortAsc},
	)
	listOptions.Sorting = &ports.QuerySorting{Fields: fields}

	profiles := make([]*users.GuardianProfile, 0)
	if listOptions.Filter != nil {
		listOptions.Filter.WithTableAlias("guardian_profile")
	}
	ambientDB10, databaseErr10 := r.runtime.DB(ctx)
	if databaseErr10 != nil {
		return *new([]*users.GuardianProfile), databaseErr10
	}
	query := ambientDB10.NewSelect().
		Model(&profiles).
		ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`)
	query = withTenantFilter(ctx, r.runtime, query, "guardian_profile")
	query = applyQueryOptions(query, listOptions)
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("failed to list guardian profiles: %w", err)
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
func (r *guardianProfileRepository) SearchByText(ctx context.Context, searchText string, limit int) ([]*users.GuardianProfile, error) {
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

	ambientDB11, databaseErr11 := r.runtime.DB(ctx)
	if databaseErr11 != nil {
		return *new([]*users.GuardianProfile), databaseErr11
	}
	query := ambientDB11.NewSelect().
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
		Limit(int64(limit)).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("failed to search guardian profiles: %w", err)
	}

	return profiles, nil
}

// Update updates an existing guardian profile
func (r *guardianProfileRepository) Update(ctx context.Context, profile *users.GuardianProfile) error {
	if err := profile.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	ambientDB12, databaseErr12 := r.runtime.DB(ctx)
	if databaseErr12 != nil {
		return databaseErr12
	}
	result, err := ambientDB12.NewUpdate().
		Model(profile).
		ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`).
		Where(`"guardian_profile".id = ?`, profile.ID).
		Exec(ctx)

	if err != nil {
		return guardianWriteError("failed to update guardian profile", err)
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
func (r *guardianProfileRepository) Delete(ctx context.Context, id int64) error {
	ambientDB13, databaseErr13 := r.runtime.DB(ctx)
	if databaseErr13 != nil {
		return databaseErr13
	}
	result, err := ambientDB13.NewDelete().
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
func (r *guardianProfileRepository) LinkAccount(ctx context.Context, profileID int64, accountID int64) error {
	if profileID <= 0 || accountID <= 0 {
		return users.ErrGuardianAccountConflict
	}
	return r.runtime.WithSavepoint(ctx, func(linkCtx context.Context) error {
		return r.linkAccount(linkCtx, profileID, accountID)
	})
}

func (r *guardianProfileRepository) linkAccount(ctx context.Context, profileID, accountID int64) error {
	tenantID, err := r.runtime.RequireTenantID(ctx)
	if err != nil {
		return err
	}
	db, err := r.runtime.DB(ctx)
	if err != nil {
		return err
	}
	if err := lockGuardianAccount(ctx, db, tenantID, accountID); err != nil {
		return err
	}
	var profiles []*users.GuardianProfile
	if err := db.NewSelect().Model(&profiles).
		ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`).
		Where(`"guardian_profile".tenant_id = ?`, tenantID).
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
	if err := unlinkChildlessGuardianProfile(ctx, db, tenantID, previous); err != nil {
		return err
	}
	result, err := db.NewUpdate().
		Model((*users.GuardianProfile)(nil)).
		ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`).
		Set("account_id = ?", accountID).
		Set("has_account = ?", true).
		Where(`"guardian_profile".tenant_id = ?`, tenantID).
		Where(`"guardian_profile".id = ?`, profileID).
		Exec(ctx)

	return guardianAccountLinkResult(result, err)
}

// LoadProfileWithChildren returns the guardian profile linked to the
// account plus the primary phone + active-student summaries. Multi-
// schema join (guardian relationships → users.students →
// users.persons) lives here so handlers/services don't reach into the
// schema directly. Returns (nil, nil) when no profile is linked under
// the current tenant context.
func (r *guardianProfileRepository) LoadProfileWithChildren(ctx context.Context, accountID int64) (*users.GuardianProfileWithChildren, error) {
	profile := new(users.GuardianProfile)
	ambientDB15, databaseErr15 := r.runtime.DB(ctx)
	if databaseErr15 != nil {
		return *new(*users.GuardianProfileWithChildren), databaseErr15
	}
	err := ambientDB15.NewSelect().
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

	result.PrimaryPhone, err = r.primaryPhone(ctx, profile.ID)
	if err != nil {
		return nil, err
	}

	result.Children, err = r.guardianChildren(ctx, profile.ID)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func unlinkChildlessGuardianProfile(ctx context.Context, db bun.IDB, tenantID int64, previous *users.GuardianProfile) error {
	if previous == nil {
		return nil
	}
	linked, err := db.NewSelect().TableExpr("users.student_guardian_relationships").
		Where("tenant_id = ?", tenantID).
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
		Where(`"guardian_profile".tenant_id = ?`, tenantID).
		Where(`"guardian_profile".id = ?`, previous.ID).Exec(ctx); err != nil {
		return fmt.Errorf("unlink childless guardian profile: %w", err)
	}
	return nil
}

func (r *guardianProfileRepository) primaryPhone(ctx context.Context, profileID int64) (string, error) {
	var primaryPhone users.GuardianPhoneNumber
	ambientDB16, databaseErr16 := r.runtime.DB(ctx)
	if databaseErr16 != nil {
		return *new(string), databaseErr16
	}
	phoneErr := ambientDB16.NewSelect().
		Model(&primaryPhone).
		ModelTableExpr(`users.guardian_phone_numbers AS "guardian_phone_number"`).
		Where(`"guardian_phone_number".guardian_profile_id = ?`, profileID).
		OrderExpr(`"guardian_phone_number".is_primary DESC, "guardian_phone_number".id ASC`).
		Limit(1).
		Scan(ctx)
	if phoneErr != nil && !errors.Is(phoneErr, sql.ErrNoRows) {
		return "", fmt.Errorf("failed to load primary phone: %w", phoneErr)
	}
	if phoneErr == nil && primaryPhone.PhoneNumber != "" {
		return primaryPhone.PhoneNumber, nil
	}

	return "", nil
}

func lockGuardianAccount(ctx context.Context, db bun.IDB, tenantID, accountID int64) error {
	// Serialize invitations for the same account, including when it has no
	// profile yet. Row locks also stop concurrent child FK inserts until commit.
	lockKey := fmt.Sprintf("guardian-account:%d:%d", tenantID, accountID)
	if _, err := db.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", lockKey); err != nil {
		return fmt.Errorf("lock guardian account link: %w", err)
	}
	return nil
}

func (r *guardianProfileRepository) guardianChildren(ctx context.Context, profileID int64) ([]users.GuardianChildSummary, error) {
	var children []users.GuardianChildSummary
	type childRow struct {
		StudentID        int64  `bun:"student_id"`
		FirstName        string `bun:"first_name"`
		LastName         string `bun:"last_name"`
		SchoolClass      string `bun:"school_class"`
		Status           string `bun:"status"`
		EnrollmentSubmit bool   `bun:"enrollment_submit"`
	}
	var rows []childRow
	ambientDB17, databaseErr17 := r.runtime.DB(ctx)
	if databaseErr17 != nil {
		return *new([]users.GuardianChildSummary), databaseErr17
	}
	childErr := guardianLinkRows(ctx, r.runtime, r.links, ambientDB17).
		ColumnExpr(`"s".id AS student_id`).
		ColumnExpr(`"p".first_name`).
		ColumnExpr(`"p".last_name`).
		ColumnExpr(`"s".school_class`).
		// Lifecycle status travels with the summary so the reuse picker can
		// drop a child the enrolled-student gate would reject anyway (#1663).
		ColumnExpr(`"s".status`).
		// Per-relationship enrollment-submit permission, so the form can offer
		// reuse only for children this guardian may actually re-enroll (#1663).
		ColumnExpr(`COALESCE(("student_guardian".permissions ->> ?)::boolean, false) AS enrollment_submit`, guardianPermissionEnrollmentSubmit).
		Join(`INNER JOIN (?) AS "s" ON "s".id = "student_guardian".student_id`, studentRows(ctx, r.runtime, r.students, ambientDB17)).
		Join(`INNER JOIN users.persons AS "p" ON "p".id = "s".person_id`).
		Where(`"student_guardian".guardian_profile_id = ?`, profileID).
		Where(`COALESCE(("student_guardian".permissions ->> ?)::boolean, false) = TRUE`, guardianPermissionPortalAccess).
		Where(`"s".status <> ?`, "alumnus").
		OrderExpr(`"p".last_name ASC, "p".first_name ASC`).
		Scan(ctx, &rows)
	if childErr != nil && !errors.Is(childErr, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to load children: %w", childErr)
	}
	for _, row := range rows {
		children = append(children, users.GuardianChildSummary{
			StudentID:        row.StudentID,
			FirstName:        row.FirstName,
			LastName:         row.LastName,
			SchoolClass:      row.SchoolClass,
			Status:           row.Status,
			EnrollmentSubmit: row.EnrollmentSubmit,
		})
	}

	return children, nil
}

func guardianAccountLinkResult(result sql.Result, err error) error {

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
