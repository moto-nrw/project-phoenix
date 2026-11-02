package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/ports"
)

// demoSchoolFileBatch bounds one run. The demo process runs hourly; a larger
// backlog takes several runs.
const demoSchoolFileBatch = 50

// DemoSchoolFiles removes the uploads of retired demo schools (#5544) with
// the namespaces and removal of the school erasure, without its order,
// receipt or ledger. The demo process is the only caller.
type DemoSchoolFiles struct {
	store ports.DemoSchoolFileStore
	files organizationtenancy.ErasureFiles
	kinds []organizationtenancy.LifecycleFileKind
}

// NewDemoSchoolFiles reads and checks the file catalog at once, so a catalog
// without reviewed deletion paths stops the demo process at start.
func NewDemoSchoolFiles(store ports.DemoSchoolFileStore, files organizationtenancy.ErasureFiles) (*DemoSchoolFiles, error) {
	if store == nil || files == nil {
		return nil, errors.New("demo school files: database and file store required")
	}
	kinds, err := erasureFileCatalog()
	if err != nil {
		return nil, err
	}
	return &DemoSchoolFiles{store: store, files: files, kinds: kinds}, nil
}

// EraseRetiredDemoSchoolFiles works through one batch. Per school it removes
// every namespace and counts again; only an empty result writes the marker.
// A failed school keeps no marker and is due again in the next run; the other
// schools of the batch go on.
func (d *DemoSchoolFiles) EraseRetiredDemoSchoolFiles(ctx context.Context) (int, int64, error) {
	ids, err := d.store.RetiredDemoSchools(ctx, organizationtenancy.StandingDemoSchoolSlug, demoSchoolFileBatch)
	if err != nil {
		return 0, 0, err
	}
	var (
		schools int
		removed int64
		failed  []error
	)
	for _, id := range ids {
		n, err := d.eraseSchool(ctx, id)
		if err != nil {
			failed = append(failed, fmt.Errorf("demo school %d: files not erased, retried next run: %w", id, err))
			continue
		}
		if n >= 0 {
			schools++
			removed += n
		}
	}
	return schools, removed, errors.Join(failed...)
}

// eraseSchool returns the objects removed, or -1 when the school was no
// longer due when its marker was written.
func (d *DemoSchoolFiles) eraseSchool(ctx context.Context, id int64) (int64, error) {
	counts, err := removeAndVerify(func(remove bool) (map[string]int64, error) {
		return tenantFiles(ctx, d.files, id, d.kinds, remove, nil)
	})
	if err != nil {
		return 0, err
	}
	recorded, err := d.store.RecordDemoSchoolFileErasure(ctx, organizationtenancy.StandingDemoSchoolSlug, id, counts)
	if err != nil || !recorded {
		return -1, err
	}
	var total int64
	for _, n := range counts {
		total += n
	}
	return total, nil
}
