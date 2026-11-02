package timetable

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Wochenprotokoll (#4769): je OGS-Gruppe und Woche (Montag bis Sonntag) ein
// Protokoll aus den Tagesdaten. Jeden Abend hält der Worker den Tag fest,
// denn das Kind kennt nur seine aktuelle Gruppe: nach einem Gruppenwechsel
// ließe sich ein vergangener Tag nicht mehr richtig zuordnen. Am
// Freitagabend wird das Protokoll zum Entwurf, den die Gruppenleitung je Tag
// und für die Woche ergänzt und abschließt. Ein Entwurf, den niemand
// abschließt, wird nach der Folgewoche mit den bis dahin eingetragenen
// Ergänzungen abgeschlossen. Eine Schule ohne feste Gruppen bekommt ein
// Protokoll für die ganze Schule.

// Reichweite eines Protokolls.
const (
	WeekProtocolScopeGroup  = "group"
	WeekProtocolScopeSchool = "school"
)

// Zustände eines Protokolls: es sammelt die Abende der Woche, ist ab
// Freitagabend ein Entwurf und danach abgeschlossen.
const (
	WeekProtocolStatusCollecting = "collecting"
	WeekProtocolStatusDraft      = "draft"
	WeekProtocolStatusClosed     = "closed"
)

// Art eines festgehaltenen Tages.
const (
	WeekProtocolDayCare    = "care"
	WeekProtocolDayHoliday = "holiday"
	WeekProtocolDayClosing = "closing"
)

// Grund, aus dem ein Kind an einem Tag fehlte, wie ihn die Tagesauswertung
// nennt.
const (
	WeekProtocolAbsenceSick      = "sick"
	WeekProtocolAbsenceExcused   = "excused"
	WeekProtocolAbsenceClassTrip = "class_trip"
	WeekProtocolAbsenceAbsent    = "absent"
)

// Längen der Freitexte, wie die Tabelle sie prüft.
const (
	WeekProtocolWeekNoteMaxLength = 4000
	WeekProtocolDayNoteMaxLength  = 2000
)

var (
	// ErrWeekProtocolNotFound meldet ein unbekanntes, fremdes oder für die
	// Person nicht sichtbares Protokoll.
	ErrWeekProtocolNotFound = errors.New("weekprotocol: protocol not found")
	// ErrWeekProtocolNotDraft meldet eine Ergänzung oder ein Abschließen
	// außerhalb des Entwurfs: das Protokoll sammelt noch oder ist
	// abgeschlossen.
	ErrWeekProtocolNotDraft = errors.New("weekprotocol: protocol is not a draft")
	// ErrWeekProtocolInvalid meldet fachlich unzulässige Ergänzungen.
	ErrWeekProtocolInvalid = errors.New("weekprotocol: invalid notes")
	// ErrWeekProtocolNotClosed meldet das PDF eines Protokolls, das noch
	// nicht abgeschlossen ist: erst das Abschließen legt es ab.
	ErrWeekProtocolNotClosed = errors.New("weekprotocol: protocol is not closed")
)

// WeekProtocol ist ein Protokoll mit seinen Tagen. Days ist in der Liste
// leer und im einzelnen Protokoll von Montag bis Sonntag sortiert; ein Tag,
// der weder festgehalten noch ergänzt wurde, fehlt.
type WeekProtocol struct {
	ID        int64
	Scope     string
	GroupID   *int64
	GroupName string
	WeekStart calendar.Date
	// WeekEnd is the Sunday of the week.
	WeekEnd             calendar.Date
	Status              string
	WeekNote            string
	DraftedAt           *time.Time
	ClosedAt            *time.Time
	ClosedByName        string
	ClosedAutomatically bool
	Days                []WeekProtocolDay
}

// WeekProtocolDay ist ein Tag des Protokolls. Facts ist nil, wenn der Abend
// nicht festgehalten wurde.
type WeekProtocolDay struct {
	Date       calendar.Date
	CapturedAt *time.Time
	Facts      *WeekProtocolDayFacts
	Note       string
}

// WeekProtocolDayFacts ist, was der Abend festhielt. Gesundheitsangaben
// gehören nicht dazu: von einem fehlenden Kind steht nur der Grund da, wie
// in der Tagesauswertung.
type WeekProtocolDayFacts struct {
	Kind      string                     `json:"kind"`
	Present   int                        `json:"present"`
	Sick      int                        `json:"sick"`
	Excused   int                        `json:"excused"`
	ClassTrip int                        `json:"class_trip"`
	Absent    int                        `json:"absent"`
	Missing   []WeekProtocolMissingChild `json:"missing"`
	Blocks    []WeekProtocolBlock        `json:"blocks"`
	Notices   []WeekProtocolNotice       `json:"notices"`
}

// WeekProtocolMissingChild ist ein Kind, das an dem Tag fehlte.
type WeekProtocolMissingChild struct {
	StudentID int64  `json:"student_id"`
	Name      string `json:"name"`
	Reason    string `json:"reason"`
	Label     string `json:"label"`
}

// WeekProtocolBlock ist ein Block des Tages mit seinem Stand am Abend.
type WeekProtocolBlock struct {
	Title        string `json:"title"`
	StartTime    string `json:"start_time"`
	EndTime      string `json:"end_time"`
	Status       string `json:"status"`
	CancelReason string `json:"cancel_reason,omitempty"`
}

// WeekProtocolNotice ist eine Tagesinformation, die an dem Tag galt.
type WeekProtocolNotice struct {
	Title string `json:"title"`
	Body  string `json:"body,omitempty"`
}

// WeekProtocolReach ist, welche Protokolle eine Person sieht: die Leitung
// alle, eine Fachkraft die ihrer Gruppen und das der ganzen Schule.
type WeekProtocolReach struct {
	Leadership bool
	Staff      bool
	GroupIDs   []int64
}

// Sees sagt, ob die Person das Protokoll sehen und ergänzen darf.
func (r WeekProtocolReach) Sees(p WeekProtocol) bool {
	if r.Leadership {
		return true
	}
	if !r.Staff {
		return false
	}
	if p.Scope == WeekProtocolScopeSchool {
		return true
	}
	if p.GroupID == nil {
		return false
	}
	for _, id := range r.GroupIDs {
		if id == *p.GroupID {
			return true
		}
	}
	return false
}

// WeekProtocolFilter grenzt die Liste ein; GroupID nil heißt alle
// sichtbaren Protokolle.
type WeekProtocolFilter struct {
	GroupID *int64
}

// WeekProtocolNotes sind die Ergänzungen der Gruppenleitung. Days enthält
// nur Tage der Woche; ein leerer Text löscht die Ergänzung des Tages.
type WeekProtocolNotes struct {
	WeekNote string
	Days     map[calendar.Date]string
}

// WeekProtocolTarget nennt das Protokoll, zu dem ein festgehaltener Tag
// gehört: eine OGS-Gruppe, oder die ganze Schule, wenn GroupID nil ist.
type WeekProtocolTarget struct {
	GroupID   *int64
	GroupName string
}

// WeekProtocolActor ist, wer ein Protokoll abschließt.
type WeekProtocolActor struct {
	AccountID int64
	Name      string
}

// WeekProtocols ist die Sicht der Fachkräfte und der Leitung auf die
// Protokolle der Schule im Kontext.
type WeekProtocols interface {
	ListWeekProtocols(ctx context.Context, reach WeekProtocolReach, filter WeekProtocolFilter) ([]WeekProtocol, error)
	OpenWeekProtocol(ctx context.Context, reach WeekProtocolReach, id int64) (WeekProtocol, error)
	SaveWeekProtocolNotes(ctx context.Context, reach WeekProtocolReach, id int64, notes WeekProtocolNotes) (WeekProtocol, error)
	// CloseWeekProtocol schließt den Entwurf ab und legt in derselben
	// Transaktion sein PDF ab, das renderer erzeugt.
	CloseWeekProtocol(ctx context.Context, reach WeekProtocolReach, id int64, actor WeekProtocolActor, renderer WeekProtocolRenderer) (WeekProtocol, error)
	// OpenWeekProtocolPDF liefert das abgelegte PDF eines abgeschlossenen
	// Protokolls mit dem Protokoll, unverändert seit dem Abschließen.
	OpenWeekProtocolPDF(ctx context.Context, reach WeekProtocolReach, id int64) (WeekProtocol, WeekProtocolReportFile, error)
}

// WeekProtocolGroup ist eine OGS-Gruppe, für die ein Protokoll geführt wird.
type WeekProtocolGroup struct {
	ID   int64
	Name string
}

// WeekProtocolGroupDay ist der Tag einer Gruppe, wie ihn die Tagesauswertung
// meldet: die Kinder je Ergebnis, die fehlenden mit ihrem Grund und alle
// Kinder der Gruppe an dem Tag.
type WeekProtocolGroupDay struct {
	GroupID    int64
	StudentIDs []int64
	Present    int
	Sick       int
	Excused    int
	ClassTrip  int
	Absent     int
	Missing    []WeekProtocolMissingChild
}

// WeekProtocolAttendance ist der Port auf die Tagesauswertung: die heutigen
// Ergebnisse der Gruppen, mit den Kindern in der Gruppe, zu der sie jetzt
// gehören, und für eine Schule ohne feste Gruppen (offene Betreuung) die
// Ergebnisse der ganzen Schule: jedes Kind des Tages genau einmal, mit oder
// ohne Gruppe (#5401). Er beantwortet nur den laufenden Tag, weil das Kind
// keine datierte Gruppenhistorie hat. Die Wurzel bindet ihn.
type WeekProtocolAttendance interface {
	GroupDayAttendance(ctx context.Context, day calendar.Date, groupIDs []int64) ([]WeekProtocolGroupDay, error)
	SchoolDayAttendance(ctx context.Context, day calendar.Date) (WeekProtocolGroupDay, error)
}

// WeekProtocolSources sind die Quellen des Abendlaufs außerhalb der eigenen
// Blöcke: die Tagesinformationen, der Schulkalender und die Ports, die die
// Wurzel an School Structure (Groups), die Tagesauswertung (Attendance),
// die Einstellung operations.group_mode (SchoolMode, true = ohne feste
// Gruppen) und Document Rendering (Renderer, das PDF eines automatisch
// abgeschlossenen Protokolls) bindet.
type WeekProtocolSources struct {
	Notices    StaffNotices
	Calendar   NonWorkingDayCalendar
	Groups     func(ctx context.Context) ([]WeekProtocolGroup, error)
	Attendance WeekProtocolAttendance
	SchoolMode func(ctx context.Context) (bool, error)
	Renderer   WeekProtocolRenderer
}

// WeekProtocolEveningResult zählt, was ein Abendlauf in einer Schule tat.
type WeekProtocolEveningResult struct {
	DaysCaptured  int
	Drafted       int
	AutoClosed    int
	SkippedReason string
}

// WeekProtocolCapability ist das Wochenprotokoll des Owners: die Sicht der
// Fachkräfte und der Leitung, der Abendlauf des Workers für die Schule im
// Kontext (den Tag festhalten, ab Freitagabend Entwurf, liegengebliebene
// Entwürfe nach der Folgewoche abschließen; ein wiederholter Lauf am selben
// Abend tut nichts doppelt) und das Löschen der Protokolle, deren Sonntag
// vor dem Fenster von retentionDays (gdpr.attendance_retention_days) liegt.
type WeekProtocolCapability interface {
	WeekProtocols
	RunWeekProtocolEvening(ctx context.Context, now time.Time, sources WeekProtocolSources) (WeekProtocolEveningResult, error)
	DeleteExpiredWeekProtocols(ctx context.Context, retentionDays int) (int, error)
}

// ErrWeekProtocolRetentionWindow verweigert ein Fenster, das nicht positiv
// ist: nichts wird gelöscht, der Lauf holt es nach, sobald die Einstellung
// stimmt.
var ErrWeekProtocolRetentionWindow = errors.New("week protocol retention window must be positive")
