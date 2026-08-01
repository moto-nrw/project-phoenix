package planexport

import (
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/services/listexport"
)

// weekdayLabels are the printed day columns in weekday order.
var weekdayLabels = [7]string{
	"Montag", "Dienstag", "Mittwoch", "Donnerstag", "Freitag", "Samstag", "Sonntag",
}

const (
	// workweekDays is the ordinary sheet: Monday–Friday. Saturday and Sunday
	// are dropped whenever nothing is planned on them, because two
	// permanently empty columns cost a quarter of the page width.
	workweekDays = 5
	// fullWeekDays is the widened sheet. A staff shift may be planned on any
	// ISO weekday (1–7) and the overview counts weekend shifts, so a sheet
	// fixed at Monday–Friday would silently drop a Dienst somebody is
	// actually rostered for — the one failure a printed plan must not have.
	fullWeekDays = 7
)

// week is one printed sheet: its Monday and the days it covers — five, or
// seven once the exported range plans a weekend.
type week struct {
	monday timezone.Date
	days   []timezone.Date
}

// last is the final printed day of the week: Friday, or Sunday on a widened
// sheet.
func (w week) last() timezone.Date {
	return w.days[len(w.days)-1]
}

// label reads "KW 31 · 27.07.–31.07.2026", matching how the planning screens
// title a week so the printout and the screen name the same thing.
func (w week) label() string {
	_, isoWeek := w.monday.BerlinMidnight().ISOWeek()
	return fmt.Sprintf("KW %d · %s–%s",
		isoWeek,
		w.days[0].Format("02.01."),
		w.last().Format("02.01.2006"),
	)
}

// mondayOf is the Monday of the calendar week containing d.
func mondayOf(d timezone.Date) timezone.Date {
	offset := (int(d.Weekday()) + 6) % 7 // Monday = 0 … Sunday = 6
	return d.AddDays(-offset)
}

// expandWeeks widens [from, to] to whole Monday–Sunday weeks and returns
// them in order. A wall plan is printed by the week, so a request for a
// Wednesday prints that Wednesday's whole week. The weeks come back full so
// the callers load the weekend too; narrowWeeks then drops it again unless
// the loaded data actually uses it.
func expandWeeks(from, to timezone.Date) ([]week, error) {
	first := mondayOf(from)
	last := mondayOf(to)
	count := first.DaysUntil(last)/7 + 1
	if count > maxExportWeeks {
		return nil, ErrRangeTooLarge
	}

	weeks := make([]week, 0, count)
	for monday := first; !monday.After(last); monday = monday.AddDays(7) {
		days := make([]timezone.Date, fullWeekDays)
		for i := range days {
			days[i] = monday.AddDays(i)
		}
		weeks = append(weeks, week{monday: monday, days: days})
	}
	return weeks, nil
}

// narrowWeeks cuts every sheet back to Monday–Friday unless something is
// planned on a Saturday or Sunday anywhere in the range.
//
// Columns are document-level and cannot change per sheet, so a single
// weekend entry widens every week of the export. That is the right trade:
// the alternative is either printing two empty columns on every ordinary
// week, or dropping a rostered weekend Dienst from the plan without a trace.
func narrowWeeks(weeks []week, planned map[timezone.Date]bool) []week {
	for _, w := range weeks {
		for _, day := range w.days[workweekDays:] {
			if planned[day] {
				return weeks
			}
		}
	}

	narrowed := make([]week, 0, len(weeks))
	for _, w := range weeks {
		narrowed = append(narrowed, week{monday: w.monday, days: w.days[:workweekDays]})
	}
	return narrowed
}

// columnsFor builds the document's column set. A single-week export puts the
// concrete dates into the day headers ("Montag, 27.07.") because there is
// exactly one week they can refer to; a multi-week export keeps the headers
// generic and dates each week in its group heading instead, since columns are
// document-level and cannot change per sheet.
func columnsFor(rowLabel string, weeks []week) []listexport.Column {
	dayCount := workweekDays
	if len(weeks) > 0 {
		dayCount = len(weeks[0].days)
	}

	columns := make([]listexport.Column, 0, dayCount+1)
	columns = append(columns, listexport.Column{ID: listexport.ColumnPlanRowLabel, Label: rowLabel})
	for i, id := range listexport.PlanDayColumns[:dayCount] {
		label := weekdayLabels[i]
		if len(weeks) == 1 {
			label = fmt.Sprintf("%s, %s", label, weeks[0].days[i].Format("02.01."))
		}
		columns = append(columns, listexport.Column{ID: id, Label: label})
	}
	return columns
}

// dayCells is one printed row: the label plus the five day cells, each a
// stack of styled lines.
//
// The styles carry the whole legibility of the sheet. Every entry opens with
// a strong line (the window it happens in), its content follows in normal
// weight, and anything secondary — a room, a reason, a coverage gap — is
// muted. Printed in one uniform weight, a plan cell is a block of grey text
// in which a Dienst and an Angebot look identical; that was the first
// version, and it could not be read.
type dayCells struct {
	label string
	// count is how many of days are printed — the sheet's day columns. The
	// array stays full-week so a caller can fill by weekday index without
	// knowing the width.
	count int
	days  [fullWeekDays][]listexport.Line
}

// newDayCells starts a row sized to the sheet the week describes.
func newDayCells(label string, w week) dayCells {
	return dayCells{label: label, count: len(w.days)}
}

// strong, normal and muted build the three ranks of line a cell uses.
func strong(text string) listexport.Line {
	return listexport.Line{Text: text, Style: listexport.LineStrong}
}

func normal(text string) listexport.Line {
	return listexport.Line{Text: text, Style: listexport.LineNormal}
}

func muted(text string) listexport.Line {
	return listexport.Line{Text: text, Style: listexport.LineMuted}
}

// accented is a strong line that also opens a colour bar. An empty colour
// degrades to a plain strong line, so a Schichtart or Kategorie without a
// stored colour costs the bar and nothing else.
func accented(text, color string) listexport.Line {
	return listexport.Line{Text: text, Style: listexport.LineStrong, Accent: color}
}

// toRow turns the collected lines into a listexport row. Empty days print an
// em dash rather than blank space, so a reader can tell "nothing planned"
// from "the export lost it".
func (c dayCells) toRow() listexport.Row {
	values := map[listexport.ColumnID]string{
		listexport.ColumnPlanRowLabel: listexport.StyledCell([]listexport.Line{strong(c.label)}),
	}
	for i, id := range listexport.PlanDayColumns[:c.count] {
		if len(c.days[i]) == 0 {
			values[id] = listexport.StyledCell([]listexport.Line{muted("—")})
			continue
		}
		values[id] = listexport.StyledCell(c.days[i])
	}
	return listexport.Row{Values: values}
}

// timeRange formats a wall-clock window the way both plans display it.
func timeRange(start, end time.Time) string {
	return fmt.Sprintf("%s–%s", start.Format(clockLayout), end.Format(clockLayout))
}

const clockLayout = "15:04"

// distinctRoom returns the room name unless it merely repeats the title.
// Rooms are routinely named after what happens in them, and "Mensa · Mensa"
// is noise on a sheet whose whole point is being read from across a room.
func distinctRoom(title, room string) string {
	if strings.EqualFold(strings.TrimSpace(title), strings.TrimSpace(room)) {
		return ""
	}
	return room
}

// shortName is "Kessener, F." — the form used inside cells that list several
// people, where full first names would make the column tall enough to push a
// week onto a second page. Row labels keep the full name.
func shortName(firstName, lastName string) string {
	lastName = strings.TrimSpace(lastName)
	firstName = strings.TrimSpace(firstName)
	if lastName == "" {
		if firstName == "" {
			return "Unbekannt"
		}
		return firstName
	}
	if firstName == "" {
		return lastName
	}
	return fmt.Sprintf("%s, %s.", lastName, string([]rune(firstName)[:1]))
}

// fullName is "Kessener, Franziska" — the row label form.
func fullName(firstName, lastName string) string {
	firstName = strings.TrimSpace(firstName)
	lastName = strings.TrimSpace(lastName)
	switch {
	case lastName == "" && firstName == "":
		return "Unbekannt"
	case lastName == "":
		return firstName
	case firstName == "":
		return lastName
	default:
		return lastName + ", " + firstName
	}
}
