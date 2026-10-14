package inbound_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/workforce/inbound"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The typed root composes the owners the internal route tests run over. It
// imports this package, so the binding lives in the external test package.
func init() {
	inbound.ComposeAbsenceTypes = func(t *testing.T, clocks ...func() time.Time) (*bun.DB, inbound.AbsenceTypesComposition) {
		t.Helper()
		db := testpkg.SetupTestDB(t)
		module, err := api.NewAbsenceTypeTestModule(testpkg.NewDatabase(t, db), clocks...)
		require.NoError(t, err)
		return db, inbound.AbsenceTypesComposition{Administration: module.Administration, ActingStaff: module.UserContext.Caller().StaffID}
	}
}
