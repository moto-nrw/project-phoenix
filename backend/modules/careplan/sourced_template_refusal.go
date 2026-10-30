package careplan

import "errors"

// Wire codes of a catalog edit that would leave a Regeltermin without a
// valid offering source (#5313). The package may not import api/common, so
// each registered code is declared once here.
const (
	CodeCareOfferingTemplateNoActiveSource = "enrollment.care_offering_template_no_active_source"
	CodeCareOfferingTemplateMixedPhases    = "enrollment.care_offering_template_mixed_phases"
	CodeCareOfferingTemplateOutsidePeriod  = "enrollment.care_offering_template_outside_period"
	CodeCareOfferingTemplateSourceInvalid  = "enrollment.care_offering_template_source_invalid"
)

// ErrCareOfferingSourceConflict classifies a catalog edit refused because a
// Regeltermin sourcing the offering would lose its valid source. Every
// CareOfferingSourceConflictError reports itself as this value.
var ErrCareOfferingSourceConflict = errors.New("care offering edit conflicts with a sourced template")

// SourcedTemplateProblem says why a Regeltermin refused its stored sources.
type SourcedTemplateProblem int

const (
	// SourcedTemplateSourceInvalid is any other refusal.
	SourcedTemplateSourceInvalid SourcedTemplateProblem = iota
	// SourcedTemplateNoActiveSource: every source is inactive.
	SourcedTemplateNoActiveSource
	// SourcedTemplateMixedPhases: the sources belong to different phases.
	SourcedTemplateMixedPhases
	// SourcedTemplateOutsidePeriod: the sources' phase does not fit the
	// template's calendar period.
	SourcedTemplateOutsidePeriod
)

// SourcedTemplateRefusal is the refusal of an offering-scoped resync: the
// Regeltermin it would strand and why. The embedded error is the refusal.
type SourcedTemplateRefusal struct {
	error
	Template string
	Problem  SourcedTemplateProblem
}

// NewSourcedTemplateRefusal names the Regeltermin and the problem of err.
func NewSourcedTemplateRefusal(template string, problem SourcedTemplateProblem, err error) SourcedTemplateRefusal {
	return SourcedTemplateRefusal{error: err, Template: template, Problem: problem}
}

func (e SourcedTemplateRefusal) Unwrap() error { return e.error }

// CareOfferingSourceConflictValues are the values the refusal names.
type CareOfferingSourceConflictValues struct {
	Offering string `json:"offering"`
	Template string `json:"template"`
}

// CareOfferingSourceConflictError refuses a catalog edit that would leave
// a Regeltermin without a valid source: its registered code and the edited
// offering and the Regeltermin travel to the client as code and details.
// The embedded error keeps the diagnostic text and the refusal chain.
type CareOfferingSourceConflictError struct {
	error
	Code   string
	Values CareOfferingSourceConflictValues
}

func (e *CareOfferingSourceConflictError) Unwrap() []error {
	return []error{e.error, ErrCareOfferingSourceConflict}
}

// ErrorCode is the registered wire code of the refusal.
func (e *CareOfferingSourceConflictError) ErrorCode() string { return e.Code }

// ErrorDetails are the values the refusal names.
func (e *CareOfferingSourceConflictError) ErrorDetails() any { return e.Values }

// sourcedTemplateProblemCodes maps each problem onto its wire code.
var sourcedTemplateProblemCodes = map[SourcedTemplateProblem]string{
	SourcedTemplateSourceInvalid:  CodeCareOfferingTemplateSourceInvalid,
	SourcedTemplateNoActiveSource: CodeCareOfferingTemplateNoActiveSource,
	SourcedTemplateMixedPhases:    CodeCareOfferingTemplateMixedPhases,
	SourcedTemplateOutsidePeriod:  CodeCareOfferingTemplateOutsidePeriod,
}

// CareOfferingSourceConflict refuses the edit of offering with the
// Regeltermin refusal in err's chain. Without one it returns nil.
func CareOfferingSourceConflict(offering string, err error) error {
	var refusal SourcedTemplateRefusal
	if !errors.As(err, &refusal) {
		return nil
	}
	code, ok := sourcedTemplateProblemCodes[refusal.Problem]
	if !ok {
		code = CodeCareOfferingTemplateSourceInvalid
	}
	return &CareOfferingSourceConflictError{
		error:  err,
		Code:   code,
		Values: CareOfferingSourceConflictValues{Offering: offering, Template: refusal.Template},
	}
}
