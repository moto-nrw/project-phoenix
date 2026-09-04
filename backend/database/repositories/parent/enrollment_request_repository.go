package parent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/uptrace/bun"

	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// EnrollmentRequestRepository implements
// parentModels.EnrollmentRequestRepository.
type EnrollmentRequestRepository struct {
	runtime   Runtime
	guardians GuardianDirectory
}

// NewEnrollmentRequestRepository wires a fresh repository.
func NewEnrollmentRequestRepository(runtime Runtime) parentModels.EnrollmentRequestRepository {
	return &EnrollmentRequestRepository{runtime: requireRuntime(runtime)}
}

// BindGuardianDirectory installs the People Directory the account's guardian
// links are read from (#2663).
func (r *EnrollmentRequestRepository) BindGuardianDirectory(guardians GuardianDirectory) {
	r.guardians = guardians
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
		RequestID                int64        `bun:"request_id"`
		TenantID                 int64        `bun:"tenant_id"`
		StatusToken              string       `bun:"status_token"`
		SubmittedAt              time.Time    `bun:"submitted_at"`
		WithdrawnAt              *time.Time   `bun:"withdrawn_at"`
		PhaseID                  int64        `bun:"phase_id"`
		PhaseName                string       `bun:"phase_name"`
		ServiceStartDate         calendarDate `bun:"service_start_date"`
		ServiceEndDate           calendarDate `bun:"service_end_date"`
		ShowStatusReasonToParent bool         `bun:"show_status_reason_to_parent"`
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
		ORDER BY req.submitted_at DESC, req.id DESC
	`

	var rows []row
	if err := runtimeDB(ctx, r.runtime).NewRaw(
		requestQuery,
		accountID,
		accountID,
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
		RequestID        int64   `bun:"request_id"`
		ChildID          int64   `bun:"child_id"`
		FirstName        string  `bun:"first_name"`
		LastName         string  `bun:"last_name"`
		Status           string  `bun:"status"`
		StatusReason     *string `bun:"status_reason"`
		SortOrder        int     `bun:"sort_order"`
		CreatedStudentID *int64  `bun:"created_student_id"`
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
			rc.sort_order    AS sort_order,
			rc.created_student_id AS created_student_id
		FROM enrollment.request_children AS rc
		WHERE rc.request_id IN (?)
		ORDER BY rc.request_id, rc.sort_order, rc.id
	`
	if err := runtimeDB(ctx, r.runtime).NewRaw(childQuery, bun.List(requestIDs)).Scan(ctx, &children); err != nil {
		return nil, fmt.Errorf("parent: list enrollment request children: %w", err)
	}

	// A request whose approved child became a student is only listed while
	// the account still holds parent_portal.enrollments.view on EVERY such
	// child at the request's school. The links belong to the People
	// Directory (#2663) and are read inside the same admin transaction.
	links, err := guardianLinksByAccount(ctx, r.guardians, accountID)
	if err != nil {
		return nil, fmt.Errorf("parent: list enrollment requests: %w", err)
	}
	viewable := make(map[[2]int64]struct{}, len(links))
	for _, link := range links {
		if link.HasPermission(careplan.GuardianPermissionEnrollmentsView) {
			viewable[[2]int64{link.TenantID, link.StudentID}] = struct{}{}
		}
	}
	tenantByRequest := make(map[int64]int64, len(rows))
	for _, rr := range rows {
		tenantByRequest[rr.RequestID] = rr.TenantID
	}
	hidden := make(map[int64]struct{})
	childrenByRequest := make(map[int64][]parentModels.EnrollmentRequestChildSummary, len(rows))
	for _, c := range children {
		if c.CreatedStudentID != nil {
			if _, ok := viewable[[2]int64{tenantByRequest[c.RequestID], *c.CreatedStudentID}]; !ok {
				hidden[c.RequestID] = struct{}{}
			}
		}
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
		if _, skip := hidden[rr.RequestID]; skip {
			continue
		}
		out = append(out, &parentModels.EnrollmentRequestSummary{
			RequestID:                rr.RequestID,
			TenantID:                 rr.TenantID,
			StatusToken:              rr.StatusToken,
			SubmittedAt:              rr.SubmittedAt,
			WithdrawnAt:              rr.WithdrawnAt,
			PhaseID:                  rr.PhaseID,
			PhaseName:                rr.PhaseName,
			ServiceStartDate:         careplan.Date(rr.ServiceStartDate),
			ServiceEndDate:           careplan.Date(rr.ServiceEndDate),
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

	res, err := runtimeDB(ctx, r.runtime).NewRaw(`
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
