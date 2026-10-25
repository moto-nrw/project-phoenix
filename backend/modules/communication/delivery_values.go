package communication

import "context"

// NotificationAudience retains every authorization and portal selector.
type NotificationAudience struct {
	TenantID           int64
	Scope              string
	GuardianAccountID  int64
	GuardianAccountIDs []int64
	ActiveGroupID      string
	StaffAccountIDs    []int64
	StudentIDs         []int64
}

// NotificationEvent is the complete producer input to the existing router.
type NotificationEvent struct {
	Type           string
	IdempotencyKey string
	RelatedType    string
	RelatedID      int64
	Audience       NotificationAudience
	Priority       string
	Title          string
	Body           string
	DeepLink       string
	SchoolDeepLink string
	Data           map[string]string
	Portal         string
}

// Notification protocol vocabulary is independent of dispatch policy.
const (
	TypeParentMessage           = "parent_message"
	TypeParentAnnouncement      = "parent_announcement"
	TypeParentCareCancelled     = "parent_care_cancelled"
	TypeStaffMessage            = "staff_message"
	TypeStaffSurvey             = "staff_survey"
	ParentAnnouncementPublished = "announcement"
	ParentPollPublished         = "poll"
	ParentPollReminder          = "poll_reminder"
	ParentAnnouncementReminder  = "announcement_reminder"
	ParentCareCancelled         = "care_cancelled"
	PriorityNormal              = "normal"
	PriorityHigh                = "high"
	ScopeGuardian               = "guardian"
	ScopeStaff                  = "staff"
	TypeParentRequestDecided    = "parent_request_decided"
)

// NotificationSender is the public event callback used by owner bindings.
type NotificationSender interface {
	Notify(context.Context, NotificationEvent) error
}

// NotificationPreferences describes only the producer's consent selectors.
type NotificationPreferences interface {
	FilterOptedIn(context.Context, string, []int64) ([]int64, error)
	FilterNotOptedOut(context.Context, string, []int64) ([]int64, error)
}
