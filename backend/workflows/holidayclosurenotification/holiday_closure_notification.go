// Package holidayclosurenotification is the workflow that tells the parents
// of booked holiday weeks about a changed school closing day (#6442, #6457,
// ADR 0196). A change of a school closing day only proposes a notice. The
// school admin or the Träger-Büro confirms it with one click; both clicks
// write the same durable command in the school's transaction. The worker
// then processes the command in a Träger transaction of its own, where
// Enrollment finds the families and Delivery sends one mail per family.
// The workflow owns no data and no SQL: it orders the units of the owners
// and keeps the school's answers free of family data beyond one count.
package holidayclosurenotification

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

var (
	// ErrNoticeNotFound reports a notice outside the caller's school or
	// Träger.
	ErrNoticeNotFound = errors.New("holiday closure notice not found")
	// ErrNoticeChanged reports a click on a notice whose closing day or
	// affected bookings changed since the caller saw it; nothing was
	// confirmed, the caller reloads.
	ErrNoticeChanged = errors.New("the holiday closure notice changed")
)

// Range is a closing day range.
type Range struct {
	StartDate calendar.Date `json:"start_date"`
	EndDate   calendar.Date `json:"end_date"`
}

// Notice is what the school and the Träger-Büro see of a proposed or
// confirmed notice. Families counts the families a click would tell now,
// computed like the mails themselves; it is set while the notice waits for
// a click and holds no name, contact or child (decision of 10.11.2026).
type Notice struct {
	Token        string     `json:"token"`
	ID           int64      `json:"id"`
	SchoolID     int64      `json:"school_id"`
	SchoolName   string     `json:"school_name,omitempty"`
	ClosingDayID int64      `json:"closing_day_id"`
	Reason       string     `json:"reason"`
	Previous     *Range     `json:"previous"`
	Current      *Range     `json:"current"`
	Revision     int64      `json:"revision"`
	Status       string     `json:"status"`
	Changed      bool       `json:"changed"`
	CanNotify    bool       `json:"can_notify"`
	Families     int        `json:"families"`
	ConfirmedAt  *time.Time `json:"confirmed_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// TraegerNotice adds what the Träger-Büro sees of the affected bookings:
// the weeks with their changed days and the children.
type TraegerNotice struct {
	Notice
	Weeks    []enrollment.HolidayClosureWeek `json:"weeks"`
	Children int                             `json:"children"`
}

// Pending is the list "Eltern noch nicht informiert" of the Träger-Büro:
// the affected weeks and families of every notice that waits for a click,
// the token the click confirms, and every notice the Büro follows.
type Pending struct {
	enrollment.HolidayClosureImpact
	Notices        []TraegerNotice `json:"notices"`
	RetryableMails int             `json:"retryable_mails"`
	UnknownMails   int             `json:"unknown_mails"`
	mailKeys       []string
}

// NotifiedPending is the answer of the Büro's click on the whole list.
type NotifiedPending struct {
	RetriedMails int64 `json:"retried_mails"`
	Notices      int   `json:"notices"`
	Families     int   `json:"families"`
}

// Notifications is the workflow.
type Notifications interface {
	// SchoolNotices lists the notices of the school.
	SchoolNotices(ctx context.Context, schoolID int64) ([]Notice, error)
	// NotifyFromSchool confirms a notice of the school, or takes its
	// failed command up again, for accountID.
	NotifyFromSchool(ctx context.Context, schoolID, accountID, noticeID, revision int64, token string) (Notice, error)
	// PendingHolidayClosures lists the notices of the Träger's schools.
	PendingHolidayClosures(ctx context.Context, organizationID int64) (Pending, error)
	// NotifyFromTraeger confirms a notice of a current school of the
	// Träger, or takes its failed command up again, for accountID.
	NotifyFromTraeger(ctx context.Context, organizationID, accountID, noticeID, revision int64, token string) (Notice, error)
	// NotifyPendingHolidayClosures confirms every notice of the list the
	// Büro saw, each under its own operation; token is that list's.
	NotifyPendingHolidayClosures(ctx context.Context, organizationID, accountID int64, token string) (NotifiedPending, error)
	// ProcessDue processes the confirmed commands of every Träger and
	// follows their mails.
	ProcessDue(ctx context.Context) error
}
