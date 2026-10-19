package parent

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestParentSubmissionRunsWithResolvedTenantRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	own := testpkg.CreateTestPerson(t, db, "Submission", "Own")
	foreignTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenant)
	foreign := testpkg.CreateTestPersonForTenant(t, db, foreignTenant, "Submission", "Foreign")
	school := &EnrollmentSchool{ID: testpkg.Tenant(t), Active: true}
	rs := &Resource{SchoolService: &parentSubmitSchoolStub{school: school}, ParentService: &fakeParentService{}}
	req := httptest.NewRequest("POST", "/", nil).WithContext(testpkg.WithPackageTenantRuntime(context.Background()))
	rejected := errors.New("submission rejected after write")
	for _, failure := range []error{rejected, nil} {
		out := rs.runParentEnrollmentSubmit(req, own.ID, "school", func(ctx context.Context, schoolID, accountID int64, eligible bool, ip string) (EnrollmentResponse, error) {
			require.Equal(t, school.ID, tenant.FromContext(ctx))
			require.Equal(t, school.ID, schoolID)
			require.False(t, tenant.IsAdminTx(ctx))
			raw, ok := tenant.TransactionFromContext(ctx)
			require.True(t, ok)
			tx := raw.(bun.Tx)
			var bypass bool
			require.NoError(t, tx.NewRaw("SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user").Scan(ctx, &bypass))
			require.False(t, bypass)
			var ids []int64
			require.NoError(t, tx.NewSelect().Table("users.persons").Column("id").Where("id IN (?, ?)", own.ID, foreign.ID).Scan(ctx, &ids))
			require.Equal(t, []int64{own.ID}, ids)
			_, err := tx.NewUpdate().Table("users.persons").Set("first_name = ?", "Committed").Where("id = ?", own.ID).Exec(ctx)
			require.NoError(t, err)
			return nil, failure
		})
		if failure != nil {
			require.ErrorIs(t, out.submitErr, rejected)
		} else {
			require.NoError(t, out.resolveErr)
			require.NoError(t, out.submitErr)
		}
		var name string
		require.NoError(t, db.NewSelect().Table("users.persons").Column("first_name").Where("id = ?", own.ID).Scan(testpkg.Ctx(t), &name))
		if failure != nil {
			require.Equal(t, "Submission", name)
		} else {
			require.Equal(t, "Committed", name)
		}
	}
	school.ID = 0
	called := false
	out := rs.runParentEnrollmentSubmit(req, own.ID, "school", func(context.Context, int64, int64, bool, string) (EnrollmentResponse, error) {
		called = true
		return nil, nil
	})
	require.ErrorIs(t, out.resolveErr, tenant.ErrInvalidTenantID)
	require.False(t, called)
}
