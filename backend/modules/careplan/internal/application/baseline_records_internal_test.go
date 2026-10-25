package application

import (
	"context"
	"testing"

	carePlanPorts "github.com/moto-nrw/project-phoenix/modules/careplan/ports"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

type excusalStoreSpy struct {
	carePlanPorts.PickupExcusalRecords
	writes int
}

func (s *excusalStoreSpy) ListDepartureExceptions(context.Context, careplan.StudentScheduleFilter) ([]careplan.DepartureException, error) {
	return nil, nil
}

func (s *excusalStoreSpy) SaveDepartureException(_ context.Context, row careplan.DepartureException) (careplan.DepartureException, error) {
	s.writes++
	return row, nil
}

func (s *excusalStoreSpy) CreatePickupNote(_ context.Context, row careplan.PickupNote) (careplan.PickupNote, error) {
	s.writes++
	return row, nil
}

func (s *excusalStoreSpy) CreatePickupException(_ context.Context, row careplan.PickupException) (careplan.PickupException, error) {
	s.writes++
	return row, nil
}

func (s *excusalStoreSpy) UpdatePickupException(context.Context, careplan.PickupException) error {
	s.writes++
	return nil
}

func TestPickupExcusalRecordsRejectInvalidRowsBeforeStorage(t *testing.T) {
	t.Parallel()
	spy := &excusalStoreSpy{}
	records := PickupExcusalRows{spy}

	require.EqualError(t, records.Create(context.Background(), &careplan.PickupException{}), "student_id is required")
	require.EqualError(t, records.Update(context.Background(), &careplan.PickupException{}), "student_id is required")
	assert.Zero(t, spy.writes)
}

func TestPickupApprovalExceptionsRejectInvalidRowsBeforeStorage(t *testing.T) {
	t.Parallel()
	spy := &excusalStoreSpy{}
	approvals := pickupApprovalExceptions{records: spy}

	_, err := approvals.Create(context.Background(), careplan.PickupException{})
	require.EqualError(t, err, "student_id is required")
	require.EqualError(t, approvals.Update(context.Background(), careplan.PickupException{}), "student_id is required")
	assert.Zero(t, spy.writes)
}
