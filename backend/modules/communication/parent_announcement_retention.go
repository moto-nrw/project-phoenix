package communication

import "context"

// ParentAnnouncementRetention deletes a school's ended parent announcements
// once its parent message window has passed (#4045). A published
// announcement counts from its expiry or, without one, from its
// publication; a draft from its last change. An announcement whose
// declaration has submissions stays until the submissions go with their
// child. Each deletion goes the way of a manual delete: pending mail and
// pushes are cancelled, delivery rows dropped and attachment files queued
// for removal. The composed ParentAnnouncementCapability implements it.
type ParentAnnouncementRetention interface {
	DeleteExpiredParentAnnouncements(ctx context.Context, retentionDays int) (int, error)
}
