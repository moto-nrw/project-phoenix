package api

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	educationRepo "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type handoverWritesStub struct {
	createErr error
	created   workforce.GroupSubstitution
	deleted   int64
}

func (s *handoverWritesStub) CreateGroupSubstitution(_ context.Context, value workforce.GroupSubstitution) (workforce.GroupSubstitution, error) {
	if s.createErr != nil {
		return workforce.GroupSubstitution{}, s.createErr
	}
	value.ID = 501
	s.created = value
	return value, nil
}

func (s *handoverWritesStub) DeleteGroupSubstitution(_ context.Context, id int64) error {
	s.deleted = id
	return nil
}

func TestEducationHandoversWriteThroughWorkforce(t *testing.T) {
	t.Parallel()
	writes := &handoverWritesStub{}
	handovers := NewEducationHandovers(nil, writes)
	handover := &educationRepo.GroupSubstitution{
		TargetType: workforce.GroupSubstitutionTypeGroupHandover, GroupID: 61, SubstituteStaffID: 71,
		StartDate: calendar.NewDate(2026, 10, 19), EndDate: calendar.NewDate(2026, 10, 23), Reason: "Gruppenübergabe",
	}

	require.NoError(t, handovers.Create(context.Background(), handover))
	assert.Equal(t, int64(501), handover.ID)
	assert.Equal(t, "Gruppenübergabe", writes.created.Reason)
	assert.Equal(t, "2026-10-19", writes.created.StartDate)

	require.NoError(t, handovers.Delete(context.Background(), int64(501)))
	assert.Equal(t, int64(501), writes.deleted)
}

func TestEducationHandoversReportADuplicateAsAnExistingHandover(t *testing.T) {
	t.Parallel()
	duplicate := &workforce.ConflictError{Kind: workforce.ErrGroupSubstitutionExists, Cause: errors.New("unique violation")}
	err := NewEducationHandovers(nil, &handoverWritesStub{createErr: duplicate}).Create(context.Background(), &educationRepo.GroupSubstitution{})
	require.ErrorIs(t, err, educationRepo.ErrHandoverExists)

	refused := errors.New("refused")
	err = NewEducationHandovers(nil, &handoverWritesStub{createErr: refused}).Create(context.Background(), &educationRepo.GroupSubstitution{})
	require.ErrorIs(t, err, refused, "every other failure passes through")
	require.NotErrorIs(t, err, educationRepo.ErrHandoverExists)

	assert.Panics(t, func() { NewEducationHandovers(nil, nil) })
}
