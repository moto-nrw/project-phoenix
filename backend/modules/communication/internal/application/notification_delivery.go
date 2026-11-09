package application

import (
	"context"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

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

// NotificationSender is the producer's port.
type NotificationSender interface {
	communication.NotificationSender
}

// NotificationPreferences only narrows the producer's candidate recipients.
type NotificationOptIns interface {
	FilterOptedIn(context.Context, string, []int64) ([]int64, error)
}

type NotificationPreferences interface {
	communication.NotificationPreferences
}

// ParentDeliveryPolicy is the exact stateless policy used by parent producers.
type ParentDeliveryPolicy interface {
	DefaultLocale() string
	NormalizeLocale(string) string
	MotoLogoURL(string) string
	SchoolLogoURL(string, string) string
	ParentMessageCopy(string) (string, string)
	ParentAnnouncementCopy(string, string) (string, string)
	IsNotificationsDisabled(error) bool
	IsOutsideActiveWindow(error) bool
}

// NotificationErrors is the staff producer's narrower error-classification port.
type NotificationErrors interface {
	IsNotificationsDisabled(error) bool
	IsOutsideActiveWindow(error) bool
}

type notificationSender func(context.Context, communication.NotificationEvent) error

func (send notificationSender) Notify(ctx context.Context, event communication.NotificationEvent) error {
	return send(ctx, event)
}

// NewNotificationSender binds a public callback while preserving absent delivery.
func NewNotificationSender(send func(context.Context, communication.NotificationEvent) error) NotificationSender {
	if send == nil {
		return nil
	}
	return notificationSender(send)
}

// Public preserves every selector when crossing the application boundary.
func (event NotificationEvent) Public() communication.NotificationEvent {
	return communication.NotificationEvent{
		Type:           event.Type,
		IdempotencyKey: event.IdempotencyKey,
		RelatedType:    event.RelatedType,
		RelatedID:      event.RelatedID,
		Audience: communication.NotificationAudience{
			TenantID:           event.Audience.TenantID,
			Scope:              event.Audience.Scope,
			GuardianAccountID:  event.Audience.GuardianAccountID,
			GuardianAccountIDs: event.Audience.GuardianAccountIDs,
			ActiveGroupID:      event.Audience.ActiveGroupID,
			StaffAccountIDs:    event.Audience.StaffAccountIDs,
			StudentIDs:         event.Audience.StudentIDs,
		},
		Priority:       event.Priority,
		Title:          event.Title,
		Body:           event.Body,
		DeepLink:       event.DeepLink,
		SchoolDeepLink: event.SchoolDeepLink,
		Data:           event.Data,
		Portal:         event.Portal,
	}
}

// Stored notification protocol values, independent of routing and copy policy.
const (
	TypeParentMessage           = communication.TypeParentMessage
	TypeParentAnnouncement      = communication.TypeParentAnnouncement
	TypeParentCareCancelled     = communication.TypeParentCareCancelled
	TypeStaffMessage            = communication.TypeStaffMessage
	TypeStaffSurvey             = communication.TypeStaffSurvey
	ParentAnnouncementPublished = communication.ParentAnnouncementPublished
	ParentPollPublished         = communication.ParentPollPublished
	ParentPollReminder          = communication.ParentPollReminder
	ParentAnnouncementReminder  = communication.ParentAnnouncementReminder
	ParentCareCancelled         = communication.ParentCareCancelled
	PriorityNormal              = communication.PriorityNormal
	PriorityHigh                = communication.PriorityHigh
	ScopeGuardian               = communication.ScopeGuardian
	ScopeStaff                  = communication.ScopeStaff
)

// LocaleNormalizer is the renderer's read-only language normalization port.
type LocaleNormalizer interface{ NormalizeLocale(string) string }

const TypeParentRequestDecided = communication.TypeParentRequestDecided

// notifyEach sends the events in order as consecutive Notify calls and
// reports how many the router accepted before the first error.
func (s *service) notifyEach(ctx context.Context, events []communication.NotificationEvent) (int, error) {
	if s.series.Notify != nil {
		return s.series.Notify(ctx, events)
	}
	for i, event := range events {
		if err := s.notifier.Notify(ctx, event); err != nil {
			return i, err
		}
	}
	return len(events), nil
}

// notifyEachDurably sends the events in order as consecutive NotifyDurably
// calls and sums the receipts of those accepted before the first error.
func (s *service) notifyEachDurably(ctx context.Context, events []communication.NotificationEvent) (int, error) {
	receipts, err := s.series.NotifyDurably(ctx, events)
	accepted := 0
	for _, count := range receipts {
		accepted += count
	}
	return accepted, err
}

// localeGroups splits the push audience by portal language, in a fixed
// language order so that every run sends the variants in the same order.
func localeGroups(accountIDs []int64, localeByAccount map[int64]string) ([]string, map[string][]int64) {
	groups := make(map[string][]int64)
	var locales []string
	for _, accountID := range accountIDs {
		locale := localeByAccount[accountID]
		if _, ok := groups[locale]; !ok {
			locales = append(locales, locale)
		}
		groups[locale] = append(groups[locale], accountID)
	}
	slices.Sort(locales)
	return locales, groups
}
