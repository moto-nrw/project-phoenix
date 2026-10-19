package mealplan

import (
	"fmt"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ListExport is a list of the meal plan as an export renders it: the
// kitchen's Tagesliste or the monthly overview per child.
type ListExport struct {
	Title    string
	Subtitle string
	Filters  []string
	Columns  []ListExportColumn
	// Lines hold one value per column: the child as "Last, First" and its
	// class.
	Lines [][]string
	// Footer is stamped on every page.
	Footer string
	// Filename is the download name without its extension.
	Filename string
}

// ListExportColumn is one column of a list export.
type ListExportColumn struct {
	ID    string
	Label string
}

// Export lays out the Tagesliste of the day.
func (l DailyList) Export() (ListExport, error) {
	date, err := l.Date.German()
	if err != nil {
		return ListExport{}, err
	}
	lines := make([][]string, 0, len(l.Participants))
	for _, participant := range l.Participants {
		lines = append(lines, []string{participant.LastName + ", " + participant.FirstName, participant.SchoolClass})
	}
	return ListExport{
		Title:    "Mittagessen – Tagesliste",
		Subtitle: "Tagesliste für den " + date,
		Filters:  []string{"Frist: " + l.Deadline.In(calendar.Berlin).Format("02.01.2006, 15:04") + " Uhr", fmt.Sprintf("%d Kinder", len(lines))},
		Columns:  []ListExportColumn{{ID: "student_name", Label: "Kind"}, {ID: "student_class", Label: "Klasse"}},
		Lines:    lines,
		Footer:   "Vertraulich behandeln und nach dem Küchendienst sicher entsorgen.",
		Filename: "mittagessen-" + string(l.Date),
	}, nil
}
