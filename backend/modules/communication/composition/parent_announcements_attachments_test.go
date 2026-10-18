package compose

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewParentAnnouncementsWithAttachmentsRequiresThePurger(t *testing.T) {
	t.Parallel()
	_, err := NewParentAnnouncementsWithAttachments(ParentAnnouncementConfig{}, nil, nil, nil)
	require.EqualError(t, err, "parent announcements compose: the attachment purger is required")
}
