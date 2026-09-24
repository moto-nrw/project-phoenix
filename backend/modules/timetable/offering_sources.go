package timetable

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// OfferingSourceSupport is what the Regeltermin editor learns from Enrollment
// about templates that source their roster from care offerings (#2137,
// #3140): the offerings a template may source from, the deduplicated
// children of a selection, why a sourced occurrence is still empty, and
// whether later approvals reach the roster without staff action. The
// composition root binds it over the enrollment decision service.
type OfferingSourceSupport interface {
	// ListOfferingSourceOptions lists the tenant's offerings, restricted to
	// those fitting the calendar period when one is given. An invalid
	// selection carries ErrOfferingSourceInvalid.
	ListOfferingSourceOptions(ctx context.Context, calendarPeriodID *int64) ([]OfferingSourceOption, error)
	// CombinedOfferingSourceCounts validates a selection like a save would
	// and counts its distinct approved children. An invalid selection carries
	// ErrOfferingSourceInvalid.
	CombinedOfferingSourceCounts(ctx context.Context, offeringIDs []int64, calendarPeriodID *int64) (OfferingSourceCounts, error)
	// EmptyRosterExplainer reads the offerings of one calendar period once
	// and explains, for any selection and day, why an occurrence sourced from
	// them has no children.
	EmptyRosterExplainer(ctx context.Context, calendarPeriodID *int64) (EmptyOfferingRosterExplainer, error)
	// TemplateRosterMaintenance derives the roster indicator of each template
	// with a constant number of reads, keyed by template id.
	TemplateRosterMaintenance(ctx context.Context, templates []TemplateRosterMaintenanceQuery) (map[int64]TemplateRosterMaintenance, error)
}

// OfferingSourceOption is one selectable Betreuungsangebot of the editor with
// the approved children per Jahrgang and the templates already sourcing it.
type OfferingSourceOption struct {
	ID        int64
	Name      string
	PhaseID   int64
	PhaseName string
	// PhaseServiceStart is the phase's first service day; zero when unset.
	PhaseServiceStart calendar.Date
	TotalCount        int
	// GradeCounts maps Jahrgang → approved children; key 0 collects children
	// without a derivable grade.
	GradeCounts            map[int]int
	SourcedTemplates       []OfferingSourcedTemplate
	LegacyLinkedTemplateID *int64
}

// OfferingSourcedTemplate is a template already sourcing an offering.
type OfferingSourcedTemplate struct {
	ID            int64
	Name          string
	GradeLevels   []int
	SchoolClasses []string
}

// OfferingSourceCounts are the distinct approved children of a selection of
// offerings, each counted once.
type OfferingSourceCounts struct {
	TotalCount int
	// GradeCounts maps Jahrgang → distinct approved children; key 0 collects
	// children without a derivable grade.
	GradeCounts map[int]int
	Students    []OfferingSourceStudent
}

// OfferingSourceStudent is one child of a selection with the school class the
// Jahrgang and Klassen filters match on.
type OfferingSourceStudent struct {
	StudentID   int64
	SchoolClass string
}

// The kinds of EmptyOfferingRoster.
const (
	EmptyOfferingRosterBeforeServiceStart = "before_offering_start"
	EmptyOfferingRosterSourceEmpty        = "offering_source_empty"
)

// EmptyOfferingRoster explains why an occurrence sourced from care offerings
// has no children yet.
type EmptyOfferingRoster struct {
	Kind             string
	PhaseName        string
	ServiceStartDate calendar.Date
}

// EmptyOfferingRosterExplainer returns the explanation for an empty
// occurrence on date sourced from the selected offerings, or nil when the
// selection is empty.
type EmptyOfferingRosterExplainer func(selectedOfferingIDs []int64, date calendar.Date) *EmptyOfferingRoster

// TemplateRosterMaintenanceQuery carries one template's source rule and
// targets for the roster indicator.
type TemplateRosterMaintenanceQuery struct {
	TemplateID            int64
	CalendarPeriodID      *int64
	SourceCareOfferingIDs []int64
	SourceGradeLevels     []int
	SourceSchoolClasses   []string
	// HasDynamicTargets is true when the template targets a Klasse, a
	// Jahrgang or a Gruppe; those resolve once per created occurrence.
	HasDynamicTargets bool
}

// TemplateRosterMaintenance is the Regeltermin indicator (#3140): whether
// children approved later reach the roster without staff action.
type TemplateRosterMaintenance struct {
	// Mode is "automatic", "partial" or "manual".
	Mode              string
	Offerings         []OfferingRef
	GradeLevels       []int
	SchoolClasses     []string
	InactiveOfferings []OfferingRef
	InvalidOfferings  []OfferingRef
	// DynamicTargetsManual is true when children joining a Klasse, Jahrgang
	// or Gruppe target later are not added to existing occurrences.
	DynamicTargetsManual bool
	// CareOfferingsDisabled is true when offerings are configured but the
	// school switched Betreuungsangebote off.
	CareOfferingsDisabled bool
}

// OfferingRef names one care offering.
type OfferingRef struct {
	ID   int64
	Name string
}
