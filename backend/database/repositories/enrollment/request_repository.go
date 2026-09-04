package enrollment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/uptrace/bun"
)

const requestTableExpr = `enrollment.requests AS "request"`

// existingStudentMatchLockClass salts the per-phase advisory lock used by
// AcquireExistingStudentMatchLock so it shares no key space with other advisory
// locks. It is the first arg of the two-int pg_advisory_xact_lock form ("enrl"
// in ASCII); the second arg is the phase id. The submission dedup lock keys its
// first arg on the phase id itself (small sequence values), so a fixed class id
// in the ~1.7e9 range never collides with it.
const existingStudentMatchLockClass int32 = 0x656e726c

type RequestRepository struct {
	*base.Repository[*enrollment.Request]
	db *bun.DB
}

func NewRequestRepository(db *bun.DB) enrollment.RequestRepository {
	repo := base.NewRepository[*enrollment.Request](db, "enrollment.requests", "Request")
	repo.TenantScoped = true
	return &RequestRepository{Repository: repo, db: db}
}

func (r *RequestRepository) Create(ctx context.Context, req *enrollment.Request) error {
	base.EnsureTenantID(ctx, req)

	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(req).
		ModelTableExpr(requestTableExpr).
		Returning("*").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create enrollment request: %w", err)
	}
	return nil
}

func (r *RequestRepository) FindByID(ctx context.Context, id int64) (*enrollment.Request, error) {
	return r.findByID(ctx, id, "")
}

func (r *RequestRepository) ListByIDs(ctx context.Context, ids []int64) ([]*enrollment.Request, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []*enrollment.Request
	if err := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(requestTableExpr).
		Where(`"request".id IN (?)`, bun.List(ids)).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("failed to list enrollment requests by ids: %w", err)
	}
	return rows, nil
}

func (r *RequestRepository) FindByIDForUpdate(ctx context.Context, id int64) (*enrollment.Request, error) {
	return r.findByID(ctx, id, "UPDATE")
}

func (r *RequestRepository) findByID(ctx context.Context, id int64, lockClause string) (*enrollment.Request, error) {
	req := new(enrollment.Request)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(req).
		ModelTableExpr(requestTableExpr).
		Where(`"request".id = ?`, id)
	if lockClause != "" {
		query = query.For(lockClause)
	}
	err := query.Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Keep sql.ErrNoRows in the chain so services can tell a
			// missing row from a query failure (same contract as the
			// form schema repository).
			return nil, fmt.Errorf("enrollment request %d not found: %w", id, sql.ErrNoRows)
		}
		return nil, fmt.Errorf("failed to find enrollment request: %w", err)
	}
	return req, nil
}

// ListAdmin returns every request for the tenant in context, newest
// first. Tenant-scoped via RLS. ChildStatus filter joins through the
// children table; an empty filter returns every row.
func (r *RequestRepository) ListAdmin(ctx context.Context, filters enrollment.RequestListFilters) ([]*enrollment.Request, error) {
	var rows []*enrollment.Request
	q := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(requestTableExpr)

	if filters.PhaseID > 0 {
		q = q.Where(`"request".phase_id = ?`, filters.PhaseID)
	}
	if filters.ChildStatus != "" {
		q = q.Where(`EXISTS (SELECT 1 FROM enrollment.request_children rc WHERE rc.request_id = "request".id AND rc.status = ?)`, filters.ChildStatus)
	}
	if filters.CreatedStudentID > 0 {
		q = q.Where(`EXISTS (SELECT 1 FROM enrollment.request_children rc WHERE rc.request_id = "request".id AND rc.created_student_id = ?)`, filters.CreatedStudentID)
	}
	if len(filters.CreatedStudentIDs) > 0 {
		q = q.Where(`EXISTS (SELECT 1 FROM enrollment.request_children rc WHERE rc.request_id = "request".id AND rc.created_student_id IN (?))`, bun.List(filters.CreatedStudentIDs))
	}
	q = q.OrderExpr(`"request".submitted_at DESC, "request".id DESC`)

	if err := q.Scan(ctx); err != nil {
		return nil, fmt.Errorf("failed to list admin requests: %w", err)
	}
	return rows, nil
}

// FindActiveDuplicate returns the subset of `children` that already
// have an active (non-rejected, non-withdrawn) enrollment in the given
// phase under the given guardian email. Comparison is
// LOWER(TRIM(...))-based on all three fields. The query joins
// requests → request_children and excludes child rows whose status is
// terminal-negative; an `approved` child still counts as a duplicate
// since the child is already enrolled. Tenant-scoped via RLS — caller
// must already be in a tenant-tx.
func (r *RequestRepository) FindActiveDuplicate(ctx context.Context, phaseID int64, guardianEmail string, children []enrollment.DuplicateChildKey) ([]enrollment.DuplicateChildKey, error) {
	return r.findActiveDuplicate(ctx, phaseID, guardianEmail, children, 0)
}

// FindActiveDuplicateExcludingRequest mirrors FindActiveDuplicate but ignores
// children belonging to the given request. Change-request approval uses this
// after locking the current request rows, where deleting/reinserting first
// would destroy the snapshot conflict guard.
func (r *RequestRepository) FindActiveDuplicateExcludingRequest(ctx context.Context, phaseID int64, guardianEmail string, children []enrollment.DuplicateChildKey, excludedRequestID int64) ([]enrollment.DuplicateChildKey, error) {
	return r.findActiveDuplicate(ctx, phaseID, guardianEmail, children, excludedRequestID)
}

func (r *RequestRepository) findActiveDuplicate(ctx context.Context, phaseID int64, guardianEmail string, children []enrollment.DuplicateChildKey, excludedRequestID int64) ([]enrollment.DuplicateChildKey, error) {
	if phaseID <= 0 {
		return nil, fmt.Errorf("phase id must be positive")
	}
	email := strings.ToLower(strings.TrimSpace(guardianEmail))
	if email == "" {
		return nil, nil
	}
	if len(children) == 0 {
		return nil, nil
	}

	type row struct {
		FirstName string `bun:"first_name"`
		LastName  string `bun:"last_name"`
	}
	var rows []row

	q := base.GetDB(ctx, r.db).NewSelect().
		ColumnExpr(`LOWER(TRIM(rc.first_name)) AS first_name`).
		ColumnExpr(`LOWER(TRIM(rc.last_name)) AS last_name`).
		TableExpr(`enrollment.request_children AS rc`).
		Join(`JOIN enrollment.requests AS req ON req.id = rc.request_id`).
		Where(`req.phase_id = ?`, phaseID).
		Where(`LOWER(TRIM(req.guardian_email)) = ?`, email).
		Where(`rc.status NOT IN (?, ?)`, enrollment.ChildStatusRejected, enrollment.ChildStatusWithdrawn)
	if excludedRequestID > 0 {
		q = q.Where(`req.id <> ?`, excludedRequestID)
	}

	conds := make([]string, 0, len(children))
	args := make([]any, 0, len(children)*2)
	for _, c := range children {
		fn := strings.ToLower(strings.TrimSpace(c.FirstName))
		ln := strings.ToLower(strings.TrimSpace(c.LastName))
		if fn == "" || ln == "" {
			continue
		}
		conds = append(conds, `(LOWER(TRIM(rc.first_name)) = ? AND LOWER(TRIM(rc.last_name)) = ?)`)
		args = append(args, fn, ln)
	}
	if len(conds) == 0 {
		return nil, nil
	}
	q = q.Where("("+strings.Join(conds, " OR ")+")", args...)

	if err := q.Distinct().Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("failed to check duplicate enrollments: %w", err)
	}

	out := make([]enrollment.DuplicateChildKey, 0, len(rows))
	for _, rr := range rows {
		out = append(out, enrollment.DuplicateChildKey{FirstName: rr.FirstName, LastName: rr.LastName})
	}
	return out, nil
}

// ExistsByPhaseID returns true if any request row references the given
// phase. Powers the phase-delete safety check — admin sees a 409 instead
// of cascading every parent submission into oblivion.
func (r *RequestRepository) ExistsByPhaseID(ctx context.Context, phaseID int64) (bool, error) {
	count, err := base.GetDB(ctx, r.db).NewSelect().
		Model((*enrollment.Request)(nil)).
		ModelTableExpr(requestTableExpr).
		Where(`"request".phase_id = ?`, phaseID).
		Count(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to check phase references in requests: %w", err)
	}
	return count > 0, nil
}

// CountByPhaseID returns how many request rows reference the given
// phase. Powers the phase-delete confirmation modal ("X Anmeldungen
// werden gelöscht"). Tenant-scoped via RLS.
func (r *RequestRepository) CountByPhaseID(ctx context.Context, phaseID int64) (int, error) {
	count, err := base.GetDB(ctx, r.db).NewSelect().
		Model((*enrollment.Request)(nil)).
		ModelTableExpr(requestTableExpr).
		Where(`"request".phase_id = ?`, phaseID).
		Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to count phase requests: %w", err)
	}
	return count, nil
}

// DeleteByPhaseID removes every request row for the phase. The FK
// cascade drops request_children and request_child_offerings with it;
// the request_child_offerings.care_offering_id ON DELETE RESTRICT means
// callers MUST delete requests before deleting the phase's care
// offerings (or the phase itself, which cascades the offerings).
// users.students rows are untouched — request_children.created_student_id
// is ON DELETE SET NULL, and the student is the parent in that
// relationship, so deleting children never deletes the student.
// Tenant-scoped via RLS.
func (r *RequestRepository) DeleteByPhaseID(ctx context.Context, phaseID int64) (int, error) {
	res, err := base.GetDB(ctx, r.db).NewDelete().
		Model((*enrollment.Request)(nil)).
		ModelTableExpr(requestTableExpr).
		Where(`"request".phase_id = ?`, phaseID).
		Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to delete phase requests: %w", err)
	}
	affected, _ := res.RowsAffected()
	return int(affected), nil
}

func (r *RequestRepository) ListFullyRejectedBefore(ctx context.Context, cutoff time.Time) ([]int64, error) {
	var ids []int64
	err := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`enrollment.requests AS "request"`).
		ColumnExpr(`"request".id`).
		Join(`JOIN enrollment.request_children AS rc ON rc.request_id = "request".id`).
		GroupExpr(`"request".id`).
		Having(`COUNT(rc.id) > 0`).
		Having(`BOOL_AND(rc.status = ?)`, enrollment.ChildStatusRejected).
		Having(`BOOL_AND(rc.reviewed_at IS NOT NULL AND rc.reviewed_at < ?)`, cutoff).
		OrderExpr(`"request".id`).
		Scan(ctx, &ids)
	if err != nil {
		return nil, fmt.Errorf("failed to list fully rejected enrollment requests: %w", err)
	}
	return ids, nil
}

func (r *RequestRepository) DeleteByID(ctx context.Context, requestID int64) error {
	result, err := base.GetDB(ctx, r.db).NewDelete().
		Model((*enrollment.Request)(nil)).
		ModelTableExpr(requestTableExpr).
		Where(`"request".id = ?`, requestID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete enrollment request: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return errors.New("enrollment request not found for cleanup")
	}
	return nil
}

// ExistsBySchemaID returns true if any request row references the
// schema version. Used to keep historical enrollment submissions
// auditable when admins delete unused form templates.
func (r *RequestRepository) ExistsBySchemaID(ctx context.Context, schemaID int64) (bool, error) {
	count, err := base.GetDB(ctx, r.db).NewSelect().
		Model((*enrollment.Request)(nil)).
		ModelTableExpr(requestTableExpr).
		Where(`"request".schema_id = ?`, schemaID).
		Count(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to check schema references in requests: %w", err)
	}
	return count > 0, nil
}

// FindByStatusToken looks up a request by its status_token. Used by the
// public status/edit page (PR 7). Public route — caller must wrap in
// WithAdminTx because the token is the only auth signal.
func (r *RequestRepository) FindByStatusToken(ctx context.Context, token string) (*enrollment.Request, error) {
	return r.findByStatusToken(ctx, token, "")
}

func (r *RequestRepository) FindByStatusTokenForUpdate(ctx context.Context, token string) (*enrollment.Request, error) {
	return r.findByStatusToken(ctx, token, "UPDATE")
}

// AcquireSubmissionDedupLock serializes concurrent writes for the same
// (phase, guardian email hash) pair inside the caller's transaction.
func (r *RequestRepository) AcquireSubmissionDedupLock(ctx context.Context, phaseID int64, emailHash uint64) error {
	_, err := base.GetDB(ctx, r.db).
		NewRaw(`SELECT pg_advisory_xact_lock(?, ?)`, int32(phaseID&0x7fffffff), int32(emailHash&0x7fffffff)).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire enrollment submission dedup lock: %w", err)
	}
	return nil
}

// AcquireExistingStudentMatchLock serializes existing-student re-enrollment
// matching for a phase across ALL guardians, held until the caller's
// transaction ends. The email-scoped AcquireSubmissionDedupLock does not cover
// this: two guardians with different emails submitting the same already-enrolled
// child take different email locks yet resolve the same matched_student_id, so
// without a phase-wide lock both could pass the matched-student duplicate check
// and pin the same live student — approving both then renews/overwrites one
// student twice and duplicates its care-offering enrollments (#1663).
func (r *RequestRepository) AcquireExistingStudentMatchLock(ctx context.Context, phaseID int64) error {
	if phaseID <= 0 || phaseID > math.MaxInt32 {
		return fmt.Errorf("AcquireExistingStudentMatchLock: phase_id %d out of advisory-lock range", phaseID)
	}
	_, err := base.GetDB(ctx, r.db).
		NewRaw(`SELECT pg_advisory_xact_lock(?, ?)`, existingStudentMatchLockClass, int32(phaseID)).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire existing-student match lock: %w", err)
	}
	return nil
}

// HasActiveRequestForMatchedStudent reports whether any non-rejected,
// non-withdrawn request_child in the phase is already pinned to the given
// already-enrolled student. Callers hold AcquireExistingStudentMatchLock so the
// check-then-insert stays race-free across guardians (#1663).
//
// excludeRequestChildID (0 = no exclusion) skips one persisted row. Submission
// and replacement edits insert fresh rows and pass 0; the change-request path
// re-checks a row that ALREADY exists and is already active, so it must exclude
// itself or it would always find its own pin.
func (r *RequestRepository) HasActiveRequestForMatchedStudent(ctx context.Context, phaseID, studentID, excludeRequestChildID int64) (bool, error) {
	if phaseID <= 0 || studentID <= 0 {
		return false, nil
	}
	q := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`enrollment.request_children AS rc`).
		Join(`JOIN enrollment.requests AS req ON req.id = rc.request_id`).
		Where(`req.phase_id = ?`, phaseID).
		Where(`rc.matched_student_id = ?`, studentID).
		Where(`rc.status NOT IN (?, ?)`, enrollment.ChildStatusRejected, enrollment.ChildStatusWithdrawn)
	if excludeRequestChildID > 0 {
		q = q.Where(`rc.id <> ?`, excludeRequestChildID)
	}
	exists, err := q.Exists(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to check active request for matched student: %w", err)
	}
	return exists, nil
}

// PinDecisionNotificationMode freezes the notification mode for a request.
// COALESCE makes the first successful pin win even when sibling decisions race;
// later calls return the existing value without overwriting it.
func (r *RequestRepository) PinDecisionNotificationMode(ctx context.Context, requestID int64, proposed string) (string, error) {
	switch proposed {
	case configModel.EnrollmentNotifyPerDecisionDigest, configModel.EnrollmentNotifyPerDecisionImmediate:
		// Valid persisted modes.
	default:
		return "", fmt.Errorf("invalid enrollment decision notification mode %q", proposed)
	}

	var mode string
	err := base.GetDB(ctx, r.db).NewRaw(`
		UPDATE enrollment.requests
		SET decision_notification_mode = COALESCE(decision_notification_mode, ?),
			updated_at = CASE
				WHEN decision_notification_mode IS NULL THEN NOW()
				ELSE updated_at
			END
		WHERE id = ?
		RETURNING decision_notification_mode
	`, proposed, requestID).Scan(ctx, &mode)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("enrollment request %d not found for notification mode pin", requestID)
		}
		return "", fmt.Errorf("failed to pin enrollment decision notification mode: %w", err)
	}
	return mode, nil
}

func (r *RequestRepository) findByStatusToken(ctx context.Context, token, lockClause string) (*enrollment.Request, error) {
	req := new(enrollment.Request)
	q := base.GetDB(ctx, r.db).NewSelect().
		Model(req).
		ModelTableExpr(requestTableExpr).
		Where(`"request".status_token = ?`, token)
	if lockClause != "" {
		q = q.For(lockClause)
	}
	err := q.Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("enrollment request with token not found")
		}
		return nil, fmt.Errorf("failed to find enrollment request by token: %w", err)
	}
	return req, nil
}

// UpdateGuardianData writes the guardian-editable fields of the request
// (names, phone, consent flags, custom answers) and bumps updated_at.
// Custom method (backend-conventions Rule 2): fixed multi-column projection
// for the parent-portal edit flow.
func (r *RequestRepository) UpdateGuardianData(ctx context.Context, req *enrollment.Request) error {
	return r.updateGuardianData(ctx, req, false)
}

func (r *RequestRepository) UpdateGuardianDataWithEmail(ctx context.Context, req *enrollment.Request) error {
	return r.updateGuardianData(ctx, req, true)
}

func (r *RequestRepository) updateGuardianData(ctx context.Context, req *enrollment.Request, includeEmail bool) error {
	q := base.GetDB(ctx, r.db).NewUpdate().
		Model(req).
		ModelTableExpr(requestTableExpr).
		Set("guardian_first_name = ?", req.GuardianFirstName).
		Set("guardian_last_name = ?", req.GuardianLastName).
		Set("guardian_phone = ?", req.GuardianPhone).
		Set("consent_flags = ?", req.ConsentFlags).
		Set("legal_blocks_snapshot = ?", req.LegalBlocksSnapshot).
		Set("custom_data = ?", req.CustomData).
		Set("updated_at = NOW()").
		Where(`"request".id = ?`, req.ID)
	if includeEmail {
		q = q.Set("guardian_email = ?", req.GuardianEmail)
	}
	_, err := q.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update guardian data: %w", err)
	}
	return nil
}

// MarkWithdrawn stamps withdrawn_at on the request and bumps updated_at.
// Used by the parent-portal withdraw flow once every child is withdrawn.
func (r *RequestRepository) MarkWithdrawn(ctx context.Context, requestID int64, withdrawnAt time.Time) error {
	_, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*enrollment.Request)(nil)).
		ModelTableExpr(requestTableExpr).
		Set("withdrawn_at = ?", withdrawnAt).
		Set("updated_at = NOW()").
		Where(`"request".id = ?`, requestID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to mark request withdrawn: %w", err)
	}
	return nil
}

// ClearWithdrawn is the inverse of MarkWithdrawn: it nulls withdrawn_at and
// bumps updated_at. Used by the admin restore flow (#2157) after every
// withdrawn child of the request was flipped back to submitted.
func (r *RequestRepository) ClearWithdrawn(ctx context.Context, requestID int64) error {
	_, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*enrollment.Request)(nil)).
		ModelTableExpr(requestTableExpr).
		Set("withdrawn_at = NULL").
		Set("updated_at = NOW()").
		Where(`"request".id = ?`, requestID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to clear request withdrawn state: %w", err)
	}
	return nil
}
