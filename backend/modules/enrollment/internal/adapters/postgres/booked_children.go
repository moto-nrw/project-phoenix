package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// BookedChildStore ties the approved children of the online enrollment to
// their student, for the Träger's occupancy (#4676). It reads in the Träger
// projection's administrative transaction, which reads across schools; the
// statement filters on the given schools.
type BookedChildStore struct{}

var _ enrollment.BookedChildren = BookedChildStore{}

// NewBookedChildStore builds the store.
func NewBookedChildStore() BookedChildStore { return BookedChildStore{} }

// bookedChildrenAndRunningPhasesQuery reads both answers in one statement:
// kind 'child' maps an approved request child to its student, kind 'phase'
// names a running phase. Takes the schools, the approved status and the day.
const bookedChildrenAndRunningPhasesQuery = `SELECT 'child' AS kind, tenant_id, id, COALESCE(created_student_id, matched_student_id) AS student_id
	FROM enrollment.request_children
	WHERE tenant_id = ANY(?0) AND status = ?1
		AND COALESCE(created_student_id, matched_student_id) IS NOT NULL
	UNION ALL
	SELECT 'phase', tenant_id, id, NULL
	FROM enrollment.phases
	WHERE tenant_id = ANY(?0) AND is_active
		AND service_start_date <= ?2::date AND service_end_date >= ?2::date`

// BookedChildrenAndRunningPhases maps each approved request child to its
// student and lists the running phases.
func (BookedChildStore) BookedChildrenAndRunningPhases(ctx context.Context, schools []int64, day calendar.Date) (map[int64]map[int64]int64, []int64, error) {
	children := make(map[int64]map[int64]int64, len(schools))
	if len(schools) == 0 {
		return children, nil, nil
	}
	if !tenant.IsAdminTx(ctx) {
		return nil, nil, errors.New("enrollment postgres: booked children need the administrative transaction")
	}
	db, err := NewDatabase()(ctx)
	if err != nil {
		return nil, nil, err
	}
	var rows []struct {
		Kind      string `bun:"kind"`
		TenantID  int64  `bun:"tenant_id"`
		ID        int64  `bun:"id"`
		StudentID *int64 `bun:"student_id"`
	}
	if err := db.NewRaw(bookedChildrenAndRunningPhasesQuery, pgdialect.Array(schools), enrollment.ChildStatusApproved, day).Scan(ctx, &rows); err != nil {
		return nil, nil, fmt.Errorf("enrollment postgres: booked children: %w", err)
	}
	var phases []int64
	for _, row := range rows {
		if row.Kind == "phase" {
			phases = append(phases, row.ID)
			continue
		}
		if row.StudentID == nil {
			continue
		}
		if children[row.TenantID] == nil {
			children[row.TenantID] = map[int64]int64{}
		}
		children[row.TenantID][row.ID] = *row.StudentID
	}
	return children, phases, nil
}
