package mealplan

import "fmt"

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
		Filters:  []string{"Änderungsfrist: " + l.CutoffTime + " Uhr", fmt.Sprintf("%d Kinder", len(lines))},
		Columns:  []DailyListExportColumn{{ID: "student_name", Label: "Kind"}, {ID: "student_class", Label: "Klasse"}},
		Lines:    lines,
		Footer:   "Vertraulich behandeln und nach dem Küchendienst sicher entsorgen.",
		Filename: "mittagessen-" + string(l.Date),
	}, nil
}
