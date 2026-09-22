package timetable

import (
	"context"
	"errors"
	"time"
)

// Tagesinformationen (#2180): interne Hinweise der Leitung an das Team, die
// an bestimmten Tagen gelten. Timetable besitzt users.staff_notices und
// users.staff_notice_acks; der Dienst wertet einen Hinweis beim Lesen gegen
// das Datum aus und materialisiert nie Tageszeilen, damit hier keine zweite
// Recurrence-Engine entsteht. Das Wochenmuster (0 = jede Woche, 1 = Woche A,
// 2 = Woche B) ist dieselbe Kodierung wie im Stundenplan.

var (
	// ErrStaffNoticeNotFound meldet einen unbekannten oder fremden Hinweis.
	ErrStaffNoticeNotFound = errors.New("staffnotice: notice not found")
	// ErrStaffNoticeInvalid meldet fachlich unzulässige Eingaben.
	ErrStaffNoticeInvalid = errors.New("staffnotice: invalid notice")
)

// Wichtigkeit eines Hinweises.
const (
	StaffNoticePriorityInfo      = "info"
	StaffNoticePriorityImportant = "important"
)

// Zielgruppe eines Hinweises und die Leserarten der Portale (#2208): eine
// Betreuungskraft liest als StaffNoticeAudienceStaff, eine Lehrkraft als
// StaffNoticeAudienceLehrkraft; "all" erreicht beide und ist als Leserart
// unzulässig, weil es kein Portal "alle" gibt.
const (
	StaffNoticeAudienceAll       = "all"
	StaffNoticeAudienceStaff     = "staff"
	StaffNoticeAudienceLehrkraft = "lehrkraft"
)

// StaffNoticeUnknownAcknowledgerName steht in der Bestätigungsliste, wenn zum
// Konto keine aktive Person mehr gehört. Die Kenntnisnahme bleibt sichtbar:
// sie ist passiert.
const StaffNoticeUnknownAcknowledgerName = "Unbekannte Person"

// StaffNotice ist ein Hinweis. ValidFrom und ValidUntil sind Kalendertage im
// Format YYYY-MM-DD; ein leeres ValidUntil heißt unbefristet. Weekdays sind
// ISO-Wochentage (1 = Montag … 7 = Sonntag), leer = jeder Tag im Zeitraum.
type StaffNotice struct {
	ID                      int64
	TenantID                int64
	Title                   string
	Body                    string
	Priority                string
	Audience                string
	ValidFrom               string
	ValidUntil              string
	Weekdays                []int16
	WeekPattern             int
	RequiresAcknowledgement bool
	Active                  bool
	CreatedBy               int64
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

// StaffNoticeView ist ein Hinweis aus Sicht einer Person: mit dem Zeitpunkt
// der eigenen Kenntnisnahme und, für die Leitung, der Zahl der
// Kenntnisnahmen.
type StaffNoticeView struct {
	StaffNotice
	AcknowledgedAt    *time.Time
	AcknowledgedCount int
}

// StaffNoticeAcknowledger ist eine Zeile der Bestätigungsliste (#2208).
type StaffNoticeAcknowledger struct {
	AccountID      int64
	Name           string
	AcknowledgedAt time.Time
}

// StaffNoticeInput ist die Schreibform eines Hinweises. Priority leer heißt
// info, Audience leer heißt all. ValidFrom ist Pflicht; beide Daten sind
// Kalendertage im Format YYYY-MM-DD.
type StaffNoticeInput struct {
	Title                   string
	Body                    string
	Priority                string
	Audience                string
	ValidFrom               string
	ValidUntil              string
	Weekdays                []int16
	WeekPattern             int
	RequiresAcknowledgement bool
	Active                  bool
}

// StaffNotices ist die Capability der Tagesinformationen.
type StaffNotices interface {
	// ListStaffNotices gibt alle Hinweise des Mandanten zurück
	// (Leitungssicht), jeweils mit der Zahl der Kenntnisnahmen.
	ListStaffNotices(ctx context.Context, accountID int64, includeInactive bool) ([]StaffNoticeView, error)
	// StaffNoticesOn gibt die Hinweise zurück, die an diesem Kalendertag
	// (YYYY-MM-DD) für diese Leserart gelten. reader ist
	// StaffNoticeAudienceStaff oder StaffNoticeAudienceLehrkraft; der Aufrufer
	// leitet ihn aus der Route ab, nicht aus der Eingabe der Person. Eine
	// unbekannte Leserart erhält nichts.
	StaffNoticesOn(ctx context.Context, accountID int64, date string, reader string) ([]StaffNoticeView, error)
	FindStaffNotice(ctx context.Context, id int64) (StaffNotice, error)
	CreateStaffNotice(ctx context.Context, createdBy int64, in StaffNoticeInput) (StaffNotice, error)
	UpdateStaffNotice(ctx context.Context, id int64, in StaffNoticeInput) (StaffNotice, error)
	DeleteStaffNotice(ctx context.Context, id int64) error
	// AcknowledgeStaffNotice nimmt die Kenntnisnahme einer Person entgegen.
	// Sie gilt nur für Hinweise, die diese Leserart heute überhaupt erreichen.
	AcknowledgeStaffNotice(ctx context.Context, id, accountID int64, reader string) error
	// StaffNoticeAcknowledgers gibt der Leitung die Bestätigungsliste eines
	// Hinweises: wer wann zur Kenntnis genommen hat, neueste zuerst (#2208).
	StaffNoticeAcknowledgers(ctx context.Context, id int64) ([]StaffNoticeAcknowledger, error)
}
