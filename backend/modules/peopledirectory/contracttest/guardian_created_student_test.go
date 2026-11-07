package contracttest_test

import (
	"context"
	"errors"
	"testing"

	people "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/require"
)

// Only a matching create result in the same tenant transaction replaces the
// child read. Guardian lookup still wins, even with a valid child fact.
func TestLinkGuardian_CreatedStudentFact(t *testing.T) {
	t.Parallel()
	const guardianID, studentID, schoolID int64 = 21, 22, 23
	ctx := tenant.WithTenantID(guardianAmbientTxContext(), schoolID)
	created := people.CreatedEnrollmentStudent{ID: studentID, TenantID: schoolID}
	foreign, mismatched, zeroID, noTenant := created, created, created, created
	foreign.TenantID++
	mismatched.ID++
	zeroID.ID = 0
	noTenant.TenantID = 0
	guardianFailure := errors.New("guardian lookup failed")
	studentFailure := errors.New("student lookup failed")
	for _, tc := range []struct {
		name        string
		ctx         context.Context
		created     *people.CreatedEnrollmentStudent
		guardianErr error
		want        []string
		err         error
	}{
		{"created child", ctx, &created, nil, []string{"guardian", "link"}, nil},
		{"existing child", ctx, nil, nil, []string{"guardian", "student"}, studentFailure},
		{"other child", ctx, &mismatched, nil, []string{"guardian", "student"}, studentFailure},
		{"missing ID", ctx, &zeroID, nil, []string{"guardian", "student"}, studentFailure},
		{"other tenant", ctx, &foreign, nil, []string{"guardian", "student"}, studentFailure},
		{"missing tenant", ctx, &noTenant, nil, []string{"guardian", "student"}, studentFailure},
		{"no tenant scope", guardianAmbientTxContext(), &noTenant, nil, []string{"guardian", "student"}, studentFailure},
		{"no transaction", tenant.WithTenantID(context.Background(), schoolID), &created, nil, []string{"guardian", "student"}, studentFailure},
		{"guardian before created child", ctx, &created, guardianFailure, []string{"guardian"}, guardianFailure},
		{"guardian before existing child", ctx, nil, guardianFailure, []string{"guardian"}, guardianFailure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var calls []string
			service := peopleCompose.NewGuardianService(peoplePorts.GuardianServiceDependencies{
				GuardianProfileRepo: &fakeProfileRepo{findByIDFn: func(_ context.Context, id int64) (*people.GuardianProfileSnapshot, error) {
					require.Equal(t, guardianID, id)
					calls = append(calls, "guardian")
					return &people.GuardianProfileSnapshot{}, tc.guardianErr
				}},
				StudentRepo: &fakeStudentRepo{findByIDFn: func(_ context.Context, id interface{}) (*people.StudentState, error) {
					require.Equal(t, studentID, id)
					calls = append(calls, "student")
					return nil, studentFailure
				}},
				RolePresets: peoplePorts.GuardianRolePresetFuncs{Presets: func(role string) (string, []string) {
					return role, []string{"parent_portal.access"}
				}},
				StudentGuardianRepo: createdStudentLinkRepo{link: func(_ context.Context, link *people.GuardianRelationshipSnapshot) (bool, error) {
					calls = append(calls, "link")
					require.Equal(t, studentID, link.StudentID)
					require.Equal(t, guardianID, link.GuardianProfileID)
					require.Equal(t, schoolID, link.TenantID)
					require.Equal(t, "parent", link.RelationshipType)
					require.Equal(t, "primary_guardian", link.GuardianRole)
					require.True(t, link.CanPickup)
					require.JSONEq(t, `{"parent_portal.access":true}`, string(link.Permissions))
					return true, nil
				}},
			}, nil)
			link, err := service.LinkGuardianToStudent(tc.ctx, peoplePorts.StudentGuardianCreateRequest{
				StudentID: studentID, GuardianProfileID: guardianID, CreatedStudent: tc.created,
				RelationshipType: "parent", GuardianRole: "primary_guardian", CanPickup: true,
			})
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
				require.ErrorIs(t, err, people.ErrGuardianNotFound)
				require.Nil(t, link)
			} else {
				require.NoError(t, err)
				require.NotNil(t, link)
			}
			require.Equal(t, tc.want, calls)
		})
	}
}

type createdStudentLinkRepo struct {
	peoplePorts.StudentGuardianRepository
	link func(context.Context, *people.GuardianRelationshipSnapshot) (bool, error)
}

func (r createdStudentLinkRepo) LinkIfNotExists(ctx context.Context, link *people.GuardianRelationshipSnapshot) (bool, error) {
	return r.link(ctx, link)
}
