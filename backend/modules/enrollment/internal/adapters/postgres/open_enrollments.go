package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/tenant"
)

// OpenEnrollmentStore counts the children of the online enrollment that wait
// for a decision, for the Träger dashboard (#4675). It reads in the Träger
// projection's administrative transaction, which reads across schools; the
// statement filters on the given schools.
type OpenEnrollmentStore struct{}

var _ enrollment.OpenEnrollments = OpenEnrollmentStore{}

// NewOpenEnrollmentStore builds the store.
func NewOpenEnrollmentStore() OpenEnrollmentStore { return OpenEnrollmentStore{} }

const openEnrollmentChildrenQuery = `SELECT request_child.tenant_id, COUNT(*) AS open
	FROM enrollment.request_children AS request_child
	JOIN enrollment.requests AS request
		ON request.id = request_child.request_id AND request.tenant_id = request_child.tenant_id
	WHERE request_child.tenant_id = ANY(?0) AND request_child.status IN (?1)
		AND request.withdrawn_at IS NULL
	GROUP BY request_child.tenant_id`

type openEnrollmentRow struct {
	TenantID int64 `bun:"tenant_id"`
	Open     int   `bun:"open"`
}

// OpenEnrollmentChildrenByTenant counts the waiting children of each school.
func (OpenEnrollmentStore) OpenEnrollmentChildrenByTenant(ctx context.Context, schools []int64) (map[int64]int, error) {
	result := map[int64]int{}
	if len(schools) == 0 {
		return result, nil
	}
	if !tenant.IsAdminTx(ctx) {
		return nil, errors.New("enrollment postgres: open enrollments need the administrative transaction")
	}
	db, err := NewDatabase()(ctx)
	if err != nil {
		return nil, err
	}
	var rows []openEnrollmentRow
	if err := db.NewRaw(openEnrollmentChildrenQuery, pgdialect.Array(schools), bun.List(enrollment.OpenEnrollmentChildStatuses())).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("enrollment postgres: open enrollments: %w", err)
	}
	for _, row := range rows {
		result[row.TenantID] = row.Open
	}
	return result, nil
}
