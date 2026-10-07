package planexport

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/services/listexport"
)

// The hours sheet (#3819) is the list the schools keep by hand next to the
// roster: per person and week the planned hours of each Schichtart, their
// sum, and that sum against the contractual target. The figures are the
// Dienstplan's weekly summaries, read through WeeklyHoursReader, so the sheet
// and the Dienstplan row header cannot disagree.

// WeeklySummary is one staff member's planned minutes in one calendar week,
// against the contractual target when one resolves. Weekend shifts of the
// week are included.
type WeeklySummary struct {
	StaffID int64
	// WeekStart is the Monday of the summarized week.
	WeekStart      Date
	PlannedMinutes int
	// TargetMinutes and DeltaMinutes are nil when no target resolves.
	TargetMinutes *int
	DeltaMinutes  *int
	// ByShiftType splits PlannedMinutes by Schichtart.
	ByShiftType []ShiftTypeMinutes
}

// ShiftTypeMinutes is the planned minutes of one Schichtart; ShiftTypeID is
// nil for shifts without one.
type ShiftTypeMinutes struct {
	ShiftTypeID *int64
	Minutes     int
}

// WeeklyHours is the staff (in display order) and their weekly summaries.
type WeeklyHours struct {
	Staff     []*StaffMember
	Summaries []WeeklySummary
}

// WeeklyHoursReader supplies the weekly figures for whole Monday–Sunday
// weeks in [from, to]. The Dienstplan owner implements it and hands it in
// with the request.
type WeeklyHoursReader interface {
	WeeklyHours(ctx context.Context, from, to Date) (WeeklyHours, error)
}

const (
	hoursColumnUntyped listexport.ColumnID = "plan_hours_untyped"
	hoursColumnTotal   listexport.ColumnID = "plan_hours_total"
	hoursColumnTarget  listexport.ColumnID = "plan_hours_target"
	hoursColumnDelta   listexport.ColumnID = "plan_hours_delta"

	hoursLabelUntyped     = "Ohne Schichtart"
	hoursLabelUnknownType = "Unbekannte Schichtart"
	// hoursEmpty matches the empty-cell glyph of the other plan sheets.
	hoursEmpty = "—"
)

func hoursTypeColumn(typeID int64) listexport.ColumnID {
	return listexport.ColumnID("plan_hours_type_" + strconv.FormatInt(typeID, 10))
}

// hoursColumn is one Schichtart column of the sheet; typeID 0 is the column
// for shifts without a Schichtart.
type hoursColumn struct {
	typeID int64
	id     listexport.ColumnID
	label  string
}

// ExportDienstplanHours renders the hours sheet. The sheet names its weeks
// Monday to Friday like the Dienstplan screen, while the figures cover the
// whole week, weekend shifts included.
func (s *service) ExportDienstplanHours(ctx context.Context, params Params, reader WeeklyHoursReader) (listexport.File, error) {
	window, err := params.validate(TemplatesForHours)
	if err != nil {
		return listexport.File{}, err
	}
	if reader == nil || s.deps.Renderer == nil {
		return listexport.File{}, errors.New("plan export service is not fully wired")
	}
	weeks, err := expandWeeks(window.from, window.to)
	if err != nil {
		return listexport.File{}, err
	}
	hours, err := reader.WeeklyHours(ctx, dayKey(weeks[0].days[0]), dayKey(weeks[len(weeks)-1].last()))
	if err != nil {
		return listexport.File{}, fmt.Errorf("load weekly hours: %w", err)
	}
	weeks = narrowWeeks(weeks, nil)

	doc := hoursDocument(hours, s.shiftTypes(ctx), weeks)
	s.getLogger().Info("dienstplan hours export rendered",
		"from", weeks[0].days[0].String(),
		"to", weeks[len(weeks)-1].last().String(),
		"format", string(params.Format),
		"week_count", len(weeks),
	)
	return s.deps.Renderer.Render(doc, params.Format, filename("Stundenuebersicht", weeks))
}

// hoursDocument renders the weekly summaries as one row per
// staff member and week. Columns are document-level, so the sheet carries
// every Schichtart planned anywhere in the range — sorted by name, the shifts
// without one last — and a week that does not use one prints a dash there.
func hoursDocument(hours WeeklyHours, shiftTypes map[int64]shiftTypeInfo, weeks []week) listexport.Document {
	typeColumns := hoursTypeColumns(hours.Summaries, shiftTypes)

	return listexport.Document{
		Title:       "Stundenübersicht",
		Subtitle:    rangeSubtitle(weeks),
		GeneratedAt: time.Now(),
		Filters: []string{
			"Geplante Stunden ohne Pausen, ausgefallene Dienste zählen nicht",
			"Soll aus dem Arbeitszeitmodell",
		},
		Columns: hoursColumns(typeColumns),
		Rows:    hoursRows(hours.Staff, hours.Summaries, weeks, typeColumns),
		Footer:  confidentialityNote,
	}
}

func hoursColumns(typeColumns []hoursColumn) []listexport.Column {
	columns := make([]listexport.Column, 0, len(typeColumns)+4)
	columns = append(columns, listexport.Column{ID: listexport.ColumnPlanRowLabel, Label: "Mitarbeitende"})
	for _, column := range typeColumns {
		columns = append(columns, listexport.Column{ID: column.id, Label: column.label})
	}
	return append(columns,
		listexport.Column{ID: hoursColumnTotal, Label: "Gesamt"},
		listexport.Column{ID: hoursColumnTarget, Label: "Soll"},
		listexport.Column{ID: hoursColumnDelta, Label: "Differenz"},
	)
}

// hoursRows lists, per week, the staff with a summary in the overview's
// staff order. A week without any prints one explicit row, because a
// silently omitted week is indistinguishable from a broken export.
func hoursRows(staff []*StaffMember, summaries []WeeklySummary, weeks []week, typeColumns []hoursColumn) []listexport.Row {
	byWeek := make(map[Date]map[int64]WeeklySummary)
	for _, summary := range summaries {
		if byWeek[summary.WeekStart] == nil {
			byWeek[summary.WeekStart] = make(map[int64]WeeklySummary)
		}
		byWeek[summary.WeekStart][summary.StaffID] = summary
	}

	rows := make([]listexport.Row, 0, len(weeks)*(len(staff)+1))
	for _, w := range weeks {
		if len(weeks) > 1 {
			rows = append(rows, listexport.Row{GroupTitle: w.label()})
		}
		weekRows := hoursWeekRows(staff, byWeek[dayKey(w.monday)], typeColumns)
		if len(weekRows) == 0 {
			weekRows = []listexport.Row{{Values: map[listexport.ColumnID]string{
				listexport.ColumnPlanRowLabel: "Keine Dienste in dieser Woche",
			}}}
		}
		rows = append(rows, weekRows...)
	}
	return rows
}

func hoursWeekRows(staff []*StaffMember, weekSummaries map[int64]WeeklySummary, typeColumns []hoursColumn) []listexport.Row {
	rows := make([]listexport.Row, 0, len(weekSummaries))
	for _, member := range staff {
		if member == nil {
			continue
		}
		if summary, ok := weekSummaries[member.ID]; ok {
			rows = append(rows, hoursRow(memberFullName(member), summary, typeColumns))
		}
	}
	return rows
}

// hoursTypeColumns collects the Schichtarten planned in the range.
func hoursTypeColumns(summaries []WeeklySummary, shiftTypes map[int64]shiftTypeInfo) []hoursColumn {
	seen := make(map[int64]bool)
	for _, summary := range summaries {
		for _, entry := range summary.ByShiftType {
			seen[derefID(entry.ShiftTypeID)] = true
		}
	}
	columns := make([]hoursColumn, 0, len(seen))
	for typeID := range seen {
		column := hoursColumn{typeID: typeID, id: hoursColumnUntyped, label: hoursLabelUntyped}
		if typeID != 0 {
			column.id = hoursTypeColumn(typeID)
			column.label = strings.TrimSpace(shiftTypes[typeID].name)
			if column.label == "" {
				column.label = hoursLabelUnknownType
			}
		}
		columns = append(columns, column)
	}
	sort.SliceStable(columns, func(i, j int) bool {
		if (columns[i].typeID == 0) != (columns[j].typeID == 0) {
			return columns[j].typeID == 0
		}
		left, right := strings.ToLower(columns[i].label), strings.ToLower(columns[j].label)
		if left != right {
			return left < right
		}
		return columns[i].typeID < columns[j].typeID
	})
	return columns
}

func hoursRow(name string, summary WeeklySummary, typeColumns []hoursColumn) listexport.Row {
	minutesByType := make(map[int64]int, len(summary.ByShiftType))
	for _, entry := range summary.ByShiftType {
		minutesByType[derefID(entry.ShiftTypeID)] += entry.Minutes
	}
	values := map[listexport.ColumnID]string{
		listexport.ColumnPlanRowLabel: listexport.StyledCell([]listexport.Line{strong(name)}),
		hoursColumnTotal:              listexport.StyledCell([]listexport.Line{strong(formatHours(summary.PlannedMinutes))}),
		hoursColumnTarget:             hoursEmpty,
		hoursColumnDelta:              hoursEmpty,
	}
	for _, column := range typeColumns {
		values[column.id] = hoursEmpty
		if minutes, ok := minutesByType[column.typeID]; ok && minutes > 0 {
			values[column.id] = formatHours(minutes)
		}
	}
	if summary.TargetMinutes != nil {
		values[hoursColumnTarget] = formatHours(*summary.TargetMinutes)
	}
	if summary.DeltaMinutes != nil {
		values[hoursColumnDelta] = formatDeltaHours(*summary.DeltaMinutes)
	}
	return listexport.Row{Values: values}
}

// formatHours prints minutes as hours the way the Dienstplan screen does:
// German decimal comma, at most two decimals ("20,25 h", "40 h").
func formatHours(minutes int) string {
	hours := math.Round(float64(minutes)/60*100) / 100
	return strings.ReplaceAll(strconv.FormatFloat(hours, 'f', -1, 64), ".", ",") + " h"
}

// formatDeltaHours signs the difference to the target ("+2 h", "−0,5 h",
// "±0 h"), with a real minus sign as on the screen.
func formatDeltaHours(minutes int) string {
	switch {
	case minutes == 0:
		return "±0 h"
	case minutes > 0:
		return "+" + formatHours(minutes)
	default:
		return fmt.Sprintf("−%s", formatHours(-minutes))
	}
}
