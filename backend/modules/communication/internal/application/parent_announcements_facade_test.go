package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type parentAnnouncementServiceStub struct {
	StaffService
	row *StaffParentAnnouncement
	err error
}

func (s parentAnnouncementServiceStub) Get(context.Context, int64) (*StaffParentAnnouncement, error) {
	return s.row, s.err
}

func TestParentAnnouncementFacadeMapsLegacyRows(t *testing.T) {
	t.Parallel()
	refID := int64(23)
	refText := "2b"
	deadline := time.Now().Add(time.Hour)
	row := &StaffParentAnnouncement{
		Title: "Ausflug", Body: "Bitte Rucksack mitbringen", Priority: StaffParentAnnouncementPriorityImportant,
		ResponseType: StaffParentAnnouncementResponseSingleChoice, ResponseDeadline: &deadline,
		Targets: []*StaffParentAnnouncementTarget{{TargetType: StaffAnnouncementTargetStudent, TargetRefID: &refID, TargetRefText: &refText}},
		Options: []*StaffParentAnnouncementOption{{Label: "Ja"}},
	}
	row.ID = 41
	optionID := time.Now().UnixNano()
	row.Options[0].ID = optionID
	capability := &parentAnnouncements{service: parentAnnouncementServiceStub{row: row}}

	got, err := capability.GetParentAnnouncement(context.Background(), row.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, row.ID, got.ID)
	assert.Equal(t, row.ResponseType, got.ResponseType)
	assert.Equal(t, deadline, *got.ResponseDeadline)
	require.Len(t, got.Targets, 1)
	assert.Equal(t, refID, *got.Targets[0].RefID)
	assert.Equal(t, refText, *got.Targets[0].RefText)
	require.Len(t, got.Options, 1)
	assert.Equal(t, optionID, got.Options[0].ID)
}

func TestParentAnnouncementFacadeMapsStableErrorsWithoutLosingCause(t *testing.T) {
	t.Parallel()
	databaseErr := errors.New("database unavailable")
	legacyErr := errors.Join(ErrNotFound, databaseErr)
	capability := &parentAnnouncements{service: parentAnnouncementServiceStub{err: legacyErr}}

	_, err := capability.GetParentAnnouncement(context.Background(), 9)
	require.Error(t, err)
	assert.ErrorIs(t, err, communication.ErrParentAnnouncementNotFound)
	assert.ErrorIs(t, err, ErrNotFound)
	assert.ErrorIs(t, err, databaseErr)
}
