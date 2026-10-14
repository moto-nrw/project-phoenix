package students_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/inbound/students"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The typed root's test compositions these suites build: the setup helpers
// open the package's test database and compose the module through the root,
// the others work on the database their caller opened (#4005).

// deviceAuthenticator is the kiosk device authentication over the module's
// fleet and settings, with the root's guard against deleted schools.
func deviceAuthenticator(t testing.TB, db *bun.DB, svc api.StudentTestModule) common.Middleware {
	t.Helper()
	schools, err := api.NewDeviceSchools(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	return testutil.NewDeviceAuthenticators(svc.IoT.Fleet(), schools, svc.Settings, testDevicePIN).Device()
}
func setupBirthdayModule(t *testing.T, clocks ...func() time.Time) (*bun.DB, api.BirthdayTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := api.NewBirthdayTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), clocks...)
	require.NoError(t, err)
	return db, module
}

func setupStudentModule(t *testing.T, clocks ...func() time.Time) (*bun.DB, api.StudentTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	feedback, err := api.NewFeedbackTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	module, err := api.NewStudentTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), feedback.Feedback, clocks...)
	require.NoError(t, err)
	return db, module
}

// The root imports this package, so the internal tests reach its student
// graph through this binding.
func init() {
	students.ComposeStudentRows = func(db *bun.DB) students.StudentRows {
		rows := api.MustNewStudentHTTPTestRows(testpkg.MustDatabase(db))
		return students.StudentRows{
			PeopleDirectory:  api.NewStudentHTTPTestPeople(testpkg.MustDatabase(db)),
			Companions:       api.NewStudentHTTPTestCompanions(rows, api.NewStudentHTTPTestPeople(testpkg.MustDatabase(db)), nil),
			StudentStatusDay: rows.StudentStatusDay,
			LockExceptionDay: rows.CarePlan.LockExceptionDay,
		}
	}
}
