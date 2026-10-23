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
	return DailyListExport{
		Title:    "Mittagessen – Tagesliste",
		Subtitle: "Tagesliste für den " + date,
		Filters:  []string{cutoffFilter(l.CutoffTime, l.CutoffLeadDays), fmt.Sprintf("%d Kinder", len(lines))},
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
	total := make([]string, len(columns))
	total[0] = "Summe"
	for index, day := range w.Days {
		for _, participant := range day.Participants {
			lines[rowOf[participant.StudentID]][index+2] = "✓"
		}
		total[index+2] = strconv.Itoa(len(day.Participants))
	}
	lines = append(lines, total)
	return DailyListExport{
		Title:    "Mittagessen – Wochenliste",
		Subtitle: "Wochenliste vom " + from + " bis " + to,
		Filters:  []string{cutoffFilter(w.CutoffTime, w.CutoffLeadDays), fmt.Sprintf("%d Kinder", len(participants))},
		Columns:  columns,
		Lines:    lines,
		Footer:   "Vertraulich behandeln und nach dem Küchendienst sicher entsorgen.",
		Filename: "mittagessen-woche-" + string(w.WeekStart),
	}, nil
}

var weekdayAbbreviations = [...]string{"Mo", "Di", "Mi", "Do", "Fr"}

func cutoffFilter(cutoffTime string, leadDays int) string {
	switch leadDays {
	case 0:
		return "Änderungsfrist: " + cutoffTime + " Uhr"
	case 1:
		return "Änderungsfrist: " + cutoffTime + " Uhr, 1 Werktag vorher"
	default:
		return fmt.Sprintf("Änderungsfrist: %s Uhr, %d Werktage vorher", cutoffTime, leadDays)
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
