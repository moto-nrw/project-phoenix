package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/uptrace/bun"
)

func (r *Store) AccountRequests(ctx context.Context, accountID int64, accountEmail string) ([]enrollment.AccountRequest, error) {
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	type row struct {
		RequestID                int64           `bun:"request_id"`
		TenantID                 int64           `bun:"tenant_id"`
		StatusToken              string          `bun:"status_token"`
		SubmittedAt              time.Time       `bun:"submitted_at"`
		WithdrawnAt              *time.Time      `bun:"withdrawn_at"`
		PhaseID                  int64           `bun:"phase_id"`
		PhaseName                string          `bun:"phase_name"`
		ServiceStartDate         enrollment.Date `bun:"service_start_date"`
		ServiceEndDate           enrollment.Date `bun:"service_end_date"`
		ShowStatusReasonToParent bool            `bun:"show_status_reason_to_parent"`
	}

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
		JOIN enrollment.phases AS ph ON ph.id = req.phase_id AND ph.tenant_id = req.tenant_id
		WHERE (
		    req.guardian_account_id = ?
		    OR (
		      req.guardian_account_id IS NULL
		      AND ? <> ''
		      AND LOWER(TRIM(req.guardian_email)) = ?
		    )
		  )
		ORDER BY req.submitted_at DESC, req.id DESC
	`

	var rows []row
	if err := db.NewRaw(requestQuery, accountID, accountEmail, accountEmail).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("parent: list enrollment requests: %w", err)
	}
	if len(rows) == 0 {
		return []enrollment.AccountRequest{}, nil
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
	var children []childRow
	if err := db.NewRaw(childQuery, bun.List(requestIDs)).Scan(ctx, &children); err != nil {
		return nil, fmt.Errorf("parent: list enrollment request children: %w", err)
	}
	childrenByRequest := make(map[int64][]enrollment.AccountRequestChild, len(rows))
	for _, c := range children {
		childrenByRequest[c.RequestID] = append(childrenByRequest[c.RequestID], enrollment.AccountRequestChild{ChildID: c.ChildID, FirstName: c.FirstName, LastName: c.LastName, Status: c.Status, StatusReason: c.StatusReason, CreatedStudentID: c.CreatedStudentID})
	}
	result := make([]enrollment.AccountRequest, 0, len(rows))
	for _, rr := range rows {
		result = append(result, enrollment.AccountRequest{RequestID: rr.RequestID, TenantID: rr.TenantID, StatusToken: rr.StatusToken, SubmittedAt: rr.SubmittedAt, WithdrawnAt: rr.WithdrawnAt, PhaseID: rr.PhaseID, PhaseName: rr.PhaseName, ServiceStartDate: rr.ServiceStartDate, ServiceEndDate: rr.ServiceEndDate, ShowStatusReasonToParent: rr.ShowStatusReasonToParent, Children: childrenByRequest[rr.RequestID]})
	}
	return result, nil
}
