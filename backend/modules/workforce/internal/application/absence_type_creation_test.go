package application

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type absenceTypeCreationStore struct {
	ports.Store
	listErr, createErr error
	creates            int
}

func (s *absenceTypeCreationStore) ListStaffAbsenceTypes(context.Context) ([]domain.StaffAbsenceType, domain.OperationStats, error) {
	return nil, domain.OperationStats{}, s.listErr
}

func (s *absenceTypeCreationStore) CreateStaffAbsenceType(context.Context, domain.StaffAbsenceTypeFields) (domain.StaffAbsenceType, domain.OperationStats, error) {
	s.creates++
	return domain.StaffAbsenceType{}, domain.OperationStats{}, s.createErr
}

type absenceTypeCreationTransaction struct{ ports.Transaction }

func (absenceTypeCreationTransaction) RunWrite(ctx context.Context, run func(context.Context) error) error {
	return run(ctx)
}

func TestCreateAbsenceTypePreservesFailureContracts(t *testing.T) {
	t.Parallel()
	failure := errors.New("database unavailable")
	for _, tc := range []struct {
		name               string
		listErr, createErr error
		creates            int
		message            string
	}{
		{name: "list failure never writes", listErr: failure, message: "database error during list all staff absence types: database unavailable"},
		{name: "write failure", createErr: failure, creates: 1, message: "database error during create: database unavailable"},
		{name: "concurrent duplicate", createErr: &domain.ConflictError{Kind: domain.ErrAbsenceTypeNameTaken, Cause: failure}, creates: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &absenceTypeCreationStore{listErr: tc.listErr, createErr: tc.createErr}
			service := &Service{store: store, transaction: absenceTypeCreationTransaction{}, observe: func(ports.Observation) {}}
			created, err := service.CreateAbsenceType(context.Background(), domain.StaffAbsenceTypeFields{Name: "Custom"})
			require.ErrorIs(t, err, failure)
			assert.Zero(t, created)
			assert.Equal(t, tc.creates, store.creates)
			if tc.message != "" {
				assert.EqualError(t, err, tc.message)
			} else {
				require.ErrorIs(t, err, domain.ErrAbsenceTypeNameTaken)
			}
		})
	}
}
