// Package planexport renders the two weekly plans as printable documents
// (issue #2079): the Dienstplan (staff view — who works when, where, on
// what) and the Betreuungsplan (child view — which block runs when, with
// whom).
//
// It is a projection, not a second source of truth: the Dienstplan export
// reads the very same StaffScheduleOverview the screen renders from, so a
// printout cannot disagree with the plan it was printed from. Nothing here
// writes.
//
// A week is a matrix — one label column plus Monday to Friday — which is a
// plain listexport.Document with five day columns. Multi-week ranges emit
// one Row.GroupTitle per calendar week, and the renderers already start a
// new page per group, so every week lands on its own sheet.
package planexport

import (
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/services/listexport"
)

// Template is the row axis of the printed matrix.
type Template string

const (
	// TemplateByPerson is the Dienstplan as the screen shows it: one row per
	// staff member.
	TemplateByPerson Template = "persons"
	// TemplateByArea is the Dienstplan as the schools keep it in Excel: one
	// row per Einsatzbereich (Schichtart or Angebot), names in the cells.
	TemplateByArea Template = "areas"
	// TemplateByOffering is the Betreuungsplan: one row per Betreuungsblock.
	TemplateByOffering Template = "offerings"
)

// Variant decides how much of the plan's internal state is printed.
type Variant string

const (
	// VariantNotice is the sheet that goes on the wall: cancellations and
	// substitutes are named, but never a reason. "krank" next to a name is a
	// health datum and has no place in a corridor.
	VariantNotice Variant = "aushang"
	// VariantInternal adds the reasons, the absence markers, the staffing
	// shortfalls, and the uncovered intervals — for the office, not the wall.
	VariantInternal Variant = "intern"
)

// maxExportWeeks caps a range export. Eight weeks stays well inside the
// 62-day ceiling the underlying shift range queries enforce, and a longer
// wall plan is not a thing anyone prints.
const maxExportWeeks = 8

var (
	// ErrInvalidParams marks a request the caller got wrong (unknown
	// template, inverted range, oversized window). Handlers map it to 400.
	ErrInvalidParams = errors.New("invalid plan export parameters")
	// ErrRangeTooLarge is a specific ErrInvalidParams so the handler can
	// phrase the limit for the user.
	ErrRangeTooLarge = fmt.Errorf("%w: range exceeds %d weeks", ErrInvalidParams, maxExportWeeks)
)

// Params is one export request. From/To are any two calendar days; the
// service widens them to whole Monday–Friday weeks, because a wall plan is
// printed by the week and a Wednesday-to-Wednesday sheet helps nobody.
type Params struct {
	From     timezone.Date
	To       timezone.Date
	Template Template
	Variant  Variant
	Format   listexport.Format
}

// TemplatesForDienstplan and TemplatesForBetreuungsplan are the templates
// each plan accepts — the single place the pairing is decided, shared by
// validation and by the handlers' error messages.
var (
	TemplatesForDienstplan     = []Template{TemplateByPerson, TemplateByArea}
	TemplatesForBetreuungsplan = []Template{TemplateByOffering}
)

func (v Variant) valid() bool {
	return v == VariantNotice || v == VariantInternal
}

// internal reports whether the internal state (reasons, absences, gaps)
// belongs in the output.
func (v Variant) internal() bool { return v == VariantInternal }

func formatValid(f listexport.Format) bool {
	// DOCX is deliberately not offered: a matrix in Word is the weakest of
	// the three outputs and nobody asked for it (#2079).
	return f == listexport.FormatPDF || f == listexport.FormatXLSX
}

// validate checks the request against the templates the plan allows.
func (p Params) validate(allowed []Template) error {
	if p.From.IsZero() || p.To.IsZero() {
		return fmt.Errorf("%w: from and to are required", ErrInvalidParams)
	}
	if p.To.Before(p.From) {
		return fmt.Errorf("%w: to must not be before from", ErrInvalidParams)
	}
	if !p.Variant.valid() {
		return fmt.Errorf("%w: unknown variant %q", ErrInvalidParams, p.Variant)
	}
	if !formatValid(p.Format) {
		return fmt.Errorf("%w: unsupported format %q", ErrInvalidParams, p.Format)
	}
	for _, template := range allowed {
		if p.Template == template {
			return nil
		}
	}
	return fmt.Errorf("%w: unknown template %q", ErrInvalidParams, p.Template)
}

// ParseParams turns the raw request strings into validated Params. It lives
// here rather than in either handler so the two routes cannot drift apart on
// what "aushang" or an empty format means.
//
// An empty format defaults to PDF: the printed sheet is the point of the
// feature, and every caller that omits the field wants paper.
func ParseParams(from, to, template, variant, format string) (Params, error) {
	fromDate, err := timezone.ParseDate(from)
	if err != nil {
		return Params{}, fmt.Errorf("%w: from must be YYYY-MM-DD", ErrInvalidParams)
	}
	toDate, err := timezone.ParseDate(to)
	if err != nil {
		return Params{}, fmt.Errorf("%w: to must be YYYY-MM-DD", ErrInvalidParams)
	}
	if format == "" {
		format = string(listexport.FormatPDF)
	}
	if variant == "" {
		variant = string(VariantNotice)
	}
	return Params{
		From:     fromDate,
		To:       toDate,
		Template: Template(template),
		Variant:  Variant(variant),
		Format:   listexport.Format(format),
	}, nil
}
