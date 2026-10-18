// Package lists is the Document Rendering contract for list exports (#3356):
// the document a consumer fills (title, filter labels, columns, rows), the
// vocabulary of the child-list presets and columns, and the Renderer that
// turns it into a PDF, DOCX or XLSX file. The renderer itself stays in
// the owner’s internal application; native record types supply the contract, so
// a consumer never reaches the renderer package and a rendered file is
// byte-for-byte what the renderer produced before.
package lists

import (
	renderer "github.com/moto-nrw/project-phoenix/modules/documentrendering/internal/application/listrenderer"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/records"
)

type (
	// Document is one list: header lines, columns and rows.
	Document = records.Document
	// Column is one printed column: its id and header label.
	Column = records.Column
	// ColumnID names a column and keys a row's values.
	ColumnID = records.ColumnID
	// Row is one printed row; GroupTitle opens a new section.
	Row = records.Row
	// Preset names a child list's default column set.
	Preset = records.Preset
	// Format is the output format.
	Format = records.Format
	// File is a rendered document.
	File = records.File
)

// Renderer renders a list document. Rendering neither stores the file nor
// decides who may read it; the caller owns both.
type Renderer interface {
	Render(doc Document, format Format, filenameBase string) (File, error)
}

// The renderer the contract names satisfies it.
var _ Renderer = (*renderer.RendererService)(nil)

// NewRenderer returns the Document Rendering list renderer.
func NewRenderer() Renderer {
	return NewRendererService()
}

// Record documents are the block layout of the same design: groups of cards,
// each with label/value fields and indented sub-blocks. Proofs and reports
// that are not a table use them (#3430).
type (
	// RecordDocument is one block-layout document.
	RecordDocument = records.RecordDocument
	// RecordGroup is a titled section of records.
	RecordGroup = records.RecordGroup
	// Record is one card.
	Record = records.Record
	// SubRecord is an indented block inside a card.
	SubRecord = records.SubRecord
	// Field is one label/value line.
	Field = records.Field
)

// RecordRenderer renders a record document as a PDF. Like Renderer it
// neither stores the file nor decides who may read it.
type RecordRenderer interface {
	RenderRecords(doc RecordDocument, filenameBase string) (File, error)
}

var _ RecordRenderer = (*renderer.RendererService)(nil)

// NewRecordRenderer returns the Document Rendering record renderer.
func NewRecordRenderer() RecordRenderer {
	return NewRendererService()
}

// RecordDOCXRenderer renders a record document as an editable DOCX, in the
// same block layout as the PDF.
type RecordDOCXRenderer interface {
	RenderRecordsDOCX(doc RecordDocument, filenameBase string) (File, error)
}

var _ RecordDOCXRenderer = (*renderer.RendererService)(nil)

// DocumentRenderer renders a consumer's exports in every layout: list
// documents in any format and record documents as PDF or DOCX (#2734).
type DocumentRenderer interface {
	Renderer
	RecordRenderer
	RecordDOCXRenderer
}

var _ DocumentRenderer = (*renderer.RendererService)(nil)

const (
	FormatPDF  = records.FormatPDF
	FormatDOCX = records.FormatDOCX
	FormatXLSX = records.FormatXLSX
)

// Presets of the child lists.
const (
	PresetOGSWeekly          = records.PresetOGSWeekly
	PresetOGSCompact         = records.PresetOGSCompact
	PresetClassRoster        = records.PresetClassRoster
	PresetDailyPlanning      = records.PresetDailyPlanning
	PresetAttendanceSnapshot = records.PresetAttendanceSnapshot
	PresetPickupList         = records.PresetPickupList
	PresetBlankChecklist     = records.PresetBlankChecklist
	PresetBirthdayList       = records.PresetBirthdayList
	PresetStaffBirthdayList  = records.PresetStaffBirthdayList
	PresetHealthList         = records.PresetHealthList
)

// Columns of the child lists.
const (
	ColumnName              = records.ColumnName
	ColumnSchoolClass       = records.ColumnSchoolClass
	ColumnGroup             = records.ColumnGroup
	ColumnEnrollmentSummary = records.ColumnEnrollmentSummary
	ColumnCareDays          = records.ColumnCareDays
	ColumnWeeklyMonday      = records.ColumnWeeklyMonday
	ColumnWeeklyTuesday     = records.ColumnWeeklyTuesday
	ColumnWeeklyWednesday   = records.ColumnWeeklyWednesday
	ColumnWeeklyThursday    = records.ColumnWeeklyThursday
	ColumnWeeklyFriday      = records.ColumnWeeklyFriday
	ColumnPlannedArrival    = records.ColumnPlannedArrival
	ColumnPlannedPickup     = records.ColumnPlannedPickup
	ColumnDailyStatus       = records.ColumnDailyStatus
	ColumnDeparture         = records.ColumnDeparture
	ColumnDailyNotes        = records.ColumnDailyNotes
	ColumnCurrentLocation   = records.ColumnCurrentLocation
	ColumnHealthInfo        = records.ColumnHealthInfo
	ColumnBirthday          = records.ColumnBirthday
	ColumnAge               = records.ColumnAge
	ColumnGuardianContacts  = records.ColumnGuardianContacts
	ColumnSlot              = records.ColumnSlot
	ColumnPresenceStatus    = records.ColumnPresenceStatus
)

// Columns of the weekly plans (Dienstplan, Betreuungsplan): the row label and
// one column per weekday.
const (
	ColumnPlanRowLabel  = records.ColumnPlanRowLabel
	ColumnPlanMonday    = records.ColumnPlanMonday
	ColumnPlanTuesday   = records.ColumnPlanTuesday
	ColumnPlanWednesday = records.ColumnPlanWednesday
	ColumnPlanThursday  = records.ColumnPlanThursday
	ColumnPlanFriday    = records.ColumnPlanFriday
	ColumnPlanSaturday  = records.ColumnPlanSaturday
	ColumnPlanSunday    = records.ColumnPlanSunday
)

// ResolveColumns resolves a caller-chosen column list against the catalog,
// falling back to the preset's defaults. The health note is never in the
// catalog; only the Gesundheitsliste prints it.
func ResolveColumns(ids []ColumnID, preset Preset) []Column {
	return records.ResolveColumns(ids, preset)
}

// SanitizeUserText removes the renderer's reserved styling markers from
// user-supplied text.
func SanitizeUserText(text string) string {
	return records.SanitizeUserText(text)
}

// ClassGroupTitle labels a per-class section of a grouped class list.
func ClassGroupTitle(schoolClass string) string {
	return records.ClassGroupTitle(schoolClass)
}

// Columns of the guardian finance export.
const (
	ColumnStudentName  = records.ColumnStudentName
	ColumnStudentClass = records.ColumnStudentClass
	ColumnContactName  = records.ColumnContactName
	ColumnIBAN         = records.ColumnIBAN
)

// RendererService is the concrete renderer behind every renderer interface
// above; compositions that hand one instance to several consumers hold it
// under this name (#2747).
type RendererService struct{ DocumentRenderer }

// NewRendererService returns the Document Rendering renderer.
func NewRendererService() *RendererService {
	return &RendererService{DocumentRenderer: renderer.NewService()}
}

// RenderWorkbook renders a plain XLSX sheet: one header row and the given
// rows, with the listed columns formatted as decimals.
func RenderWorkbook(sheet string, headers []string, rows [][]any, decimalColumns []int) ([]byte, error) {
	return renderer.RenderWorkbook(sheet, headers, rows, decimalColumns)
}
