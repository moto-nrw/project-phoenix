package mealplan

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// deliveryColumns are the fixed columns of the delivery list (#3873): the
// caterer takes the file into its own planning, so they never change with
// a school's settings.
var deliveryColumns = []ListExportColumn{
	{ID: "date", Label: "Datum"},
	{ID: "school", Label: "Schule"},
	{ID: "delivery_address", Label: "Lieferadresse"},
	{ID: "school_class", Label: "Klasse oder Gruppe"},
	{ID: "meal_count", Label: "Anzahl"},
	{ID: "late_changes", Label: "Nachmeldung"},
}

// allClasses names the line of a school that orders a total only.
const allClasses = "Alle"

// DeliveryLines lays out the days of the week for the kitchen (#3873): one
// line per day and class when the school orders by class, otherwise one
// line per day. A closed day or a day never ordered has no line. Nachmeldung
// is the net change after the deadline, empty without one.
func (w OrderWeek) DeliveryLines(school, deliveryAddress string) ([][]string, error) {
	var lines [][]string
	for _, day := range w.Days {
		if day.Closed || day.Status == OrderUnordered {
			continue
		}
		date, err := day.Date.German()
		if err != nil {
			return nil, err
		}
		for _, count := range deliveryCounts(day, w.Detail == "by_class") {
			lines = append(lines, []string{date, cellText(school), cellText(deliveryAddress), cellText(count.group), strconv.Itoa(count.meals), signedChange(count.change)})
		}
	}
	return lines, nil
}

type deliveryCount struct {
	group  string
	meals  int
	change int
}

// deliveryCounts are the meals of a day per class, or in one line.
func deliveryCounts(day OrderDay, byClass bool) []deliveryCount {
	if !byClass {
		change := 0
		for _, late := range day.Changes {
			change += late.Delta
		}
		return []deliveryCount{{group: allClasses, meals: day.Total, change: change}}
	}
	counts := make(map[string]*deliveryCount)
	entry := func(group string) *deliveryCount {
		if counts[group] == nil {
			counts[group] = &deliveryCount{group: group}
		}
		return counts[group]
	}
	for _, class := range day.Classes {
		entry(class.SchoolClass).meals = class.Count
	}
	for _, late := range day.Changes {
		entry(late.Child.SchoolClass).change += late.Delta
	}
	result := make([]deliveryCount, 0, len(counts))
	for _, count := range counts {
		if count.group == "" {
			count.group = "–"
		}
		result = append(result, *count)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].group < result[j].group })
	return result
}

// cellText keeps a school's own text from reading as a formula in a
// spreadsheet: a leading =, +, - or @ gets an apostrophe.
func cellText(text string) string {
	if text != "" && strings.ContainsRune("=+-@\t\r", rune(text[0])) {
		return "'" + text
	}
	return text
}

func signedChange(change int) string {
	switch {
	case change > 0:
		return "+" + strconv.Itoa(change)
	case change < 0:
		return "−" + strconv.Itoa(-change)
	default:
		return ""
	}
}

// DeliveryExport is the delivery list of [from, to] for one school or for
// every school a caterer supplies: the same document for both (#3873).
func DeliveryExport(from, to Date, lines [][]string) (ListExport, error) {
	start, err := from.German()
	if err != nil {
		return ListExport{}, err
	}
	end, err := to.German()
	if err != nil {
		return ListExport{}, err
	}
	meals := 0
	for _, line := range lines {
		count, _ := strconv.Atoi(line[4])
		meals += count
	}
	return ListExport{
		Title:    "Mittagessen – Lieferliste",
		Subtitle: fmt.Sprintf("Vom %s bis %s", start, end),
		Filters:  []string{fmt.Sprintf("%d Essen", meals), "Nachmeldung: Änderung nach der Frist"},
		Columns:  deliveryColumns,
		Lines:    lines,
		Footer:   "Bestellt über moto.",
		Filename: fmt.Sprintf("lieferliste-%s-bis-%s", from, to),
	}, nil
}

// CSV renders the export as semicolon-separated values with a byte order
// mark, the way German spreadsheet programs open them.
func (e ListExport) CSV() ([]byte, error) {
	var buffer bytes.Buffer
	buffer.WriteString("\uFEFF")
	writer := csv.NewWriter(&buffer)
	writer.Comma = ';'
	header := make([]string, 0, len(e.Columns))
	for _, column := range e.Columns {
		header = append(header, column.Label)
	}
	if err := writer.Write(header); err != nil {
		return nil, err
	}
	if err := writer.WriteAll(e.Lines); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
