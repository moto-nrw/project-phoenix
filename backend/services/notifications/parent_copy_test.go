package notifications

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParentMessageCopyUsesPortalLocale(t *testing.T) {
	title, body := ParentMessageCopy("en")
	assert.Equal(t, "New message from the OGS", title)
	assert.Equal(t, "You have a new message in the parent portal.", body)
}

func TestParentAppointmentCopyUsesPortalLocale(t *testing.T) {
	title, body := ParentAppointmentCopy("sq", ParentAppointmentCancelled)
	assert.Equal(t, "Takimi u anulua", title)
	assert.Equal(t, "Një takim për ju është anuluar.", body)
}

func TestParentAnnouncementCopyUsesPortalLocale(t *testing.T) {
	title, body := ParentAnnouncementCopy("ru", ParentPollReminder)
	assert.Equal(t, "Напоминание: опрос открыт", title)
	assert.Contains(t, body, "ребёнка")
}

func TestParentRequestDecisionCopyUsesPortalLocale(t *testing.T) {
	title, body := ParentRequestDecisionCopy("sq", "master_data", "abgelehnt")
	assert.Equal(t, "Kërkesa u refuzua", title)
	assert.Equal(t, "Kërkesa juaj për të dhënat bazë u refuzua.", body)
}
