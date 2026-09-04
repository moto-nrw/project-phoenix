package compose

import (
	"context"
	"errors"
	"testing"
	"time"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	staff "github.com/moto-nrw/project-phoenix/modules/communication/internal/staffannouncements"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type parentAnnouncementServiceStub struct {
	staff.Service
	row *usersModels.ParentAnnouncement
	err error
}

func (s parentAnnouncementServiceStub) Get(context.Context, int64) (*usersModels.ParentAnnouncement, error) {
	return s.row, s.err
}

func TestParentAnnouncementFacadeMapsLegacyRows(t *testing.T) {
	t.Parallel()
	refID := int64(23)
	refText := "2b"
	deadline := time.Now().Add(time.Hour)
	row := &usersModels.ParentAnnouncement{
		Title: "Ausflug", Body: "Bitte Rucksack mitbringen", Priority: usersModels.ParentAnnouncementPriorityImportant,
		ResponseType: usersModels.ParentAnnouncementResponseSingleChoice, ResponseDeadline: &deadline,
		Targets: []*usersModels.ParentAnnouncementTarget{{TargetType: usersModels.AnnouncementTargetStudent, TargetRefID: &refID, TargetRefText: &refText}},
		Options: []*usersModels.ParentAnnouncementOption{{Label: "Ja"}},
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
	legacyErr := errors.Join(staff.ErrNotFound, databaseErr)
	capability := &parentAnnouncements{service: parentAnnouncementServiceStub{err: legacyErr}}

	_, err := capability.GetParentAnnouncement(context.Background(), 9)
	require.Error(t, err)
	assert.ErrorIs(t, err, communication.ErrParentAnnouncementNotFound)
	assert.ErrorIs(t, err, staff.ErrNotFound)
	assert.ErrorIs(t, err, databaseErr)
}
