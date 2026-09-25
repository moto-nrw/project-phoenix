// Package lists is the Document Rendering contract for list exports (#3356):
// the document a consumer fills (title, filter labels, columns, rows), the
// vocabulary of the child-list presets and columns, and the Renderer that
// turns it into a PDF, DOCX or XLSX file. The renderer itself stays in
// services/listexport; its types appear here under the contract's names, so
// a consumer never reaches the renderer package and a rendered file is
// byte-for-byte what the renderer produced before.
package lists

import "github.com/moto-nrw/project-phoenix/services/listexport"

type (
	// Document is one list: header lines, columns and rows.
	Document = listexport.Document
	// Column is one printed column: its id and header label.
	Column = listexport.Column
	// ColumnID names a column and keys a row's values.
	ColumnID = listexport.ColumnID
	// Row is one printed row; GroupTitle opens a new section.
	Row = listexport.Row
	// Preset names a child list's default column set.
	Preset = listexport.Preset
	// Format is the output format.
	Format = listexport.Format
	// File is a rendered document.
	File = listexport.File
)

// Renderer renders a list document. Rendering neither stores the file nor
// decides who may read it; the caller owns both.
type Renderer interface {
	Render(doc Document, format Format, filenameBase string) (File, error)
}

// The renderer the contract names satisfies it.
var _ Renderer = (*listexport.RendererService)(nil)

// NewRenderer returns the Document Rendering list renderer.
func NewRenderer() Renderer {
	return listexport.NewService()
}

const (
	FormatPDF  = listexport.FormatPDF
	FormatDOCX = listexport.FormatDOCX
	FormatXLSX = listexport.FormatXLSX
)

// Presets of the child lists.
const (
	PresetOGSWeekly          = listexport.PresetOGSWeekly
	PresetOGSCompact         = listexport.PresetOGSCompact
	PresetClassRoster        = listexport.PresetClassRoster
	PresetDailyPlanning      = listexport.PresetDailyPlanning
	PresetAttendanceSnapshot = listexport.PresetAttendanceSnapshot
	PresetPickupList         = listexport.PresetPickupList
	PresetBlankChecklist     = listexport.PresetBlankChecklist
	PresetBirthdayList       = listexport.PresetBirthdayList
	PresetHealthList         = listexport.PresetHealthList
)

// Columns of the child lists.
const (
	ColumnName              = listexport.ColumnName
	ColumnSchoolClass       = listexport.ColumnSchoolClass
	ColumnGroup             = listexport.ColumnGroup
	ColumnEnrollmentSummary = listexport.ColumnEnrollmentSummary
	ColumnCareDays          = listexport.ColumnCareDays
	ColumnWeeklyMonday      = listexport.ColumnWeeklyMonday
	ColumnWeeklyTuesday     = listexport.ColumnWeeklyTuesday
	ColumnWeeklyWednesday   = listexport.ColumnWeeklyWednesday
	ColumnWeeklyThursday    = listexport.ColumnWeeklyThursday
	ColumnWeeklyFriday      = listexport.ColumnWeeklyFriday
	ColumnPlannedArrival    = listexport.ColumnPlannedArrival
	ColumnPlannedPickup     = listexport.ColumnPlannedPickup
	ColumnDailyStatus       = listexport.ColumnDailyStatus
	ColumnDeparture         = listexport.ColumnDeparture
	ColumnDailyNotes        = listexport.ColumnDailyNotes
	ColumnCurrentLocation   = listexport.ColumnCurrentLocation
	ColumnHealthInfo        = listexport.ColumnHealthInfo
	ColumnBirthday          = listexport.ColumnBirthday
	ColumnAge               = listexport.ColumnAge
)

// ResolveColumns resolves a caller-chosen column list against the catalog,
// falling back to the preset's defaults. The health note is never in the
// catalog; only the Gesundheitsliste prints it.
func ResolveColumns(ids []ColumnID, preset Preset) []Column {
	return listexport.ResolveColumns(ids, preset)
}

// SanitizeUserText removes the renderer's reserved styling markers from
// user-supplied text.
func SanitizeUserText(text string) string {
	return listexport.SanitizeUserText(text)
}

// ClassGroupTitle labels a per-class section of a grouped class list.
func ClassGroupTitle(schoolClass string) string {
	return listexport.ClassGroupTitle(schoolClass)
}
