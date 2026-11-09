package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/ports"
)

// demoSchoolDatabaseBatch bounds one run. The demo process runs hourly; a
// larger backlog takes several runs.
const demoSchoolDatabaseBatch = 10

// DemoSchoolDatabases erases retired demo schools from the database (#5559).
type DemoSchoolDatabases struct{ store ports.DemoSchoolDatabaseStore }

func NewDemoSchoolDatabases(store ports.DemoSchoolDatabaseStore) (*DemoSchoolDatabases, error) {
	if store == nil {
		return nil, errors.New("demo school databases: database required")
	}
	return &DemoSchoolDatabases{store: store}, nil
}

// EraseRetiredDemoSchoolDatabases erases each due school on its own. Only an
// erased school counts; a failed or busy school is an error of the run, so
// the run never reports success over a school it left behind.
func (d *DemoSchoolDatabases) EraseRetiredDemoSchoolDatabases(ctx context.Context) (int, error) {
	ids, err := d.store.DueDemoSchoolDatabases(ctx, demoSchoolDatabaseBatch)
	if err != nil {
		return 0, err
	}
	var (
		erased int
		failed []error
	)
	for _, id := range ids {
		status, code, err := d.store.EraseDemoSchoolDatabase(ctx, id)
		switch {
		case err != nil:
			failed = append(failed, fmt.Errorf("demo school %d: database not erased, retried next run: %w", id, err))
		case status == "erased":
			erased++
		case status == "not_due":
		default:
			failed = append(failed, fmt.Errorf("demo school %d: database erasure %s (SQLSTATE %q), retried next run", id, status, code))
		}
	}
	return erased, errors.Join(failed...)
}
