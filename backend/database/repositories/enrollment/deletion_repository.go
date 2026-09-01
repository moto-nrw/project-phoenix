package enrollment

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// DeletionRepository owns the deliberately cross-schema cleanup for one
// enrollment request. Keeping this SQL in one repository makes the service's
// transaction boundary auditable and prevents partial per-repository cleanup.
type DeletionRepository struct {
	db                    *bun.DB
	countAuditAdjustments func(context.Context, int64, *int64) (int, error)
}

func NewDeletionRepository(
	db *bun.DB,
	countAuditAdjustments func(context.Context, int64, *int64) (int, error),
) enrollmentModels.DeletionRepository {
	return &DeletionRepository{db: db, countAuditAdjustments: countAuditAdjustments}
}

type requestDeletionCountsRow struct {
	Requests                  int `bun:"requests"`
	RequestChildren           int `bun:"request_children"`
	RequestChildOfferings     int `bun:"request_child_offerings"`
	RequestGuardians          int `bun:"request_guardians"`
	ChangeRequests            int `bun:"change_requests"`
	ChangeRequestMessages     int `bun:"change_request_messages"`
	LateInvites               int `bun:"late_invites"`
	OfferingAdjustments       int `bun:"offering_adjustments"`
	EmailOutbox               int `bun:"email_outbox"`
	RolloverLinksCleared      int `bun:"rollover_links_cleared"`
	StudentSourceLinksCleared int `bun:"student_source_links_cleared"`
	PreservedProfiles         int `bun:"preserved_profiles"`
	PreservedAccounts         int `bun:"preserved_accounts"`
	UnlinkedProfiles          int `bun:"unlinked_profiles"`
	AccountsWithoutStudents   int `bun:"accounts_without_students"`
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
		), candidate_profiles AS (
			SELECT guardian_profile_id AS id
			FROM enrollment.request_guardians
			WHERE request_id = ? AND tenant_id = ? AND guardian_profile_id IS NOT NULL
			UNION
			SELECT gp.id
			FROM users.guardian_profiles gp
			JOIN target_request tr ON tr.guardian_account_id = gp.account_id
			WHERE gp.tenant_id = ?
		), candidate_accounts AS (
			SELECT guardian_account_id AS id
			FROM target_request
			WHERE guardian_account_id IS NOT NULL
			UNION
			SELECT gp.account_id
			FROM users.guardian_profiles gp
			JOIN candidate_profiles cp ON cp.id = gp.id
			WHERE gp.account_id IS NOT NULL AND gp.tenant_id = ?
		)
		SELECT
			(SELECT COUNT(*) FROM target_request)::int AS requests,
			(SELECT COUNT(*) FROM target_children)::int AS request_children,
			(SELECT COUNT(*) FROM enrollment.request_child_offerings o JOIN target_children c ON c.id = o.request_child_id WHERE o.tenant_id = ?)::int AS request_child_offerings,
			(SELECT COUNT(*) FROM enrollment.request_guardians g WHERE g.request_id = ? AND g.tenant_id = ?)::int AS request_guardians,
			(SELECT COUNT(*) FROM target_changes)::int AS change_requests,
			(SELECT COUNT(*) FROM enrollment.change_request_messages m JOIN target_changes c ON c.id = m.change_request_id WHERE m.tenant_id = ?)::int AS change_request_messages,
			(SELECT COUNT(*) FROM enrollment.late_invites l WHERE l.used_request_id = ? AND l.tenant_id = ?)::int AS late_invites,
			0::int AS email_outbox,
			(SELECT COUNT(*) FROM enrollment.request_children c WHERE c.rollover_source_child_id IN (SELECT id FROM target_children) AND c.tenant_id = ?)::int AS rollover_links_cleared,
			(SELECT COUNT(*) FROM activities.student_enrollments se WHERE se.enrollment_request_child_id IN (SELECT id FROM target_children) AND se.tenant_id = ?)::int AS student_source_links_cleared,
			(SELECT COUNT(*) FROM candidate_profiles)::int AS preserved_profiles,
			(SELECT COUNT(*) FROM candidate_accounts)::int AS preserved_accounts,
			(SELECT COUNT(*) FROM candidate_profiles cp WHERE NOT EXISTS (SELECT 1 FROM users.students_guardians sg WHERE sg.guardian_profile_id = cp.id AND sg.tenant_id = ?))::int AS unlinked_profiles,
			(SELECT COUNT(*) FROM candidate_accounts ca WHERE NOT EXISTS (
				SELECT 1 FROM users.guardian_profiles gp
				JOIN users.students_guardians sg ON sg.guardian_profile_id = gp.id AND sg.tenant_id = gp.tenant_id
				WHERE gp.account_id = ca.id AND gp.tenant_id = ?
			))::int AS accounts_without_students
	`,
		requestID, tenantID,
		requestID, tenantID,
		requestID, tenantID,
		requestID, tenantID, tenantID, tenantID,
		tenantID,
		requestID, tenantID,
		tenantID,
		requestID, tenantID,
		tenantID, tenantID, tenantID, tenantID,
	).Scan(ctx, row)
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
	impact := &enrollmentModels.DeletionImpact{
		RequestID:                     requestID,
		DeletesRequest:                true,
		Counts:                        deletionCountsFromRow(row),
		PreservedGuardianProfiles:     row.PreservedProfiles,
		PreservedParentAccounts:       row.PreservedAccounts,
		UnlinkedGuardianProfiles:      row.UnlinkedProfiles,
		ParentAccountsWithoutStudents: row.AccountsWithoutStudents,
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
			(SELECT COUNT(*) FROM activities.student_enrollments WHERE enrollment_request_child_id = ? AND tenant_id = ?)::int AS student_source_links
	`, requestID, childID, tenantID, childID, tenantID, tenantID, childID, tenantID, childID, tenantID).Scan(ctx, &row)
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
