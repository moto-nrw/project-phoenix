package testutil

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet/deviceauth"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/uptrace/bun"
)

// DeviceSchools answers the device school guard through the Organisation &
// Tenancy capability over db, the owner the serving root binds, so handler
// tests exercise the deleted-school check against real fixtures.
func DeviceSchools(t testing.TB, db *bun.DB) deviceauth.SchoolDirectory {
	t.Helper()
	lookup, err := services.SchoolDeletionLookupForTests(db, testpkg.TenantRuntime(t, db))
	if err != nil {
		t.Fatalf("compose device school lookup: %v", err)
	}
	return deviceSchools{lookup: lookup}
}

type deviceSchools struct {
	lookup func(context.Context, int64) (found, deleted bool, err error)
}

func (d deviceSchools) FindSchool(ctx context.Context, id int64) (*deviceauth.School, error) {
	found, deleted, err := d.lookup(ctx, id)
	if err != nil || !found {
		return nil, err
	}
	return &deviceauth.School{Deleted: deleted}, nil
}
