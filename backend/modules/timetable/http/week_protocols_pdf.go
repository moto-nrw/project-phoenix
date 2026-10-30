package timetablehttp

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The PDF of a closed protocol is described in the same unnamed shapes as
// the Erklärung report and the proofs of Care Plan, so the root binds one
// Document Rendering record renderer for all of them.
type (
	// WeekProtocolReportField is one label/value line; PNG stays empty.
	WeekProtocolReportField = struct {
		Label, Value string
		PNG          []byte
	}
	// WeekProtocolReportBlock is an indented block inside a card.
	WeekProtocolReportBlock = struct {
		Title  string
		Fields []WeekProtocolReportField
	}
	// WeekProtocolReportCard is one card: a heading, its fields and blocks.
	WeekProtocolReportCard = struct {
		Title  string
		Fields []WeekProtocolReportField
		Blocks []WeekProtocolReportBlock
	}
	// WeekProtocolReportSection is a titled group of cards.
	WeekProtocolReportSection = struct {
		Title string
		Cards []WeekProtocolReportCard
	}
	// WeekProtocolReportDocument is the whole document.
	WeekProtocolReportDocument = struct {
		Title       string
		Subtitle    string
		GeneratedAt time.Time
		Filters     []string
		Footer      string
		Sections    []WeekProtocolReportSection
	}
	// WeekProtocolReportFile is the rendered file.
	WeekProtocolReportFile = struct {
		Data        []byte
		ContentType string
		Filename    string
	}
)

// WeekProtocolRenderer renders the protocol's document as a PDF.
type WeekProtocolRenderer interface {
	RenderReport(doc WeekProtocolReportDocument, filenameBase string) (WeekProtocolReportFile, error)
}

// weekProtocolDocument prints a closed protocol: one section per day with
// the attendance, the missing children, the blocks, the staff notices and
// the day's note, then the note for the week. Its date is the closing
// instant, so every download is the same document.
func weekProtocolDocument(protocol timetable.WeekProtocol) WeekProtocolReportDocument {
	generated := time.Now()
	if protocol.ClosedAt != nil {
		generated = *protocol.ClosedAt
	}
	days := weekProtocolDaysResponse(protocol)
	groups := make([]WeekProtocolReportSection, 0, len(days)+1)
	for _, day := range days {
		groups = append(groups, weekProtocolDayGroup(day))
	}
	groups = append(groups, WeekProtocolReportSection{Title: "Woche", Cards: []WeekProtocolReportCard{{
		Title:  "Ergänzung für die Woche",
		Fields: []WeekProtocolReportField{{Label: "Text", Value: orDash(protocol.WeekNote)}},
	}}})
	return WeekProtocolReportDocument{
		Title:       "Wochenprotokoll",
		Subtitle:    fmt.Sprintf("%s · %s bis %s", protocol.GroupName, protocol.WeekStart.Format("02.01.2006"), protocol.WeekEnd.Format("02.01.2006")),
		GeneratedAt: generated,
		Filters:     []string{weekProtocolClosedLine(protocol)},
		Footer:      "Vertraulich: enthält Angaben zur Anwesenheit von Kindern",
		Sections:    groups,
	}
}

func weekProtocolClosedLine(protocol timetable.WeekProtocol) string {
	if protocol.ClosedAt == nil {
		return "Nicht abgeschlossen"
	}
	at := protocol.ClosedAt.In(calendar.Berlin).Format("02.01.2006, 15:04") + " Uhr"
	if protocol.ClosedAutomatically {
		return "Automatisch abgeschlossen am " + at
	}
	return fmt.Sprintf("Abgeschlossen am %s von %s", at, protocol.ClosedByName)
}

func weekProtocolDayGroup(day weekProtocolDayResponse) WeekProtocolReportSection {
	date, _ := calendar.ParseDate(day.Date)
	group := WeekProtocolReportSection{Title: germanWeekday(date.Weekday()) + ", " + date.Format("02.01.2006")}
	if !day.Captured {
		group.Cards = append(group.Cards, WeekProtocolReportCard{Title: "Tag", Fields: []WeekProtocolReportField{
			{Label: "Hinweis", Value: "Dieser Tag wurde nicht festgehalten."},
		}})
	} else {
		group.Cards = append(group.Cards, weekProtocolAttendanceRecord(day), weekProtocolBlockRecord(day))
		if len(day.Notices) > 0 {
			group.Cards = append(group.Cards, weekProtocolNoticeRecord(day))
		}
	}
	group.Cards = append(group.Cards, WeekProtocolReportCard{Title: "Ergänzung", Fields: []WeekProtocolReportField{{Label: "Text", Value: orDash(day.Note)}}})
	return group
}

func weekProtocolAttendanceRecord(day weekProtocolDayResponse) WeekProtocolReportCard {
	if day.Kind == timetable.WeekProtocolDayHoliday || day.Kind == timetable.WeekProtocolDayClosing {
		return WeekProtocolReportCard{Title: weekProtocolDayKindLabel(day.Kind), Fields: []WeekProtocolReportField{
			{Label: "Anwesend", Value: strconv.Itoa(day.Present)},
		}}
	}
	record := WeekProtocolReportCard{Title: "Anwesenheit", Fields: []WeekProtocolReportField{
		{Label: "Anwesend", Value: strconv.Itoa(day.Present)},
		{Label: "Krank", Value: strconv.Itoa(day.Sick)},
		{Label: "Entschuldigt", Value: strconv.Itoa(day.Excused)},
		{Label: "Klassenfahrt", Value: strconv.Itoa(day.ClassTrip)},
		{Label: "Unentschuldigt", Value: strconv.Itoa(day.Absent)},
	}}
	if len(day.Missing) > 0 {
		sub := WeekProtocolReportBlock{Title: "Fehlende Kinder"}
		for _, child := range day.Missing {
			sub.Fields = append(sub.Fields, WeekProtocolReportField{Label: child.Name, Value: weekProtocolAbsenceLabel(child.Reason, child.Label)})
		}
		record.Blocks = []WeekProtocolReportBlock{sub}
	}
	return record
}

// weekProtocolAbsenceLabel names a missing child's reason with the words of
// the Tagesauswertung's counters, so the counter and the child's line agree.
// The stored label of the day log stays the fallback for an unknown reason.
func weekProtocolAbsenceLabel(reason, stored string) string {
	switch reason {
	case timetable.WeekProtocolAbsenceSick:
		return "Krank"
	case timetable.WeekProtocolAbsenceExcused:
		return "Entschuldigt"
	case timetable.WeekProtocolAbsenceClassTrip:
		return "Klassenfahrt"
	case timetable.WeekProtocolAbsenceAbsent:
		return "Unentschuldigt"
	default:
		return stored
	}
}

func weekProtocolBlockRecord(day weekProtocolDayResponse) WeekProtocolReportCard {
	record := WeekProtocolReportCard{Title: "Aktivitäten und Blöcke"}
	if len(day.Blocks) == 0 {
		record.Fields = []WeekProtocolReportField{{Label: "Blöcke", Value: "Keine"}}
		return record
	}
	for _, block := range day.Blocks {
		status := WeekProtocolBlockStatusLabel(block.Status)
		if block.CancelReason != "" {
			status += ": " + block.CancelReason
		}
		record.Fields = append(record.Fields, WeekProtocolReportField{
			Label: fmt.Sprintf("%s–%s %s", block.StartTime, block.EndTime, block.Title), Value: status,
		})
	}
	return record
}

func weekProtocolNoticeRecord(day weekProtocolDayResponse) WeekProtocolReportCard {
	record := WeekProtocolReportCard{Title: "Tagesinformationen"}
	for _, notice := range day.Notices {
		record.Fields = append(record.Fields, WeekProtocolReportField{Label: notice.Title, Value: orDash(notice.Body)})
	}
	return record
}

// WeekProtocolBlockStatusLabel names a block's state on the evening of its
// day, in the words the protocol prints.
func WeekProtocolBlockStatusLabel(status string) string {
	switch status {
	case timetable.InstanceStatusCompleted:
		return "Gelaufen"
	case timetable.InstanceStatusCancelled:
		return "Ausgefallen"
	case timetable.InstanceStatusActive:
		return "Nicht beendet"
	default:
		return "Nicht gestartet"
	}
}

func weekProtocolDayKindLabel(kind string) string {
	if kind == timetable.WeekProtocolDayHoliday {
		return "Feiertag"
	}
	return "Schließtag"
}

func germanWeekday(day time.Weekday) string {
	return [...]string{"Sonntag", "Montag", "Dienstag", "Mittwoch", "Donnerstag", "Freitag", "Samstag"}[day]
}

func orDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "–"
	}
	return value
}

// weekProtocolFilename is wochenprotokoll-<gruppe>-<montag>.
func weekProtocolFilename(protocol timetable.WeekProtocol) string {
	return "wochenprotokoll-" + slugFilename(protocol.GroupName) + "-" + protocol.WeekStart.String()
}

func slugFilename(value string) string {
	replacer := strings.NewReplacer("ä", "ae", "ö", "oe", "ü", "ue", "ß", "ss", "Ä", "ae", "Ö", "oe", "Ü", "ue")
	value = strings.ToLower(replacer.Replace(strings.TrimSpace(value)))
	var b strings.Builder
	dash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
			continue
		}
		if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	slug := strings.TrimSuffix(b.String(), "-")
	if slug == "" {
		return "gruppe"
	}
	return slug
}
