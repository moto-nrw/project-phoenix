package platform

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"
)

func TestAnnouncementView_TableName(t *testing.T) {
	view := &AnnouncementView{}
	assert.Equal(t, "platform.announcement_views", view.TableName())
}

func TestAnnouncementView_BeforeAppendModel_SelectQuery(t *testing.T) {
	view := &AnnouncementView{}
	query := &bun.SelectQuery{}
	err := view.BeforeAppendModel(query)
	assert.NoError(t, err)
}

func TestAnnouncementView_BeforeAppendModel_UpdateQuery(t *testing.T) {
	view := &AnnouncementView{}
	query := &bun.UpdateQuery{}
	err := view.BeforeAppendModel(query)
	assert.NoError(t, err)
}

func TestAnnouncementView_BeforeAppendModel_DeleteQuery(t *testing.T) {
	view := &AnnouncementView{}
	query := &bun.DeleteQuery{}
	err := view.BeforeAppendModel(query)
	assert.NoError(t, err)
}

func TestAnnouncementView_BeforeAppendModel_OtherQueryType(t *testing.T) {
	view := &AnnouncementView{}
	// Pass some other type - should not panic
	err := view.BeforeAppendModel("not a query")
	assert.NoError(t, err)
}
