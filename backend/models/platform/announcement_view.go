package platform

import (
	"time"

	"github.com/uptrace/bun"
)

// tablePlatformAnnouncementViews is the schema-qualified table name
const tablePlatformAnnouncementViews = "platform.announcement_views"

// AnnouncementView tracks which users have seen which announcements
type AnnouncementView struct {
	UserID         int64     `bun:"user_id,pk" json:"user_id"`
	AnnouncementID int64     `bun:"announcement_id,pk" json:"announcement_id"`
	SeenAt         time.Time `bun:"seen_at,notnull,default:current_timestamp" json:"seen_at"`
	Dismissed      bool      `bun:"dismissed,notnull,default:false" json:"dismissed"`

	// Relations
	Announcement *Announcement `bun:"rel:belongs-to,join:announcement_id=id" json:"announcement,omitempty"`
}

func (v *AnnouncementView) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr(tablePlatformAnnouncementViews)
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(tablePlatformAnnouncementViews)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(tablePlatformAnnouncementViews)
	}
	return nil
}

// TableName returns the database table name
func (v *AnnouncementView) TableName() string {
	return tablePlatformAnnouncementViews
}
