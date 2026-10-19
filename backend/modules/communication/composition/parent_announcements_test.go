package compose

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	staff "github.com/moto-nrw/project-phoenix/modules/communication/internal/staffannouncements"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type parentAnnouncementServiceStub struct {
	staff.Service
	row *staff.ParentAnnouncement
	err error
}

func (s parentAnnouncementServiceStub) Get(context.Context, int64) (*staff.ParentAnnouncement, error) {
	return s.row, s.err
}

func TestParentAnnouncementFacadeMapsLegacyRows(t *testing.T) {
	t.Parallel()
	refID := int64(23)
	refText := "2b"
	deadline := time.Now().Add(time.Hour)
	row := &staff.ParentAnnouncement{
		Title: "Ausflug", Body: "Bitte Rucksack mitbringen", Priority: staff.ParentAnnouncementPriorityImportant,
		ResponseType: staff.ParentAnnouncementResponseSingleChoice, ResponseDeadline: &deadline,
		Targets: []*staff.ParentAnnouncementTarget{{TargetType: staff.AnnouncementTargetStudent, TargetRefID: &refID, TargetRefText: &refText}},
		Options: []*staff.ParentAnnouncementOption{{Label: "Ja"}},
	}
	row.ID = 41
	optionID := time.Now().UnixNano()
	row.Options[0].ID = optionID
	capability := &parentAnnouncements{Service: parentAnnouncementServiceStub{row: row}}

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
	legacyErr := errors.Join(staff.ErrNotFound, databaseErr)
	capability := &parentAnnouncements{Service: parentAnnouncementServiceStub{err: legacyErr}}

	_, err := capability.GetParentAnnouncement(context.Background(), 9)
	require.Error(t, err)
	assert.ErrorIs(t, err, communication.ErrParentAnnouncementNotFound)
	assert.ErrorIs(t, err, staff.ErrNotFound)
	assert.ErrorIs(t, err, databaseErr)
}
