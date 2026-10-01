package careplan

import "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"

// OfferingSourcedTemplate is one template already sourcing an offering, so
// the editor can warn about overlapping Jahrgang subsets before the admin
// saves a second Regeltermin over the same children.
type OfferingSourcedTemplate struct {
	ID          int64
	Name        string
	GradeLevels []int
	// SchoolClasses is the template's Klassenfilter (#2482); mutually
	// exclusive with GradeLevels.
	SchoolClasses []string
}

// OfferingSourceOption is one selectable Betreuungsangebot in the Regeltermin
// editor (#2137), with per-grade counts of approved children (live filter
// preview) and the templates already sourcing it (overlap Hinweis).
type OfferingSourceOption struct {
	ID        int64
	Name      string
	PhaseID   int64
	PhaseName string
	// PhaseServiceStart is the phase's service_start_date. Sourced
	// enrollments never start earlier, so materialised occurrences before this
	// day stay empty of offering-fed children (OGS am Berg, 2026-08).
	PhaseServiceStart calendar.Date
	// TotalCount is the number of distinct approved children currently or
	// prospectively enrolled in the offering.
	TotalCount int
	// GradeCounts maps Jahrgang → approved children; key 0 collects children
	// whose school class carries no derivable grade number.
	GradeCounts            map[int]int
	SourcedTemplates       []OfferingSourcedTemplate
	LegacyLinkedTemplateID *int64
}

// OfferingSourceCombinedCounts is the deduplicated child count across a
// SELECTION of offerings (multi-source follow-up to #2137): a child enrolled
// in two of the selected offerings counts once, so the editor's Jahrgang
// preview shows the exact roster size the union resync would seed.
type OfferingSourceCombinedCounts struct {
	// TotalCount is the number of DISTINCT approved children across the
	// selected offerings.
	TotalCount int
	// GradeCounts maps Jahrgang → distinct approved children; key 0 collects
	// children whose school class carries no derivable grade number.
	GradeCounts map[int]int
	// Students is the deduplicated child list behind the counts (#2482), each
	// with the school class the filters match on. Names are not carried: the
	// editor resolves them from the student list it already holds.
	Students []OfferingSourceStudent
}

// OfferingSourceStudent is one deduplicated child of a source selection with
// the school class the Jahrgang/Klassen filters are evaluated against.
type OfferingSourceStudent struct {
	StudentID   int64
	SchoolClass string
}

// The kinds of EmptyOfferingRosterExplanation.
const (
	EmptyOfferingRosterBeforeServiceStart = "before_offering_start"
	EmptyOfferingRosterSourceEmpty        = "offering_source_empty"
)

// EmptyOfferingRosterExplanation describes why an occurrence backed by care
// offerings currently has no concrete children. After the service start the
// explanation deliberately stays neutral: an empty occurrence can result
// from weekday/filter rules, changed enrollments, or a stale materialization.
type EmptyOfferingRosterExplanation struct {
	Kind             string
	PhaseName        string
	ServiceStartDate calendar.Date
}

// ExplainEmptyOfferingRoster classifies an empty sourced occurrence from the
// authoritative offering-phase metadata.
func ExplainEmptyOfferingRoster(
	options []OfferingSourceOption,
	selectedOfferingIDs []int64,
	date calendar.Date,
) *EmptyOfferingRosterExplanation {
	if len(selectedOfferingIDs) == 0 {
		return nil
	}
	selected := make(map[int64]struct{}, len(selectedOfferingIDs))
	for _, offeringID := range selectedOfferingIDs {
		selected[offeringID] = struct{}{}
	}
	explanation := &EmptyOfferingRosterExplanation{Kind: EmptyOfferingRosterSourceEmpty}
	for _, option := range options {
		if _, ok := selected[option.ID]; !ok {
			continue
		}
		explanation.PhaseName = option.PhaseName
		explanation.ServiceStartDate = option.PhaseServiceStart
		if !option.PhaseServiceStart.IsZero() && date.Before(option.PhaseServiceStart) {
			explanation.Kind = EmptyOfferingRosterBeforeServiceStart
		}
		return explanation
	}
	return explanation
}
