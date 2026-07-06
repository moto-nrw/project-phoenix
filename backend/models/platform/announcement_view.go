package platform

import (
	"time"
)

// AnnouncementView tracks which users have seen which announcements
type AnnouncementView struct {
	UserID         int64     `bun:"user_id,pk" json:"user_id"`
	AnnouncementID int64     `bun:"announcement_id,pk" json:"announcement_id"`
	SeenAt         time.Time `bun:"seen_at,notnull,default:current_timestamp" json:"seen_at"`
	Dismissed      bool      `bun:"dismissed,notnull,default:false" json:"dismissed"`

	// Relations
}
