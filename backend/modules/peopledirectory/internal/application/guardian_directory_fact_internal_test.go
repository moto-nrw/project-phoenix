package application

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/require"
)

// Opening a new transaction for the link cannot validate a fact from a
// previously committed create. Only the caller's ambient unit may carry it.
func TestGuardianDirectory_CreatedStudentNeedsAmbientTransaction(t *testing.T) {
	t.Parallel()
	for _, ambient := range []bool{false, true} {
		t.Run(map[bool]string{false: "top level", true: "ambient"}[ambient], func(t *testing.T) {
			t.Parallel()
			created := &peopledirectory.CreatedEnrollmentStudent{ID: 12, TenantID: 13}
			ctx := tenant.WithTenantID(context.Background(), created.TenantID)
			if ambient {
				ctx = tenant.WithTransactionForTest(ctx, struct{}{})
			}
			stop := errors.New("stop before link mutation")
			service := guardianFactService{link: func(txCtx context.Context, req peoplePorts.StudentGuardianCreateRequest) (*peopledirectory.GuardianRelationshipSnapshot, error) {
				require.True(t, (RepositoryRuntime{}).InTransaction(txCtx))
				require.Equal(t, created.ID, req.StudentID)
				if ambient {
					require.Same(t, created, req.CreatedStudent)
				} else {
					require.Nil(t, req.CreatedStudent)
				}
				return nil, stop
			}}
			directory := NewGuardianDirectory(service, guardianFactRuntime{}, nil)
			_, err := directory.LinkGuardianToStudent(ctx, peopledirectory.LinkGuardian{
				StudentID: created.ID, GuardianProfileID: 14, CreatedStudent: created,
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
