package parent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
)

// EnrollmentRequestRepository implements
// parentModels.EnrollmentRequestRepository.
type EnrollmentRequestRepository struct {
	db *bun.DB
}

// NewEnrollmentRequestRepository wires a fresh repository.
func NewEnrollmentRequestRepository(db *bun.DB) parentModels.EnrollmentRequestRepository {
	return &EnrollmentRequestRepository{db: db}
}

// ListByAccount returns every enrollment.requests row owned by the
// given account, joined to phase + school + child rows. Two queries:
// one for the request envelope (with phase + school joined in), one
// for the child rows scoped to those requests. We bundle them here so
// the parent dashboard can render the whole list with a single fetch.
//
// Cross-tenant — caller must wrap in tenant.WithAdminTx.
func (r *EnrollmentRequestRepository) ListByAccount(ctx context.Context, accountID int64) ([]*parentModels.EnrollmentRequestSummary, error) {
	if accountID <= 0 {
		return nil, fmt.Errorf("parent: account_id must be positive")
	}

	type row struct {
		RequestID                int64         `bun:"request_id"`
		TenantID                 int64         `bun:"tenant_id"`
		StatusToken              string        `bun:"status_token"`
		SubmittedAt              time.Time     `bun:"submitted_at"`
		WithdrawnAt              *time.Time    `bun:"withdrawn_at"`
		PhaseID                  int64         `bun:"phase_id"`
		PhaseName                string        `bun:"phase_name"`
		ServiceStartDate         timezone.Date `bun:"service_start_date"`
		ServiceEndDate           timezone.Date `bun:"service_end_date"`
		ShowStatusReasonToParent bool          `bun:"show_status_reason_to_parent"`
	}

	// Two-pronged match:
	//   - primary: req.guardian_account_id = ? (set on parent-auth
	//     submits, or backfilled by the guardian-invite-accept flow)
	//   - fallback: req.guardian_account_id IS NULL AND the request's
	//     guardian_email matches the account's email (case- and
	//     trim-insensitive)
	// The fallback covers the edge case where the invite-accept
	// backfill failed silently (e.g. a transient FK race) — without
	// it, "Meine Anmeldungen" silently disappears for parents whose
	// submissions never got stamped.
	const requestQuery = `
		SELECT
			req.id              AS request_id,
			req.tenant_id       AS tenant_id,
			req.status_token    AS status_token,
			req.submitted_at    AS submitted_at,
			req.withdrawn_at    AS withdrawn_at,
			ph.id               AS phase_id,
			ph.name             AS phase_name,
			ph.service_start_date AS service_start_date,
			ph.service_end_date   AS service_end_date,
			ph.show_status_reason_to_parent AS show_status_reason_to_parent
		FROM enrollment.requests AS req
		JOIN enrollment.phases AS ph ON ph.id = req.phase_id
		LEFT JOIN auth.accounts AS acc ON acc.id = ?
		WHERE (
		    req.guardian_account_id = ?
		    OR (
		      req.guardian_account_id IS NULL
		      AND acc.email IS NOT NULL
		      AND LOWER(TRIM(req.guardian_email)) = LOWER(TRIM(acc.email))
		    )
		  )
		  AND NOT EXISTS (
		    SELECT 1
		    FROM enrollment.request_children AS rc_created
		    WHERE rc_created.request_id = req.id
		      AND rc_created.created_student_id IS NOT NULL
		      AND NOT EXISTS (
		        SELECT 1
		        FROM users.students_guardians AS sg
		        JOIN users.guardian_profiles AS gp
		          ON gp.id = sg.guardian_profile_id
		         AND gp.tenant_id = sg.tenant_id
		        WHERE sg.student_id = rc_created.created_student_id
		          AND sg.tenant_id = req.tenant_id
		          AND gp.account_id = ?
		          AND COALESCE((sg.permissions ->> ?)::boolean, false) = TRUE
		      )
		  )
		ORDER BY req.submitted_at DESC, req.id DESC
	`

	var rows []row
	if err := base.GetDB(ctx, r.db).NewRaw(
		requestQuery,
		accountID,
		accountID,
		accountID,
		authorize.GuardianPermissionEnrollmentsView,
	).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("parent: list enrollment requests: %w", err)
	}
	if len(rows) == 0 {
		return []*parentModels.EnrollmentRequestSummary{}, nil
	}

	requestIDs := make([]int64, 0, len(rows))
	for _, rr := range rows {
		requestIDs = append(requestIDs, rr.RequestID)
	}

	type childRow struct {
		RequestID    int64   `bun:"request_id"`
		ChildID      int64   `bun:"child_id"`
		FirstName    string  `bun:"first_name"`
		LastName     string  `bun:"last_name"`
		Status       string  `bun:"status"`
		StatusReason *string `bun:"status_reason"`
		SortOrder    int     `bun:"sort_order"`
	}

	var children []childRow
	const childQuery = `
		SELECT
			rc.request_id    AS request_id,
			rc.id            AS child_id,
			rc.first_name    AS first_name,
			rc.last_name     AS last_name,
			rc.status        AS status,
			rc.status_reason AS status_reason,
			rc.sort_order    AS sort_order
		FROM enrollment.request_children AS rc
		WHERE rc.request_id IN (?)
		ORDER BY rc.request_id, rc.sort_order, rc.id
	`
	if err := base.GetDB(ctx, r.db).NewRaw(childQuery, bun.List(requestIDs)).Scan(ctx, &children); err != nil {
		return nil, fmt.Errorf("parent: list enrollment request children: %w", err)
	}

	childrenByRequest := make(map[int64][]parentModels.EnrollmentRequestChildSummary, len(rows))
	for _, c := range children {
		childrenByRequest[c.RequestID] = append(childrenByRequest[c.RequestID], parentModels.EnrollmentRequestChildSummary{
			ChildID:      c.ChildID,
			FirstName:    c.FirstName,
			LastName:     c.LastName,
			Status:       c.Status,
			StatusReason: c.StatusReason,
		})
	}

	out := make([]*parentModels.EnrollmentRequestSummary, 0, len(rows))
	for _, rr := range rows {
		out = append(out, &parentModels.EnrollmentRequestSummary{
			RequestID:                rr.RequestID,
			TenantID:                 rr.TenantID,
			StatusToken:              rr.StatusToken,
			SubmittedAt:              rr.SubmittedAt,
			WithdrawnAt:              rr.WithdrawnAt,
			PhaseID:                  rr.PhaseID,
			PhaseName:                rr.PhaseName,
			ServiceStartDate:         rr.ServiceStartDate,
			ServiceEndDate:           rr.ServiceEndDate,
			ShowStatusReasonToParent: rr.ShowStatusReasonToParent,
			Children:                 childrenByRequest[rr.RequestID],
		})
	}
	return out, nil
}

// BackfillGuardianAccountID claims every guardian-less enrollment
// request that matches the given email (case-insensitive) for the
// given account. Used by the guardian-invite-accept flow so requests
// submitted before the parent had an account show up in /me/enrollments
// after acceptance. Cross-tenant — caller wraps in WithAdminTx.
func (r *EnrollmentRequestRepository) BackfillGuardianAccountID(ctx context.Context, accountID int64, email string) (int, error) {
	if accountID <= 0 {
		return 0, fmt.Errorf("parent: account_id must be positive")
	}
	emailLC := strings.ToLower(strings.TrimSpace(email))
	if emailLC == "" {
		return 0, nil
	}

	res, err := base.GetDB(ctx, r.db).NewRaw(`
		UPDATE enrollment.requests
		SET guardian_account_id = ?
		WHERE guardian_account_id IS NULL
		  AND LOWER(TRIM(guardian_email)) = ?
	`, accountID, emailLC).Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("parent: backfill guardian_account_id: %w", err)
	}
	rows, _ := res.RowsAffected()
	return int(rows), nil
}
