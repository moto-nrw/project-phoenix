package contracttest_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// Opening a new transaction for the link cannot validate a fact from a
// previously committed create. Only the caller's ambient unit may carry it.
func TestGuardianDirectory_CreatedStudentNeedsAmbientTransaction(t *testing.T) {
	t.Parallel()
	for _, ambient := range []bool{false, true} {
		t.Run(map[bool]string{false: "top level", true: "ambient"}[ambient], func(t *testing.T) {
			t.Parallel()
			db := testpkg.SetupTestDB(t)
			student := testpkg.CreateTestStudent(t, db, "Created", "Fact", "2a")
			guardian := testpkg.CreateTestGuardianProfile(t, db, "created-fact@example.test")
			created := &peopledirectory.CreatedEnrollmentStudent{ID: student.ID, TenantID: testpkg.Tenant(t)}
			createdGuardian := &peopledirectory.Guardian{ID: guardian.ID, TenantID: created.TenantID}
			ctx := tenant.WithTenantID(context.Background(), created.TenantID)
			if ambient {
				ctx = tenant.WithTransactionForTest(ctx, struct{}{})
			}
			stop := errors.New("stop before link mutation")
			service := guardianFactService{link: func(txCtx context.Context, req peoplePorts.StudentGuardianCreateRequest) (*peopledirectory.GuardianRelationshipSnapshot, error) {
				_, inTransaction := tenant.TransactionFromContext(txCtx)
				require.True(t, inTransaction)
				require.Equal(t, created.ID, req.StudentID)
				if ambient {
					require.Same(t, created, req.CreatedStudent)
					require.Same(t, createdGuardian, req.CreatedGuardian)
				} else {
					require.Nil(t, req.CreatedStudent)
					require.Nil(t, req.CreatedGuardian)
				}
				return nil, stop
			}}
			directory, err := peopleCompose.New(peopleCompose.Dependencies{
				DB: db,
				StudentDirectory: struct {
					peopleCompose.StudentDirectoryRows
				}{},
				Observe: func(peopleCompose.Observation) {},
				GuardianProvider: &peopleCompose.GuardianDirectoryProvider{
					Guardians: service, Runtime: guardianFactRuntime{},
					PermissionGranted: func(value any) bool { return value == true },
				},
			}, nil)
			require.NoError(t, err)
			_, err = directory.LinkGuardianToStudent(ctx, peopledirectory.LinkGuardian{
				StudentID: created.ID, GuardianProfileID: guardian.ID, CreatedStudent: created, CreatedGuardian: createdGuardian,
				RelationshipType: "parent", GuardianRole: "primary_guardian",
			})
			require.ErrorIs(t, err, stop)
		})
	}
}

type guardianFactService struct {
	peoplePorts.GuardianService
	link func(context.Context, peoplePorts.StudentGuardianCreateRequest) (*peopledirectory.GuardianRelationshipSnapshot, error)
}

func (s guardianFactService) LinkGuardianToStudent(ctx context.Context, req peoplePorts.StudentGuardianCreateRequest) (*peopledirectory.GuardianRelationshipSnapshot, error) {
	return s.link(ctx, req)
}

type guardianFactRuntime struct{}

func (guardianFactRuntime) InTenantTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(tenant.WithTransactionForTest(ctx, struct{}{}))
}
