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

// Renderer renders a list document as a PDF, DOCX, XLSX or CSV file. Rendering neither stores the file nor
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

// Letters are a single document in the layout of its sender rather than
// in the list design: letterhead, address field, body and footer (#4669).
type (
	// Letter is one letter.
	Letter = records.Letter
	// LetterFact is a label with its value, beside the address field or in
	// a table of the body.
	LetterFact = records.LetterFact
	// LetterBlock is a part of the body.
	LetterBlock = records.LetterBlock
)

// LetterRenderer renders a letter as a PDF. Like Renderer it neither stores
// the file nor decides who may read it.
type LetterRenderer interface {
	RenderLetter(letter Letter, filenameBase string) (File, error)
}

var _ LetterRenderer = (*renderer.RendererService)(nil)

// NewLetterRenderer returns the Document Rendering letter renderer.
func NewLetterRenderer() LetterRenderer {
	return renderer.NewService()
}

const (
	FormatPDF  = records.FormatPDF
	FormatDOCX = records.FormatDOCX
	FormatXLSX = records.FormatXLSX
	FormatCSV  = records.FormatCSV
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
	ColumnChecklist         = records.ColumnChecklist
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

// The shared class-list table (#4749): both class lists, the
// Kinderlisten-Vorlage and the one of an enrollment phase, render their
// children through it.
type (
	// ClassRosterChild is one child of a class list.
	ClassRosterChild = records.ClassRosterChild
	// ClassRosterDay is one weekday of a child on a class list.
	ClassRosterDay = records.ClassRosterDay
	// ClassSectionRow is one table row with the class it belongs to.
	ClassSectionRow = records.ClassSectionRow
)

// ClassRosterNoCareLabel marks a class-list-only entry.
const ClassRosterNoCareLabel = records.ClassRosterNoCareLabel

// ClassRosterWeekdayColumns are the weekday columns of a class list, Monday
// first.
var ClassRosterWeekdayColumns = records.ClassRosterWeekdayColumns

// ClassRosterColumnIDs are the columns both class lists print, in order.
func ClassRosterColumnIDs() []ColumnID {
	return records.ClassRosterColumnIDs()
}

// ClassRosterRow renders one child into a row of the shared table.
func ClassRosterRow(child ClassRosterChild) Row {
	return records.ClassRosterRow(child)
}

// ClassRosterRows renders class-sorted children into the table rows.
func ClassRosterRows(children []ClassRosterChild, grouped bool) []Row {
	return records.ClassRosterRows(children, grouped)
}

// ClassSectionRows puts a heading row before each class when grouped.
func ClassSectionRows(rows []ClassSectionRow, grouped bool) []Row {
	return records.ClassSectionRows(rows, grouped)
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
