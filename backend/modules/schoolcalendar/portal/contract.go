package portal

import (
	"context"
	"errors"
	"time"

	appointmentcap "github.com/moto-nrw/project-phoenix/modules/appointments"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
)

type FeedCancellation = schoolcalendar.FeedCancellation
type FeedHistory = schoolcalendar.FeedHistory

type Date = appointmentcap.Date

const (
	EventSourceAppointment = "appointment"
	EventSourceTimetable   = "timetable"
	EventSourceShift       = "shift"
)

var (
	ErrInvalidRequest = errors.New("invalid calendar request")
	ErrForbidden      = errors.New("calendar access forbidden")
	ErrNotFound       = errors.New("calendar item not found")
	// ErrConflict signals that a concurrent lifecycle transition (cancel/delete)
	// raced the current operation — e.g. an edit that began before the appointment
	// was cancelled. Rendered as HTTP 409.
	ErrConflict = errors.New("calendar item changed concurrently")
)

type Service interface {
	ListMyStaffEvents(ctx context.Context, from, to Date) ([]Event, error)
	ListMyParentEvents(ctx context.Context, accountID int64, from, to Date) ([]Event, error)
	CreateStaffAppointment(ctx context.Context, req CreateAppointmentRequest) (*AppointmentDetail, error)
	GetStaffAppointmentDetail(ctx context.Context, appointmentID int64) (*AppointmentDetail, error)
	UpdateStaffAppointment(ctx context.Context, appointmentID int64, req UpdateAppointmentRequest) (*AppointmentDetail, error)
	CancelStaffAppointment(ctx context.Context, appointmentID int64) (*AppointmentDetail, error)
	DeleteStaffAppointment(ctx context.Context, appointmentID int64) error
	CancelStaffAppointmentOccurrence(ctx context.Context, appointmentID int64, occurrenceDate Date) error
	GetStaffAppointmentOverview(ctx context.Context, appointmentID int64) (*AppointmentOverview, error)
	GetParentAppointmentOverview(ctx context.Context, accountID, appointmentID int64) (*AppointmentOverview, error)
	StaffAppointmentICS(ctx context.Context, appointmentID int64) (filename, content string, err error)
	ParentAppointmentICS(ctx context.Context, accountID, appointmentID int64) (filename, content string, err error)
	ParentCalendarFeedURL(ctx context.Context, accountID int64) (httpsURL, webcalURL string, err error)
	RotateParentCalendarFeed(ctx context.Context, accountID int64) (httpsURL, webcalURL string, err error)
	ParentCalendarFeedByToken(ctx context.Context, token string) (filename, content string, err error)
	StaffCalendarFeedURL(ctx context.Context) (httpsURL, webcalURL string, err error)
	RotateStaffCalendarFeed(ctx context.Context) (httpsURL, webcalURL string, err error)
	StaffCalendarFeedByToken(ctx context.Context, token string) (filename, content string, err error)
	RespondToStaffInvitation(ctx context.Context, recipientID int64, status string) error
	RespondToParentInvitation(ctx context.Context, accountID, recipientID int64, status string) error
	RecipientOptions(ctx context.Context, query string, limit int) (*RecipientOptions, error)
}

type FeedCleanupService interface {
	CleanupExpiredFeedTombstones(ctx context.Context) (int, error)
}

type FullService interface {
	Service
	FeedCleanupService
	StaffCalDAVService
}

type StaffCalDAVService interface {
	StaffCalendarAccess(ctx context.Context) (StaffCalendarAccessInfo, error)
	RotateStaffCalendarAccess(ctx context.Context) (StaffCalendarAccessInfo, error)
	AuthenticateStaffCalDAV(ctx context.Context, username, appPassword string) (*StaffCalDAVCalendar, error)
}

type CalendarRecurrence struct {
	Frequency string
	Interval  int
	Weekdays  []string
	MonthDays []int
	Until     string
	Count     *int
}

type CalendarEvent struct {
	UID          string
	Summary      string
	Description  string
	Location     string
	StartDate    string
	EndDate      string
	StartClock   time.Time
	EndClock     time.Time
	AllDay       bool
	Cancelled    bool
	Sequence     int
	Stamp        time.Time
	LastModified time.Time
	Recurrence   *CalendarRecurrence
	ExDates      []string
}

type CalendarRenderer interface {
	RenderCalendar(context.Context, string, []CalendarEvent) (string, error)
	RenderCalendarObject(context.Context, CalendarEvent) (string, error)
}

type CalDAVPolicy interface {
	Enabled(context.Context) (bool, error)
	EnabledForTenant(context.Context, int64) (bool, error)
}

type StaffCalendarAccessInfo struct {
	URL       string
	WebcalURL string
	CalDAV    *StaffCalDAVCredentials
}

type StaffCalDAVCredentials struct {
	ServerURL   string
	Username    string
	AppPassword string
}

type StaffCalDAVCalendar struct {
	AccountID string
	TenantID  int64
	Revision  string
	Items     []StaffCalDAVItem
}

type StaffCalDAVItem struct {
	Name       string
	UID        string
	Content    []byte
	ETag       string
	ModifiedAt time.Time
}

type Event struct {
	ID               string    `json:"id"`
	Source           string    `json:"source"`
	AppointmentID    *string   `json:"appointment_id,omitempty"`
	OccurrenceDate   *string   `json:"occurrence_date,omitempty"`
	TimetableID      *string   `json:"timetable_id,omitempty"`
	StudentID        *string   `json:"student_id,omitempty"`
	StudentName      *string   `json:"student_name,omitempty"`
	TenantID         *string   `json:"tenant_id,omitempty"`
	SchoolName       *string   `json:"school_name,omitempty"`
	Title            string    `json:"title"`
	Description      *string   `json:"description,omitempty"`
	Location         *string   `json:"location,omitempty"`
	StartDate        string    `json:"start_date"`
	EndDate          string    `json:"end_date"`
	StartTime        string    `json:"start_time"`
	EndTime          string    `json:"end_time"`
	AllDay           bool      `json:"all_day"`
	Cancelled        bool      `json:"cancelled"`
	Recurring        bool      `json:"recurring"`
	DeliveryMode     *string   `json:"delivery_mode,omitempty"`
	ResponseStatus   *string   `json:"response_status,omitempty"`
	RecipientID      *string   `json:"recipient_id,omitempty"`
	OrganizerStaffID *string   `json:"organizer_staff_id,omitempty"`
	CanRespond       bool      `json:"can_respond"`
	CanEdit          bool      `json:"can_edit"`
	CanViewOverview  bool      `json:"can_view_overview"`
	ModifiedAt       time.Time `json:"-"`
}

type AppointmentDetail struct {
	Appointment *appointmentcap.Appointment            `json:"appointment"`
	Recurrence  *appointmentcap.RecurrenceRule         `json:"recurrence,omitempty"`
	Recipients  []*appointmentcap.AppointmentRecipient `json:"recipients"`
	Targets     []*appointmentcap.AppointmentTarget    `json:"targets"`
}

type CreateAppointmentRequest struct {
	Title              string              `json:"title"`
	Description        *string             `json:"description,omitempty"`
	Location           *string             `json:"location,omitempty"`
	StartDate          Date                `json:"start_date"`
	EndDate            Date                `json:"end_date"`
	StartTime          time.Time           `json:"start_time"`
	EndTime            time.Time           `json:"end_time"`
	AllDay             bool                `json:"all_day"`
	DeliveryMode       string              `json:"delivery_mode"`
	OverviewVisibility string              `json:"overview_visibility"`
	Recurrence         *RecurrenceRequest  `json:"recurrence,omitempty"`
	Targets            []AppointmentTarget `json:"targets"`
	SendEmail          bool                `json:"send_email"`
}

type UpdateAppointmentRequest struct {
	Title              string             `json:"title"`
	Description        *string            `json:"description,omitempty"`
	Location           *string            `json:"location,omitempty"`
	StartDate          Date               `json:"start_date"`
	EndDate            Date               `json:"end_date"`
	StartTime          time.Time          `json:"start_time"`
	EndTime            time.Time          `json:"end_time"`
	AllDay             bool               `json:"all_day"`
	OverviewVisibility string             `json:"overview_visibility"`
	Recurrence         *RecurrenceRequest `json:"recurrence,omitempty"`
	SendEmail          bool               `json:"send_email"`
	SendEmailSet       bool               `json:"-"`
}

type AppointmentOverview struct {
	AppointmentID      string                `json:"appointment_id"`
	DeliveryMode       string                `json:"delivery_mode"`
	OverviewVisibility string                `json:"overview_visibility"`
	Attendees          []AppointmentAttendee `json:"attendees"`
}

type AppointmentAttendee struct {
	RecipientID   string     `json:"recipient_id"`
	RecipientType string     `json:"recipient_type"`
	Name          string     `json:"name"`
	Status        string     `json:"status"`
	RespondedAt   *time.Time `json:"responded_at,omitempty"`
}

type RecurrenceRequest struct {
	Frequency       string   `json:"frequency"`
	IntervalCount   int      `json:"interval_count"`
	Weekdays        []string `json:"weekdays,omitempty"`
	MonthDays       []int    `json:"month_days,omitempty"`
	EndsOn          *Date    `json:"ends_on,omitempty"`
	OccurrenceCount *int     `json:"occurrence_count,omitempty"`
}

type AppointmentTarget struct {
	Type  string  `json:"type"`
	ID    *int64  `json:"id,omitempty"`
	Value *string `json:"value,omitempty"`
}

type RecipientOptions struct {
	Staff    []StaffOption   `json:"staff"`
	Parents  []ParentOption  `json:"parents"`
	Groups   []GroupOption   `json:"groups"`
	Classes  []string        `json:"classes"`
	Students []StudentOption `json:"students"`
}

type StaffOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ParentOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type GroupOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type StudentOption struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	SchoolClass string  `json:"school_class,omitempty"`
	GroupID     *string `json:"group_id,omitempty"`
}
