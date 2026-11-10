package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
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
// erased school counts. A busy school waits for the next run without failing
// it; a failed school is an error of the run, so the run never reports
// success over a school it could not erase.
func (d *DemoSchoolDatabases) EraseRetiredDemoSchoolDatabases(ctx context.Context) (int, int, error) {
	ids, err := d.store.DueDemoSchoolDatabases(ctx, demoSchoolDatabaseBatch)
	if err != nil {
		return 0, 0, err
	}
	var (
		erased, busy int
		failed       []error
	)
	for _, id := range ids {
		status, code, err := d.store.EraseDemoSchoolDatabase(ctx, id)
		switch {
		case err != nil:
			failed = append(failed, fmt.Errorf("demo school %d: database not erased, retried next run: %w", id, err))
		case status == ports.DemoSchoolErased:
			erased++
		case status == ports.DemoSchoolBusy:
			busy++
		case status == ports.DemoSchoolNotDue:
		default:
			failed = append(failed, fmt.Errorf("demo school %d: database erasure %s (SQLSTATE %q), retried next run", id, status, code))
		}
	}
	return erased, busy, errors.Join(failed...)
}

// DemoSchoolErasure erases retired demo schools: their files first (#5544),
// then their database rows (#5559).
type DemoSchoolErasure struct {
	*DemoSchoolFiles
	*DemoSchoolDatabases
}

func NewDemoSchoolErasure(fileStore ports.DemoSchoolFileStore, databaseStore ports.DemoSchoolDatabaseStore, files organizationtenancy.ErasureFiles) (*DemoSchoolErasure, error) {
	fileErasure, err := NewDemoSchoolFiles(fileStore, files)
	if err != nil {
		return nil, err
	}
	databases, err := NewDemoSchoolDatabases(databaseStore)
	if err != nil {
		return nil, err
	}
	return &DemoSchoolErasure{DemoSchoolFiles: fileErasure, DemoSchoolDatabases: databases}, nil
}
