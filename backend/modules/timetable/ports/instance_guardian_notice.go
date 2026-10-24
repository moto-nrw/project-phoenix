package ports

import (
	"context"
)

// GuardianNoticeAudience is how far a care cancellation notice reaches.
type GuardianNoticeAudience struct {
	FamilyCount int
}

// GuardianNoticePublication is one notice to publish.
type GuardianNoticePublication struct {
	StudentIDs []int64
	Title      string
	Body       string
	CreatedBy  int64
}

// GuardianNoticePublished is a published notice.
type GuardianNoticePublished struct {
	AnnouncementID int64
	RecipientCount int
}

// GuardianNotices is the consumer-owned port to Communication's care
// cancellation notice. A refused publication comes back wrapped in
// timetable.ErrGuardianNoticeInvalid.
type GuardianNotices interface {
	// ValidateNoticeText checks title and message before anything is written.
	ValidateNoticeText(title, message string) error
	// NoticeReach reports how many families the children's notice would
	// reach.
	NoticeReach(ctx context.Context, studentIDs []int64) (GuardianNoticeAudience, error)
	PublishNotice(ctx context.Context, notice GuardianNoticePublication) (GuardianNoticePublished, error)
}
