package mealplan

import (
	"fmt"
	"sort"
	"strconv"
)

// DailyListExport is the kitchen's Tagesliste as an export renders it.
type DailyListExport struct {
	Title    string
	Subtitle string
	Filters  []string
	Columns  []DailyListExportColumn
	// Lines hold one value per column: the child as "Last, First" and its
	// class.
	Lines [][]string
	// Footer is stamped on every page.
	Footer string
	// Filename is the download name without its extension.
	Filename string
}

// DailyListExportColumn is one column of the Tagesliste.
type DailyListExportColumn struct {
	ID    string
	Label string
}

// Export lays out the Tagesliste of the day.
func (l DailyList) Export() (DailyListExport, error) {
	date, err := l.Date.German()
	if err != nil {
		return DailyListExport{}, err
	}
	lines := make([][]string, 0, len(l.Participants))
	for _, participant := range l.Participants {
		lines = append(lines, []string{participant.LastName + ", " + participant.FirstName, participant.SchoolClass})
	}
	filters := append(cutoffFilters(l.Cutoffs), fmt.Sprintf("%d Kinder", len(lines)))
	if l.Closed {
		filters = append(filters, closedDayNote)
	}
	return DailyListExport{
		Title:    "Mittagessen – Tagesliste",
		Subtitle: "Tagesliste für den " + date,
		Filters:  filters,
		Columns:  []DailyListExportColumn{{ID: "student_name", Label: "Kind"}, {ID: "student_class", Label: "Klasse"}},
		Lines:    lines,
		Footer:   "Vertraulich behandeln und nach dem Küchendienst sicher entsorgen.",
		Filename: "mittagessen-" + string(l.Date),
	}, nil
}

// Export lays out the Wochenliste: one line per child registered on at least
// one day, a mark per day, and the count of each day in the last line.
func (w WeeklyList) Export() (DailyListExport, error) {
	if len(w.Days) == 0 {
		return DailyListExport{}, fmt.Errorf("meal plan: weekly list without days")
	}
	from, err := w.Days[0].Date.German()
	if err != nil {
		return DailyListExport{}, err
	}
	to, err := w.Days[len(w.Days)-1].Date.German()
	if err != nil {
		return DailyListExport{}, err
	}
	columns := []DailyListExportColumn{{ID: "student_name", Label: "Kind"}, {ID: "student_class", Label: "Klasse"}}
	seen := make(map[int64]struct{})
	var participants []DailyParticipant
	for index, day := range w.Days {
		label, err := day.Date.German()
		if err != nil {
			return DailyListExport{}, err
		}
		columns = append(columns, DailyListExportColumn{ID: "day_" + strconv.Itoa(index+1), Label: weekdayAbbreviations[index] + " " + label[:6]})
		for _, participant := range day.Participants {
			if _, exists := seen[participant.StudentID]; !exists {
				seen[participant.StudentID] = struct{}{}
				participants = append(participants, participant)
			}
		}
	}
	SortDailyParticipants(participants)
	lines := make([][]string, 0, len(participants)+1)
	rowOf := make(map[int64]int, len(participants))
	for row, participant := range participants {
		rowOf[participant.StudentID] = row
		line := make([]string, len(columns))
		line[0] = participant.LastName + ", " + participant.FirstName
		line[1] = participant.SchoolClass
		lines = append(lines, line)
	}
	lines = append(lines, markWeekDays(w.Days, lines, rowOf, len(columns)))
	return DailyListExport{
		Title:    "Mittagessen – Wochenliste",
		Subtitle: "Wochenliste vom " + from + " bis " + to,
		Filters:  append(cutoffFilters(w.Cutoffs), fmt.Sprintf("%d Kinder", len(participants))),
		Columns:  columns,
		Lines:    lines,
		Footer:   "Vertraulich behandeln und nach dem Küchendienst sicher entsorgen.",
		Filename: "mittagessen-woche-" + string(w.WeekStart),
	}, nil
}

// markWeekDays ticks each child's days and returns the line with the count
// of each day; a closed day counts no meal.
func markWeekDays(days []DailyList, lines [][]string, rowOf map[int64]int, width int) []string {
	total := make([]string, width)
	total[0] = "Summe"
	for index, day := range days {
		for _, participant := range day.Participants {
			lines[rowOf[participant.StudentID]][index+2] = "✓"
		}
		total[index+2] = strconv.Itoa(len(day.Participants))
		if day.Closed {
			total[index+2] = closedDayTotal
		}
	}
	return total
}

var weekdayAbbreviations = [...]string{"Mo", "Di", "Mi", "Do", "Fr"}

const (
	closedDayNote  = "Kein Essen: Feiertag oder Schließtag"
	closedDayTotal = "kein Essen"
)

var weekdayNames = [...]string{"Montag", "Dienstag", "Mittwoch", "Donnerstag", "Freitag"}

// cutoffFilters describe the change cutoff and, when parents may cancel
// later, the cancellation cutoff.
func cutoffFilters(cutoffs Cutoffs) []string {
	filters := []string{"Änderungsfrist: " + cutoffText(cutoffs.CutoffTime, cutoffs.CutoffLeadDays, cutoffs.CutoffWeekday)}
	if cutoffs.CancellationCutoffTime != "" {
		filters = append(filters, "Abmeldefrist: "+cutoffText(cutoffs.CancellationCutoffTime, cutoffs.CancellationCutoffLeadDays, 0))
	}
	return filters
}

func cutoffText(cutoffTime string, leadDays int, weekday Weekday) string {
	if weekday >= Monday && weekday <= Friday {
		return fmt.Sprintf("%s Uhr, %s der Vorwoche", cutoffTime, weekdayNames[weekday-1])
	}
	switch leadDays {
	case 0:
		return cutoffTime + " Uhr"
	case 1:
		return cutoffTime + " Uhr, 1 Werktag vorher"
	default:
		return fmt.Sprintf("%s Uhr, %d Werktage vorher", cutoffTime, leadDays)
	}
}

// SortDailyParticipants orders the kitchen list by class, last name, first
// name and student ID.
func SortDailyParticipants(participants []DailyParticipant) {
	sort.SliceStable(participants, func(left, right int) bool {
		a, b := participants[left], participants[right]
		if a.SchoolClass != b.SchoolClass {
			return a.SchoolClass < b.SchoolClass
		}
		if a.LastName != b.LastName {
			return a.LastName < b.LastName
		}
		if a.FirstName != b.FirstName {
			return a.FirstName < b.FirstName
		}
		return a.StudentID < b.StudentID
	})
}
