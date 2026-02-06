package platform

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAnnouncementView_TableName(t *testing.T) {
	view := &AnnouncementView{}
	assert.Equal(t, "platform.announcement_views", view.TableName())
}
