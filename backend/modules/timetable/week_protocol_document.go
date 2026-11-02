package timetable

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The PDF of a closed weekly protocol (#4769). The owner writes every text:
// Timetable renders the document when the protocol is closed, by the group
// leadership or automatically, and stores the file; a download serves the
// stored file. The shapes are unnamed and match the other record reports,
// so the roots bind one Document Rendering record renderer for all of them.
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

// WeekProtocolRenderer renders the protocol's document as a PDF; the roots
// bind Document Rendering.
type WeekProtocolRenderer interface {
	RenderReport(doc WeekProtocolReportDocument, filenameBase string) (WeekProtocolReportFile, error)
}

// ShownDays are the days a protocol shows: Monday to Friday always, a
// weekend day only when it was recorded or noted. A day without a record
// carries only its date.
func (p WeekProtocol) ShownDays() []WeekProtocolDay {
	byDate := make(map[calendar.Date]WeekProtocolDay, len(p.Days))
	for _, day := range p.Days {
		byDate[day.Date] = day
	}
	days := make([]WeekProtocolDay, 0, 7)
	for offset := 0; offset < 7; offset++ {
		date := p.WeekStart.AddDays(offset)
		day, ok := byDate[date]
		if !ok && calendar.IsWeekend(date) {
			continue
		}
		day.Date = date
		days = append(days, day)
	}
	return days
}

// WeekProtocolDocument prints a closed protocol: one section per day with
// the attendance, the missing children, the blocks, the staff notices and
// the day's note, then the note for the week. Its date is the closing
// instant.
func WeekProtocolDocument(protocol WeekProtocol) WeekProtocolReportDocument {
	generated := time.Now()
	if protocol.ClosedAt != nil {
		generated = *protocol.ClosedAt
	}
	days := protocol.ShownDays()
	sections := make([]WeekProtocolReportSection, 0, len(days)+1)
	for _, day := range days {
		sections = append(sections, weekProtocolDaySection(day))
	}
	sections = append(sections, WeekProtocolReportSection{Title: "Woche", Cards: []WeekProtocolReportCard{{
		Title:  "Ergänzung für die Woche",
		Fields: []WeekProtocolReportField{{Label: "Text", Value: orDash(protocol.WeekNote)}},
	}}})
	return WeekProtocolReportDocument{
		Title:       "Wochenprotokoll",
		Subtitle:    fmt.Sprintf("%s · %s bis %s", protocol.GroupName, protocol.WeekStart.Format("02.01.2006"), protocol.WeekEnd.Format("02.01.2006")),
		GeneratedAt: generated,
		Filters:     []string{weekProtocolClosedLine(protocol)},
		Footer:      "Vertraulich: enthält Angaben zur Anwesenheit von Kindern",
		Sections:    sections,
	}
}

// WeekProtocolFilename is wochenprotokoll-<gruppe>-<montag>.
func WeekProtocolFilename(protocol WeekProtocol) string {
	return "wochenprotokoll-" + slugFilename(protocol.GroupName) + "-" + protocol.WeekStart.String()
}

func weekProtocolClosedLine(protocol WeekProtocol) string {
	if protocol.ClosedAt == nil {
		return "Nicht abgeschlossen"
	}
	at := protocol.ClosedAt.In(calendar.Berlin).Format("02.01.2006, 15:04") + " Uhr"
	if protocol.ClosedAutomatically {
		return "Automatisch abgeschlossen am " + at
	}
	return fmt.Sprintf("Abgeschlossen am %s von %s", at, protocol.ClosedByName)
}

func weekProtocolDaySection(day WeekProtocolDay) WeekProtocolReportSection {
	section := WeekProtocolReportSection{Title: germanWeekday(day.Date.Weekday()) + ", " + day.Date.Format("02.01.2006")}
	if day.Facts == nil {
		section.Cards = append(section.Cards, WeekProtocolReportCard{Title: "Tag", Fields: []WeekProtocolReportField{
			{Label: "Hinweis", Value: "Dieser Tag wurde nicht festgehalten."},
		}})
	} else {
		section.Cards = append(section.Cards, weekProtocolAttendanceCard(*day.Facts), weekProtocolBlockCard(*day.Facts))
		if len(day.Facts.Notices) > 0 {
			section.Cards = append(section.Cards, weekProtocolNoticeCard(*day.Facts))
		}
	}
	section.Cards = append(section.Cards, WeekProtocolReportCard{Title: "Ergänzung", Fields: []WeekProtocolReportField{{Label: "Text", Value: orDash(day.Note)}}})
	return section
}

func weekProtocolAttendanceCard(facts WeekProtocolDayFacts) WeekProtocolReportCard {
	if facts.Kind == WeekProtocolDayHoliday || facts.Kind == WeekProtocolDayClosing {
		return WeekProtocolReportCard{Title: weekProtocolDayKindLabel(facts.Kind), Fields: []WeekProtocolReportField{
			{Label: "Anwesend", Value: strconv.Itoa(facts.Present)},
		}}
	}
	card := WeekProtocolReportCard{Title: "Anwesenheit", Fields: []WeekProtocolReportField{
		{Label: "Anwesend", Value: strconv.Itoa(facts.Present)},
		{Label: "Krank", Value: strconv.Itoa(facts.Sick)},
		{Label: "Entschuldigt", Value: strconv.Itoa(facts.Excused)},
		{Label: "Klassenfahrt", Value: strconv.Itoa(facts.ClassTrip)},
		{Label: "Unentschuldigt", Value: strconv.Itoa(facts.Absent)},
	}}
	if len(facts.Missing) > 0 {
		block := WeekProtocolReportBlock{Title: "Fehlende Kinder"}
		for _, child := range facts.Missing {
			block.Fields = append(block.Fields, WeekProtocolReportField{Label: child.Name, Value: weekProtocolAbsenceLabel(child.Reason, child.Label)})
		}
		card.Blocks = []WeekProtocolReportBlock{block}
	}
	return card
}

// weekProtocolAbsenceLabel names a missing child's reason with the words of
// the Tagesauswertung's counters, so the counter and the child's line agree.
// The stored label of the day log stays the fallback for an unknown reason.
func weekProtocolAbsenceLabel(reason, stored string) string {
	switch reason {
	case WeekProtocolAbsenceSick:
		return "Krank"
	case WeekProtocolAbsenceExcused:
		return "Entschuldigt"
	case WeekProtocolAbsenceClassTrip:
		return "Klassenfahrt"
	case WeekProtocolAbsenceAbsent:
		return "Unentschuldigt"
	default:
		return stored
	}
}

func weekProtocolBlockCard(facts WeekProtocolDayFacts) WeekProtocolReportCard {
	card := WeekProtocolReportCard{Title: "Aktivitäten und Blöcke"}
	if len(facts.Blocks) == 0 {
		card.Fields = []WeekProtocolReportField{{Label: "Blöcke", Value: "Keine"}}
		return card
	}
	for _, block := range facts.Blocks {
		status := weekProtocolBlockStatusLabel(block.Status)
		if block.CancelReason != "" {
			status += ": " + block.CancelReason
		}
		card.Fields = append(card.Fields, WeekProtocolReportField{
			Label: fmt.Sprintf("%s–%s %s", block.StartTime, block.EndTime, block.Title), Value: status,
		})
	}
	return card
}

func weekProtocolNoticeCard(facts WeekProtocolDayFacts) WeekProtocolReportCard {
	card := WeekProtocolReportCard{Title: "Tagesinformationen"}
	for _, notice := range facts.Notices {
		card.Fields = append(card.Fields, WeekProtocolReportField{Label: notice.Title, Value: orDash(notice.Body)})
	}
	return card
}

// weekProtocolBlockStatusLabel names a block's state on the evening of its
// day, in the words the protocol page shows.
func weekProtocolBlockStatusLabel(status string) string {
	switch status {
	case InstanceStatusCompleted:
		return "Gelaufen"
	case InstanceStatusCancelled:
		return "Ausgefallen"
	case InstanceStatusActive:
		return "Nicht beendet"
	default:
		return "Nicht gestartet"
	}
}

func weekProtocolDayKindLabel(kind string) string {
	if kind == WeekProtocolDayHoliday {
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
