// Package application answers the Träger-Portal reads of the Träger
// projection (ADR 0084 section 1): it resolves the schools of the Träger at
// Organisation & Tenancy and reads them in one administrative transaction.
package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/traegerview"
)

// SchoolRows reads the rows of a school set; the Postgres adapter serves it.
type SchoolRows interface {
	Schools(ctx context.Context, schools []int64) ([]traegerview.School, error)
}

// Overview answers the reads of one Träger.
type Overview struct {
	schoolIDs   traegerview.SchoolIDs
	rows        SchoolRows
	withinAdmin func(context.Context, func(context.Context) error) error
}

var _ traegerview.Overview = (*Overview)(nil)

// New builds the overview. withinAdmin opens the administrative transaction
// the projection reads in; it is the Träger-Portal's only one (ADR 0084).
func New(schoolIDs traegerview.SchoolIDs, rows SchoolRows, withinAdmin func(context.Context, func(context.Context) error) error) *Overview {
	if schoolIDs == nil || rows == nil || withinAdmin == nil {
		panic("traeger view: school IDs, rows and the administrative transaction are required")
	}
	return &Overview{schoolIDs: schoolIDs, rows: rows, withinAdmin: withinAdmin}
}

// Schools lists the schools of the Träger. A Träger that is deactivated or
// deleted reaches no school.
func (o *Overview) Schools(ctx context.Context, organizationID int64) ([]traegerview.School, error) {
	if organizationID <= 0 {
		return nil, traegerview.ErrTraegerRequired
	}
	var schools []traegerview.School
	err := o.withinAdmin(ctx, func(adminCtx context.Context) error {
		ids, err := o.schoolIDs.ListTraegerSchoolIDs(adminCtx, organizationID)
		if err != nil {
			return err
		}
		schools, err = o.rows.Schools(adminCtx, ids)
		return err
	})
	if err != nil {
		return nil, err
	}
	return schools, nil
}
