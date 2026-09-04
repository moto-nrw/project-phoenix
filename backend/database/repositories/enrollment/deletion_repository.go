package enrollment

import (
	"context"
	"fmt"
	"sort"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/timetableprojection"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// DeletionRepository owns the deliberately cross-schema cleanup for one
// enrollment request. Keeping this SQL in one repository makes the service's
// transaction boundary auditable and prevents partial per-repository cleanup.
type DeletionRepository struct {
	db                    *bun.DB
	countAuditAdjustments func(context.Context, int64, *int64) (int, error)
	guardians             GuardianDirectory
}

func NewDeletionRepository(
	db *bun.DB,
	countAuditAdjustments func(context.Context, int64, *int64) (int, error),
) enrollmentModels.DeletionRepository {
	return &DeletionRepository{db: db, countAuditAdjustments: countAuditAdjustments}
}

// BindGuardianDirectory installs the People Directory the preserved guardian
// profiles and parent accounts of a request are resolved through (#2663).
func (r *DeletionRepository) BindGuardianDirectory(guardians GuardianDirectory) {
	r.guardians = guardians
}

type requestDeletionCountsRow struct {
	Requests                  int    `bun:"requests"`
	GuardianAccountID         *int64 `bun:"guardian_account_id"`
	RequestChildren           int    `bun:"request_children"`
	RequestChildOfferings     int    `bun:"request_child_offerings"`
	RequestGuardians          int    `bun:"request_guardians"`
	ChangeRequests            int    `bun:"change_requests"`
	ChangeRequestMessages     int    `bun:"change_request_messages"`
	LateInvites               int    `bun:"late_invites"`
	OfferingAdjustments       int    `bun:"offering_adjustments"`
	EmailOutbox               int    `bun:"email_outbox"`
	RolloverLinksCleared      int    `bun:"rollover_links_cleared"`
	StudentSourceLinksCleared int    `bun:"student_source_links_cleared"`
}

// guardianPreservation is what a request deletion leaves behind on the
// guardian side: the profiles and parent accounts that survive, and how
// many of them no child links to any more.
type guardianPreservation struct {
	profiles                int
	accounts                int
	unlinkedProfiles        int
	accountsWithoutStudents int
}

// previewGuardianPreservation resolves the guardian side of the preview
// through the People Directory (#2663): the candidate profiles are the
// request's co-guardian rows plus the profiles of the request's account,
// the candidate accounts are the request's account plus the accounts of
// those profiles. Everything is scoped to the tenant in context.
func (r *DeletionRepository) previewGuardianPreservation(ctx context.Context, requestID, tenantID int64, guardianAccountID *int64) (guardianPreservation, error) {
	if r.guardians == nil {
		return guardianPreservation{}, errGuardianDirectoryRequired
	}
	var requestProfileIDs []int64
	err := base.GetDB(ctx, r.db).NewRaw(`
		SELECT guardian_profile_id
		FROM enrollment.request_guardians
		WHERE request_id = ? AND tenant_id = ? AND guardian_profile_id IS NOT NULL
	`, requestID, tenantID).Scan(ctx, &requestProfileIDs)
	if err != nil {
		return guardianPreservation{}, fmt.Errorf("list request guardian profiles: %w", err)
	}
	profileIDs := make(map[int64]struct{}, len(requestProfileIDs))
	for _, id := range requestProfileIDs {
		profileIDs[id] = struct{}{}
	}
	accountIDs := make(map[int64]struct{})
	if guardianAccountID != nil {
		accountIDs[*guardianAccountID] = struct{}{}
		accountProfiles, err := r.guardians.ListGuardiansByAccount(ctx, []int64{*guardianAccountID})
		if err != nil {
			return guardianPreservation{}, err
		}
		for _, profile := range accountProfiles {
			profileIDs[profile.ID] = struct{}{}
		}
	}
	candidateProfiles := sortedIDs(profileIDs)
	profiles, err := r.guardians.ListGuardiansByID(ctx, candidateProfiles)
	if err != nil {
		return guardianPreservation{}, err
	}
	for _, profile := range profiles {
		if profile.AccountID != nil {
			accountIDs[*profile.AccountID] = struct{}{}
		}
	}
	candidateAccounts := sortedIDs(accountIDs)

	linkCounts, err := r.guardians.CountGuardianLinks(ctx, candidateProfiles)
	if err != nil {
		return guardianPreservation{}, err
	}
	result := guardianPreservation{profiles: len(candidateProfiles), accounts: len(candidateAccounts)}
	for _, id := range candidateProfiles {
		if linkCounts[id] == 0 {
			result.unlinkedProfiles++
		}
	}
	result.accountsWithoutStudents, err = r.countAccountsWithoutStudents(ctx, candidateAccounts)
	return result, err
}

// countAccountsWithoutStudents counts the accounts none of whose profiles
// still holds a child link. The profiles of an account may reach beyond the
// candidate profiles, so they are resolved from the account side.
func (r *DeletionRepository) countAccountsWithoutStudents(ctx context.Context, accountIDs []int64) (int, error) {
	accountProfiles, err := r.guardians.ListGuardiansByAccount(ctx, accountIDs)
	if err != nil {
		return 0, err
	}
	profileIDs := make([]int64, 0, len(accountProfiles))
	for _, profile := range accountProfiles {
		profileIDs = append(profileIDs, profile.ID)
	}
	linkCounts, err := r.guardians.CountGuardianLinks(ctx, profileIDs)
	if err != nil {
		return 0, err
	}
	linkedAccounts := make(map[int64]struct{})
	for _, profile := range accountProfiles {
		if profile.AccountID != nil && linkCounts[profile.ID] > 0 {
			linkedAccounts[*profile.AccountID] = struct{}{}
		}
	}
	count := 0
	for _, id := range accountIDs {
		if _, linked := linkedAccounts[id]; !linked {
			count++
		}
	}
	return count, nil
}

// sortedIDs returns the set's members in ascending order.
func sortedIDs(set map[int64]struct{}) []int64 {
	result := make([]int64, 0, len(set))
	for id := range set {
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func (r *DeletionRepository) PreviewRequest(ctx context.Context, requestID int64) (*enrollmentModels.DeletionImpact, error) {
	tenantID, err := deletionTenantID(ctx)
	if err != nil {
		return nil, err
	}
	row := new(requestDeletionCountsRow)
	err = base.GetDB(ctx, r.db).NewRaw(`
		WITH target_request AS (
			SELECT id, guardian_account_id
			FROM enrollment.requests
			WHERE id = ? AND tenant_id = ?
		), target_children AS (
			SELECT id
			FROM enrollment.request_children
			WHERE request_id = ? AND tenant_id = ?
		), target_changes AS (
			SELECT id
			FROM enrollment.change_requests
			WHERE request_id = ? AND tenant_id = ?
		)
		SELECT
			(SELECT COUNT(*) FROM target_request)::int AS requests,
			(SELECT guardian_account_id FROM target_request LIMIT 1) AS guardian_account_id,
			(SELECT COUNT(*) FROM target_children)::int AS request_children,
			(SELECT COUNT(*) FROM enrollment.request_child_offerings o JOIN target_children c ON c.id = o.request_child_id WHERE o.tenant_id = ?)::int AS request_child_offerings,
			(SELECT COUNT(*) FROM enrollment.request_guardians g WHERE g.request_id = ? AND g.tenant_id = ?)::int AS request_guardians,
			(SELECT COUNT(*) FROM target_changes)::int AS change_requests,
			(SELECT COUNT(*) FROM enrollment.change_request_messages m JOIN target_changes c ON c.id = m.change_request_id WHERE m.tenant_id = ?)::int AS change_request_messages,
			(SELECT COUNT(*) FROM enrollment.late_invites l WHERE l.used_request_id = ? AND l.tenant_id = ?)::int AS late_invites,
			0::int AS email_outbox,
			(SELECT COUNT(*) FROM enrollment.request_children c WHERE c.rollover_source_child_id IN (SELECT id FROM target_children) AND c.tenant_id = ?)::int AS rollover_links_cleared,
			0::int AS student_source_links_cleared
	`,
		requestID, tenantID,
		requestID, tenantID,
		requestID, tenantID,
		tenantID,
		requestID, tenantID,
		tenantID,
		requestID, tenantID,
		tenantID,
	).Scan(ctx, row)
	if err != nil {
		return nil, fmt.Errorf("preview enrollment request deletion: %w", err)
	}
	row.StudentSourceLinksCleared, err = timetableprojection.CountRequestSourceEnrollments(ctx, base.GetDB(ctx, r.db), tenantID, requestID)
	if err != nil {
		return nil, fmt.Errorf("preview enrollment request deletion: %w", err)
	}
	if r.countAuditAdjustments == nil {
		return nil, fmt.Errorf("preview enrollment request deletion: audit count capability is required")
	}
	row.OfferingAdjustments, err = r.countAuditAdjustments(ctx, requestID, nil)
	if err != nil {
		return nil, fmt.Errorf("preview enrollment request deletion: %w", err)
	}
	preserved, err := r.previewGuardianPreservation(ctx, requestID, tenantID, row.GuardianAccountID)
	if err != nil {
		return nil, fmt.Errorf("preview enrollment request deletion: %w", err)
	}
	impact := &enrollmentModels.DeletionImpact{
		RequestID:                     requestID,
		DeletesRequest:                true,
		Counts:                        deletionCountsFromRow(row),
		PreservedGuardianProfiles:     preserved.profiles,
		PreservedParentAccounts:       preserved.accounts,
		UnlinkedGuardianProfiles:      preserved.unlinkedProfiles,
		ParentAccountsWithoutStudents: preserved.accountsWithoutStudents,
	}
	if row.Requests == 0 {
		return impact, nil
	}
	impact.BlockingStudentIDs, err = r.listBlockingStudentIDs(ctx, requestID, nil, tenantID)
	if err != nil {
		return nil, err
	}
	return impact, nil
}

func (r *DeletionRepository) PreviewChild(ctx context.Context, requestID, childID int64) (*enrollmentModels.DeletionImpact, error) {
	tenantID, err := deletionTenantID(ctx)
	if err != nil {
		return nil, err
	}
	var meta struct {
		TargetChildren int `bun:"target_children"`
		AllChildren    int `bun:"all_children"`
	}
	err = base.GetDB(ctx, r.db).NewRaw(`
		SELECT
			COUNT(*) FILTER (WHERE id = ?)::int AS target_children,
			COUNT(*)::int AS all_children
		FROM enrollment.request_children
		WHERE request_id = ? AND tenant_id = ?
	`, childID, requestID, tenantID).Scan(ctx, &meta)
	if err != nil {
		return nil, fmt.Errorf("preview enrollment child deletion target: %w", err)
	}
	if meta.TargetChildren == 0 {
		return &enrollmentModels.DeletionImpact{RequestID: requestID, ChildID: &childID}, nil
	}
	if meta.AllChildren == 1 {
		impact, previewErr := r.PreviewRequest(ctx, requestID)
		if previewErr != nil {
			return nil, previewErr
		}
		impact.ChildID = &childID
		return impact, nil
	}

	var row struct {
		Offerings             int `bun:"offerings"`
		ChangeRequests        int `bun:"change_requests"`
		ChangeRequestMessages int `bun:"change_request_messages"`
		OfferingAdjustments   int `bun:"offering_adjustments"`
		RolloverLinks         int `bun:"rollover_links"`
		StudentSourceLinks    int `bun:"student_source_links"`
	}
	err = base.GetDB(ctx, r.db).NewRaw(`
		WITH target_changes AS (
			SELECT id FROM enrollment.change_requests
			WHERE request_id = ? AND request_child_id = ? AND tenant_id = ?
		)
		SELECT
			(SELECT COUNT(*) FROM enrollment.request_child_offerings WHERE request_child_id = ? AND tenant_id = ?)::int AS offerings,
			(SELECT COUNT(*) FROM target_changes)::int AS change_requests,
			(SELECT COUNT(*) FROM enrollment.change_request_messages m JOIN target_changes c ON c.id = m.change_request_id WHERE m.tenant_id = ?)::int AS change_request_messages,
			(SELECT COUNT(*) FROM enrollment.request_children WHERE rollover_source_child_id = ? AND tenant_id = ?)::int AS rollover_links,
			0::int AS student_source_links
	`, requestID, childID, tenantID, childID, tenantID, tenantID, childID, tenantID).Scan(ctx, &row)
	if err != nil {
		return nil, fmt.Errorf("preview enrollment child deletion: %w", err)
	}
	row.StudentSourceLinks, err = timetableprojection.CountChildSourceEnrollments(ctx, base.GetDB(ctx, r.db), tenantID, childID)
	if err != nil {
		return nil, fmt.Errorf("preview enrollment child deletion: %w", err)
	}
	if r.countAuditAdjustments == nil {
		return nil, fmt.Errorf("preview enrollment child deletion: audit count capability is required")
	}
	row.OfferingAdjustments, err = r.countAuditAdjustments(ctx, requestID, &childID)
	if err != nil {
		return nil, fmt.Errorf("preview enrollment child deletion: %w", err)
	}
	impact := &enrollmentModels.DeletionImpact{
		RequestID: requestID,
		ChildID:   &childID,
		Counts: enrollmentModels.DeletionCounts{
			RequestChildren:           1,
			RequestChildOfferings:     row.Offerings,
			ChangeRequests:            row.ChangeRequests,
			ChangeRequestMessages:     row.ChangeRequestMessages,
			OfferingAdjustments:       row.OfferingAdjustments,
			RolloverLinksCleared:      row.RolloverLinks,
			StudentSourceLinksCleared: row.StudentSourceLinks,
		},
	}
	impact.BlockingStudentIDs, err = r.listBlockingStudentIDs(ctx, requestID, &childID, tenantID)
	if err != nil {
		return nil, err
	}
	return impact, nil
}

func (r *DeletionRepository) listBlockingStudentIDs(ctx context.Context, requestID int64, childID *int64, tenantID int64) ([]int64, error) {
	ids := make([]int64, 0)
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`enrollment.request_children AS "request_child"`).
		ColumnExpr(`"request_child".created_student_id`).
		Where(`"request_child".request_id = ?`, requestID).
		Where(`"request_child".tenant_id = ?`, tenantID).
		Where(`"request_child".created_student_id IS NOT NULL`)
	if childID != nil {
		query = query.Where(`"request_child".id = ?`, *childID)
	}
	if err := query.OrderExpr(`"request_child".created_student_id`).Scan(ctx, &ids); err != nil {
		return nil, fmt.Errorf("list students blocking enrollment deletion: %w", err)
	}
	return ids, nil
}

func (r *DeletionRepository) DeleteChild(ctx context.Context, requestID, childID int64) error {
	tenantID, err := deletionTenantID(ctx)
	if err != nil {
		return err
	}
	db := base.GetDB(ctx, r.db)
	// audit.enrollment_offering_adjustments rows are NOT deleted here:
	// phoenix_tenant only holds SELECT/INSERT on that append-only table.
	// Its request_child_id FK is ON DELETE CASCADE, so deleting the
	// request_children row below removes them.
	statements := []struct {
		name string
		sql  string
		args []any
	}{
		{"change request messages", `DELETE FROM enrollment.change_request_messages WHERE tenant_id = ? AND change_request_id IN (SELECT id FROM enrollment.change_requests WHERE tenant_id = ? AND request_id = ? AND request_child_id = ?)`, []any{tenantID, tenantID, requestID, childID}},
		{"change requests", `DELETE FROM enrollment.change_requests WHERE tenant_id = ? AND request_id = ? AND request_child_id = ?`, []any{tenantID, requestID, childID}},
		{"child offerings", `DELETE FROM enrollment.request_child_offerings WHERE tenant_id = ? AND request_child_id = ?`, []any{tenantID, childID}},
		{"request child", `DELETE FROM enrollment.request_children WHERE tenant_id = ? AND request_id = ? AND id = ?`, []any{tenantID, requestID, childID}},
	}
	for _, statement := range statements {
		if _, execErr := db.NewRaw(statement.sql, statement.args...).Exec(ctx); execErr != nil {
			return fmt.Errorf("delete enrollment %s: %w", statement.name, execErr)
		}
	}
	return nil
}

func (r *DeletionRepository) DeleteRequest(ctx context.Context, requestID int64) error {
	tenantID, err := deletionTenantID(ctx)
	if err != nil {
		return err
	}
	db := base.GetDB(ctx, r.db)
	// audit.enrollment_offering_adjustments rows are NOT deleted here:
	// phoenix_tenant only holds SELECT/INSERT on that append-only table.
	// Its request_id / request_child_id FKs are ON DELETE CASCADE, so
	// deleting the request_children and requests rows below removes them.
	statements := []struct {
		name string
		sql  string
		args []any
	}{
		{"change request messages", `DELETE FROM enrollment.change_request_messages WHERE tenant_id = ? AND change_request_id IN (SELECT id FROM enrollment.change_requests WHERE tenant_id = ? AND request_id = ?)`, []any{tenantID, tenantID, requestID}},
		{"change requests", `DELETE FROM enrollment.change_requests WHERE tenant_id = ? AND request_id = ?`, []any{tenantID, requestID}},
		{"late invites", `DELETE FROM enrollment.late_invites WHERE tenant_id = ? AND used_request_id = ?`, []any{tenantID, requestID}},
		{"child offerings", `DELETE FROM enrollment.request_child_offerings WHERE tenant_id = ? AND request_child_id IN (SELECT id FROM enrollment.request_children WHERE tenant_id = ? AND request_id = ?)`, []any{tenantID, tenantID, requestID}},
		{"request guardians", `DELETE FROM enrollment.request_guardians WHERE tenant_id = ? AND request_id = ?`, []any{tenantID, requestID}},
		{"request children", `DELETE FROM enrollment.request_children WHERE tenant_id = ? AND request_id = ?`, []any{tenantID, requestID}},
		{"request", `DELETE FROM enrollment.requests WHERE tenant_id = ? AND id = ?`, []any{tenantID, requestID}},
	}
	for _, statement := range statements {
		if _, execErr := db.NewRaw(statement.sql, statement.args...).Exec(ctx); execErr != nil {
			return fmt.Errorf("delete enrollment %s: %w", statement.name, execErr)
		}
	}
	return nil
}

func deletionTenantID(ctx context.Context) (int64, error) {
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return 0, fmt.Errorf("tenant context is required for enrollment deletion")
	}
	return tenantID, nil
}

func deletionCountsFromRow(row *requestDeletionCountsRow) enrollmentModels.DeletionCounts {
	return enrollmentModels.DeletionCounts{
		Requests:                  row.Requests,
		RequestChildren:           row.RequestChildren,
		RequestChildOfferings:     row.RequestChildOfferings,
		RequestGuardians:          row.RequestGuardians,
		ChangeRequests:            row.ChangeRequests,
		ChangeRequestMessages:     row.ChangeRequestMessages,
		LateInvites:               row.LateInvites,
		OfferingAdjustments:       row.OfferingAdjustments,
		EmailOutbox:               row.EmailOutbox,
		RolloverLinksCleared:      row.RolloverLinksCleared,
		StudentSourceLinksCleared: row.StudentSourceLinksCleared,
	}
}
