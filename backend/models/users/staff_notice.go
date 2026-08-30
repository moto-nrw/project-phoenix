package users

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// Wichtigkeit einer Tagesinformation (spiegelt chk_staff_notices_priority).
// "important" ändert nur die Darstellung, nicht die Reichweite.
const (
	StaffNoticePriorityInfo      = "info"
	StaffNoticePriorityImportant = "important"
)

// ValidStaffNoticePriority meldet, ob p eine bekannte Wichtigkeit ist.
func ValidStaffNoticePriority(p string) bool {
	return p == StaffNoticePriorityInfo || p == StaffNoticePriorityImportant
}

// StaffNotice ist ein interner Hinweis der Leitung an das Team
// (Tagesinformation, #2180): schulweit sichtbar für alle Mitarbeitenden,
// geschrieben mit Adminrecht.
//
// Der Hinweis existiert EINMAL, nicht als Zeile pro Tag. Wann er gilt, sagen
// drei Felder, deren Bedeutung aus Stundenplan und Dienstplan übernommen ist:
// der Zeitraum (ValidFrom/ValidUntil), die Wochentage (ISO 1..7, leer = jeder
// Tag im Zeitraum) und das WeekPattern (0 = jede Woche, 1/2 = Woche A/B des
// Kalenderzeitraums). Ausgewertet wird beim Lesen — siehe AppliesOn.
type StaffNotice struct {
	base.Model `bun:"schema:users,table:staff_notices"`
	base.TenantModel
	Title    string `bun:"title,notnull" json:"title"`
	Body     string `bun:"body,notnull" json:"body"`
	Priority string `bun:"priority,notnull" json:"priority"`
	// ValidFrom/ValidUntil sind Kalendertage, keine Zeitpunkte. ValidUntil nil
	// heißt unbefristet.
	ValidFrom  timezone.Date  `bun:"valid_from,notnull,type:date" json:"valid_from"`
	ValidUntil *timezone.Date `bun:"valid_until,type:date" json:"valid_until,omitempty"`
	// Weekdays sind ISO-Wochentage (1=Montag … 7=Sonntag). Leer = jeder Tag im
	// Zeitraum.
	Weekdays []int16 `bun:"weekdays,array,notnull" json:"weekdays"`
	// WeekPattern: 0 = jede Woche, 1 = Woche A, 2 = Woche B (dieselbe Kodierung,
	// die ShouldMaterializeWeekPattern erwartet).
	WeekPattern             int   `bun:"week_pattern,notnull,default:0" json:"week_pattern"`
	RequiresAcknowledgement bool  `bun:"requires_acknowledgement,notnull" json:"requires_acknowledgement"`
	Active                  bool  `bun:"active,notnull" json:"active"`
	CreatedBy               int64 `bun:"created_by,notnull" json:"created_by"`
}

// ContainsWeekday meldet, ob der Hinweis an diesem ISO-Wochentag gilt. Ohne
// gesetzte Wochentage gilt er an jedem Tag des Zeitraums.
func (n *StaffNotice) ContainsWeekday(weekday int) bool {
	if len(n.Weekdays) == 0 {
		return true
	}
	for _, d := range n.Weekdays {
		if int(d) == weekday {
			return true
		}
	}
	return false
}

// AppliesOn meldet, ob der Hinweis an diesem Kalendertag gilt — ohne das
// Wochenmuster, das den Kalenderzeitraum braucht und deshalb im Service
// zusätzlich geprüft wird.
//
// Reine Ableitung aus vorhandenen Feldern, keine Entscheidung: das
// Wochenmuster, die Sichtbarkeit je Rolle und die Kenntnisnahme gehören in den
// Service.
func (n *StaffNotice) AppliesOn(date timezone.Date) bool {
	if !n.Active {
		return false
	}
	if date.Before(n.ValidFrom) {
		return false
	}
	if n.ValidUntil != nil && date.After(*n.ValidUntil) {
		return false
	}
	return n.ContainsWeekday(ISOWeekday(date))
}

// ISOWeekday übersetzt einen Kalendertag in den ISO-Wochentag (1=Montag …
// 7=Sonntag). Go zählt Sonntag als 0, die Wochentagslisten in Stundenplan und
// Dienstplan zählen ihn als 7.
func ISOWeekday(d timezone.Date) int {
	if wd := d.Weekday(); wd != time.Sunday {
		return int(wd)
	}
	return 7
}

// Validate prüft die Daten, die ohne Kontext prüfbar sind. Zeitraumbezogene
// Regeln (Wochenmuster braucht einen Wochenzyklus) liegen im Service.
func (n *StaffNotice) Validate() error {
	if strings.TrimSpace(n.Title) == "" {
		return errors.New("title is required")
	}
	if len([]rune(n.Title)) > 200 {
		return errors.New("title must not exceed 200 characters")
	}
	if !ValidStaffNoticePriority(n.Priority) {
		return errors.New("priority must be info or important")
	}
	if n.WeekPattern < 0 || n.WeekPattern > 2 {
		return errors.New("week pattern must be 0, 1 or 2")
	}
	if n.ValidUntil != nil && n.ValidUntil.Before(n.ValidFrom) {
		return errors.New("valid until must not be before valid from")
	}
	for _, d := range n.Weekdays {
		if d < 1 || d > 7 {
			return errors.New("weekdays must be ISO weekdays 1..7")
		}
	}
	return nil
}

// StaffNoticeAck ist die Kenntnisnahme einer Person für einen Hinweis. Sie gilt
// für den Hinweis, nicht für den einzelnen Tag: ein wiederkehrender Hinweis
// wird einmal bestätigt, nicht jeden Dienstag erneut.
type StaffNoticeAck struct {
	bun.BaseModel `bun:"table:users.staff_notice_acks,alias:sna"`
	base.TenantModel
	NoticeID       int64     `bun:"notice_id,pk" json:"notice_id"`
	AccountID      int64     `bun:"account_id,pk" json:"account_id"`
	AcknowledgedAt time.Time `bun:"acknowledged_at,nullzero,notnull,default:current_timestamp" json:"acknowledged_at"`
}

// StaffNoticeView ist die Sicht einer Person auf einen Hinweis: der Hinweis
// selbst plus die eigene Kenntnisnahme und (für die Leitung) wie viele sie
// schon bestätigt haben.
type StaffNoticeView struct {
	*StaffNotice
	AcknowledgedAt    *time.Time `json:"acknowledged_at,omitempty"`
	AcknowledgedCount int        `json:"acknowledged_count"`
}

// StaffNoticeRepository ist der mandantengebundene Datenzugriff für
// Tagesinformationen. Alle Methoden laufen in einer Mandanten-Transaktion.
type StaffNoticeRepository interface {
	Create(ctx context.Context, n *StaffNotice) error
	Update(ctx context.Context, n *StaffNotice) error
	FindByID(ctx context.Context, id int64) (*StaffNotice, error)
	Delete(ctx context.Context, id int64) error
	// List gibt die Hinweise des Mandanten zurück, neueste zuerst.
	// includeInactive steuert, ob abgeschaltete Zeilen erscheinen.
	List(ctx context.Context, includeInactive bool) ([]*StaffNotice, error)
	// ListValidOn gibt die aktiven Hinweise zurück, deren Zeitraum den Tag
	// enthält. Wochentag und Wochenmuster prüft der Service — die Datenbank
	// grenzt nur grob ein.
	ListValidOn(ctx context.Context, date timezone.Date) ([]*StaffNotice, error)
	// Acknowledge stempelt die Kenntnisnahme einer Person; ein zweiter Aufruf
	// ändert nichts.
	Acknowledge(ctx context.Context, noticeID, accountID int64) error
	// AcknowledgedAtFor gibt je Hinweis-Id den Zeitpunkt der eigenen
	// Kenntnisnahme zurück.
	AcknowledgedAtFor(ctx context.Context, accountID int64, noticeIDs []int64) (map[int64]time.Time, error)
	// AcknowledgedCounts gibt je Hinweis-Id die Zahl der Kenntnisnahmen zurück.
	AcknowledgedCounts(ctx context.Context, noticeIDs []int64) (map[int64]int, error)
}
