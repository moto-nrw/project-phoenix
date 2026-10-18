package shiftplansync_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/workflows/shiftplansync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type handoverSubstitutions struct {
	createErr error
	created   workforce.GroupSubstitution
	deleted   int64
}

func (s *handoverSubstitutions) CreateGroupSubstitution(_ context.Context, value workforce.GroupSubstitution) (workforce.GroupSubstitution, error) {
	if s.createErr != nil {
		return workforce.GroupSubstitution{}, s.createErr
	}
	value.ID = 501
	s.created = value
	return value, nil
}

func (s *handoverSubstitutions) DeleteGroupSubstitution(_ context.Context, id int64) error {
	s.deleted = id
	return nil
}

func TestGroupHandoversWriteThroughWorkforce(t *testing.T) {
	t.Parallel()
	rows := &handoverSubstitutions{}
	handovers := shiftplansync.NewGroupHandovers(rows)

	created, err := handovers.CreateHandover(context.Background(), workforce.GroupSubstitution{
		TargetType: workforce.GroupSubstitutionTypeGroupHandover, GroupID: 61, SubstituteStaffID: 71,
		StartDate: "2026-10-19", EndDate: "2026-10-23", Reason: "Gruppenübergabe",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(501), created.ID)
	assert.Equal(t, "Gruppenübergabe", rows.created.Reason)

	require.NoError(t, handovers.DeleteHandover(context.Background(), 501))
	assert.Equal(t, int64(501), rows.deleted)
}

func TestGroupHandoversReportADuplicateAsAnExistingHandover(t *testing.T) {
	t.Parallel()
	duplicate := &workforce.ConflictError{Kind: workforce.ErrGroupSubstitutionExists, Cause: errors.New("unique violation")}
	_, err := shiftplansync.NewGroupHandovers(&handoverSubstitutions{createErr: duplicate}).CreateHandover(context.Background(), workforce.GroupSubstitution{})
	require.ErrorIs(t, err, shiftplansync.ErrHandoverExists)

	refused := errors.New("refused")
	_, err = shiftplansync.NewGroupHandovers(&handoverSubstitutions{createErr: refused}).CreateHandover(context.Background(), workforce.GroupSubstitution{})
	require.ErrorIs(t, err, refused, "every other failure passes through")
	require.NotErrorIs(t, err, shiftplansync.ErrHandoverExists)

	assert.Panics(t, func() { shiftplansync.NewGroupHandovers(nil) })
}
