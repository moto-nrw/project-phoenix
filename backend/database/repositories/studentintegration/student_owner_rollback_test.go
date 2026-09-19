package studentintegration_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"

	careCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

type failingStudentOwner struct {
	peopleCompose.StudentOwners
	stage   string
	failure error
}

func (f failingStudentOwner) Renew(ctx context.Context, record peopledirectory.StudentRecord) (int64, error) {
	id, err := f.StudentOwners.Renew(ctx, record)
	if err == nil && f.stage == "membership" {
		err = f.failure
	}
	return id, err
}

func (f failingStudentOwner) Enroll(ctx context.Context, record peopledirectory.StudentRecord) (int64, error) {
	id, err := f.StudentOwners.Enroll(ctx, record)
	if err == nil && f.stage == "membership" {
		err = f.failure
	}
	return id, err
}

func (f failingStudentOwner) SaveCare(ctx context.Context, id int64, record peopledirectory.StudentRecord, plan peopledirectory.StudentPlan, note *string, touched bool) error {
	if err := f.StudentOwners.SaveCare(ctx, id, record, plan, note, touched); err != nil {
		return err
	}
	if f.stage == "care" {
		return f.failure
	}
	return nil
}

func TestStudentOwnerWritesRollbackWhenCallerCatchesFailure(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"membership", "care"} {
		t.Run(stage, func(t *testing.T) {
			t.Parallel()
			db := testpkg.SetupTestDB(t)
			ctx := testpkg.Ctx(t)
			person := testpkg.CreateTestPerson(t, db, "Atomic", stage)
			membership, err := repositories.NewSchoolMembership(db)
			require.NoError(t, err)
			care, err := careCompose.NewStudentProfiles(db, func(careCompose.Observation) {})
			require.NoError(t, err)
			failure := errors.New("failure after owner write")
			module, err := peopleCompose.New(peopleCompose.Dependencies{
				DB: db, Observe: func(peopleCompose.Observation) {},
				StudentOwners: failingStudentOwner{StudentOwners: repositories.NewStudentOwners(membership, care), stage: stage, failure: failure},
			})
			require.NoError(t, err)
			require.NoError(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
				_, createErr := module.CreateStudent(txCtx, peopledirectory.StudentWrite{Record: peopledirectory.StudentRecord{
					PersonID: person.ID, SchoolClass: "1a", Status: "active",
				}})
				require.ErrorIs(t, createErr, failure)
				return nil // Deliberately commit the outer transaction after catching it.
			}))
			var count int
			require.NoError(t, db.NewRaw("SELECT count(*) FROM users.student_profiles WHERE tenant_id = ? AND person_id = ?", testpkg.Tenant(t), person.ID).Scan(ctx, &count))
			require.Zero(t, count, "no profile (or cascading membership/care row) may survive the failed command")
		})
	}
}

func TestStudentOwnerUpdatesRollbackWhenCallerCatchesFailure(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"membership", "care"} {
		for _, command := range []string{"update", "enrollment-profile"} {
			t.Run(stage+"/"+command, func(t *testing.T) {
				t.Parallel()
				db := testpkg.SetupTestDB(t)
				ctx := testpkg.Ctx(t)
				student := testpkg.CreateTestStudent(t, db, "Atomic", command, "1a")
				membership, err := repositories.NewSchoolMembership(db)
				require.NoError(t, err)
				care, err := careCompose.NewStudentProfiles(db, func(careCompose.Observation) {})
				require.NoError(t, err)
				failure := errors.New("failure after owner update")
				module, err := peopleCompose.New(peopleCompose.Dependencies{
					DB: db, Observe: func(peopleCompose.Observation) {},
					StudentOwners: failingStudentOwner{StudentOwners: repositories.NewStudentOwners(membership, care), stage: stage, failure: failure},
				})
				require.NoError(t, err)
				before, err := module.FindStudentRecord(ctx, student.ID)
				require.NoError(t, err)
				changed := "must roll back"
				require.NoError(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
					var updateErr error
					if command == "update" {
						record := before
						record.ExtraInfo = &changed
						record.SchoolClass = "2a"
						record.HealthInfo = &changed
						_, updateErr = module.UpdateStudent(txCtx, peopledirectory.StudentWrite{Record: record})
					} else {
						updateErr = module.ApplyEnrollmentProfile(txCtx, student.ID, enrollment.ProfilePatch{
							ExtraInfoSet: true, ExtraInfo: &changed, HealthInfoSet: true, HealthInfo: &changed,
						})
					}
					require.ErrorIs(t, updateErr, failure)
					return nil
				}))
				after, err := module.FindStudentRecord(ctx, student.ID)
				require.NoError(t, err)
				require.Equal(t, before, after, "every owner must retain its pre-command state")
			})
		}
	}
}
