package mealplan

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ChildMeals is the number of ordered meals of one child in a window.
type ChildMeals struct {
	Child OrderChild
	Meals int
}

// MealsPerChild counts the ordered meals of each child over the ordered days
// of the window, every change applied (#3873). Open and unordered days do not
// count: nothing is ordered for them. Sorted like the kitchen list.
func (w OrderWeek) MealsPerChild() []ChildMeals {
	counts := make(map[int64]*ChildMeals)
	for _, day := range w.Days {
		if !day.Status.Billed() {
			continue
		}
		for _, child := range day.Children {
			entry, ok := counts[child.StudentID]
			if !ok {
				entry = &ChildMeals{Child: child}
				counts[child.StudentID] = entry
			}
			entry.Child.SchoolClass = child.SchoolClass
			entry.Meals++
		}
	}
	result := make([]ChildMeals, 0, len(counts))
	for _, entry := range counts {
		result = append(result, *entry)
	}
	sort.Slice(result, func(i, j int) bool {
		a, b := result[i].Child, result[j].Child
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
	return result
}

var germanMonths = []string{"Januar", "Februar", "März", "April", "Mai", "Juni", "Juli", "August", "September", "Oktober", "November", "Dezember"}

// MonthExport lays out the monthly overview per child: the basis to check a
// meal flat rate or a bill. month is the first day of the month.
func (w OrderWeek) MonthExport(month Date) (ListExport, error) {
	first, err := calendar.ParseDate(string(month))
	if err != nil {
		return ListExport{}, err
	}
	title := germanMonths[first.Month()-1] + " " + strconv.Itoa(first.Year())
	rows := w.MealsPerChild()
	lines := make([][]string, 0, len(rows))
	total := 0
	for _, row := range rows {
		lines = append(lines, []string{childName(row.Child), row.Child.SchoolClass, strconv.Itoa(row.Meals)})
		total += row.Meals
	}
	return ListExport{
		Title:    "Mittagessen – Monatsübersicht",
		Subtitle: "Bestellte Essen im " + title,
		Filters:  []string{"Mit allen Nachmeldungen", fmt.Sprintf("%d Kinder", len(rows)), fmt.Sprintf("%d Essen", total)},
		Columns:  []ListExportColumn{{ID: "student_name", Label: "Kind"}, {ID: "student_class", Label: "Klasse"}, {ID: "meal_count", Label: "Essen"}},
		Lines:    lines,
		Footer:   "Vertraulich behandeln.",
		Filename: "mittagessen-" + first.Format("2006-01"),
	}, nil
}

// childName is the child as lists show it; a child the school no longer
// knows shows its number.
func childName(child OrderChild) string {
	name := strings.TrimSpace(child.LastName + ", " + child.FirstName)
	if name == "," {
		return "Kind " + strconv.FormatInt(child.StudentID, 10)
	}
	return name
}

// MonthWindow is the first and the last day of the month of a date.
func MonthWindow(date Date) (Date, Date, error) {
	parsed, err := calendar.ParseDate(string(date))
	if err != nil {
		return "", "", err
	}
	first := calendar.NewDate(parsed.Year(), parsed.Month(), 1)
	last := calendar.DateFromTime(time.Date(parsed.Year(), parsed.Month()+1, 1, 12, 0, 0, 0, calendar.Berlin)).AddDays(-1)
	return Date(first.String()), Date(last.String()), nil
}
