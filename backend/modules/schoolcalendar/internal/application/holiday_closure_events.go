package application

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// HolidayClosureEventStore persists the proposals and commands of ADR 0196.
type HolidayClosureEventStore interface {
	SchoolHolidayClosureNotices(ctx context.Context, today calendar.Date) ([]schoolcalendar.HolidayClosureNotice, error)
	TraegerHolidayClosureNotices(ctx context.Context, organizationID int64, today calendar.Date) ([]schoolcalendar.HolidayClosureNotice, error)
	ConfirmHolidayClosureNotice(ctx context.Context, proposalID, revision int64, actor schoolcalendar.HolidayClosureActor, today calendar.Date) (schoolcalendar.HolidayClosureNotice, error)
	DueHolidayClosureTraeger(ctx context.Context) ([]int64, error)
	LeaseHolidayClosureEvent(ctx context.Context, organizationID int64, leaseFor time.Duration) (schoolcalendar.HolidayClosureEvent, bool, error)
	LockLeasedHolidayClosureEvent(ctx context.Context, organizationID, eventID int64, leaseToken string) error
	HolidayClosureCalendar(ctx context.Context, organizationID, schoolID int64, from, to calendar.Date) (schoolcalendar.HolidayClosureCalendar, error)
	HolidayClosureEventSuperseded(ctx context.Context, organizationID, eventID int64) (bool, error)
	FinishHolidayClosureEvent(ctx context.Context, organizationID, eventID int64, leaseToken string, outcome schoolcalendar.HolidayClosureOutcome) error
	WaitingHolidayClosureEvents(ctx context.Context, organizationID int64) ([]schoolcalendar.HolidayClosureEvent, error)
	SettleHolidayClosureEvent(ctx context.Context, organizationID, eventID int64, state string) error
	RetireHolidayClosureEvents(ctx context.Context, organizationID int64) (int64, error)
}

// HolidayClosureEvents serves the proposal and the worker side.
type HolidayClosureEvents struct {
	HolidayClosureEventStore
	today func() calendar.Date
}

var (
	_ schoolcalendar.HolidayClosureProposals  = (*HolidayClosureEvents)(nil)
	_ schoolcalendar.HolidayClosureEventQueue = (*HolidayClosureEvents)(nil)
)

// NewHolidayClosureEvents builds the notices over store; today defaults to
// the Berlin day.
func NewHolidayClosureEvents(store HolidayClosureEventStore, today func() calendar.Date) *HolidayClosureEvents {
	if store == nil {
		panic("school calendar: the holiday closure event store is required")
	}
	if today == nil {
		today = calendar.TodayDate
	}
	return &HolidayClosureEvents{HolidayClosureEventStore: store, today: today}
}

// SchoolHolidayClosureNotices lists the notices of the school.
func (h *HolidayClosureEvents) SchoolHolidayClosureNotices(ctx context.Context) ([]schoolcalendar.HolidayClosureNotice, error) {
	return h.HolidayClosureEventStore.SchoolHolidayClosureNotices(ctx, h.today())
}

// TraegerHolidayClosureNotices lists the notices of the Träger's schools.
func (h *HolidayClosureEvents) TraegerHolidayClosureNotices(ctx context.Context, organizationID int64) ([]schoolcalendar.HolidayClosureNotice, error) {
	if organizationID <= 0 {
		return nil, errTraegerRequired
	}
	return h.HolidayClosureEventStore.TraegerHolidayClosureNotices(ctx, organizationID, h.today())
}

// ConfirmHolidayClosureNotice confirms the proposal or takes its failed
// command up again.
func (h *HolidayClosureEvents) ConfirmHolidayClosureNotice(ctx context.Context, proposalID, revision int64, actor schoolcalendar.HolidayClosureActor) (schoolcalendar.HolidayClosureNotice, error) {
	if proposalID <= 0 {
		return schoolcalendar.HolidayClosureNotice{}, schoolcalendar.ErrHolidayClosureNoticeNotFound
	}
	if actor.Channel != schoolcalendar.HolidayClosureChannelSchool && actor.Channel != schoolcalendar.HolidayClosureChannelTraeger {
		return schoolcalendar.HolidayClosureNotice{}, errors.New("school calendar: the channel of the click is required")
	}
	return h.HolidayClosureEventStore.ConfirmHolidayClosureNotice(ctx, proposalID, revision, actor, h.today())
}
