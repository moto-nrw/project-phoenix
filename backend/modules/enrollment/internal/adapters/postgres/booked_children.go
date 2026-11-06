package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
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

const approvedChildStudentsQuery = `SELECT tenant_id, id, COALESCE(created_student_id, matched_student_id) AS student_id
	FROM enrollment.request_children
	WHERE tenant_id = ANY(?0) AND status = ?1
		AND COALESCE(created_student_id, matched_student_id) IS NOT NULL`

// ApprovedChildStudentIDsByTenant maps each approved request child to its
// student.
func (BookedChildStore) ApprovedChildStudentIDsByTenant(ctx context.Context, schools []int64) (map[int64]map[int64]int64, error) {
	result := make(map[int64]map[int64]int64, len(schools))
	if len(schools) == 0 {
		return result, nil
	}
	if !tenant.IsAdminTx(ctx) {
		return nil, errors.New("enrollment postgres: booked children need the administrative transaction")
	}
	db, err := NewDatabase()(ctx)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		TenantID  int64 `bun:"tenant_id"`
		ID        int64 `bun:"id"`
		StudentID int64 `bun:"student_id"`
	}
	if err := db.NewRaw(approvedChildStudentsQuery, pgdialect.Array(schools), enrollment.ChildStatusApproved).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("enrollment postgres: booked children: %w", err)
	}
	for _, row := range rows {
		if result[row.TenantID] == nil {
			result[row.TenantID] = map[int64]int64{}
		}
		result[row.TenantID][row.ID] = row.StudentID
	}
	return result, nil
}
