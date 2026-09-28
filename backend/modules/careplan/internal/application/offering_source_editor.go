package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// CombinedOfferingSourceCounts validates the selection exactly like a save
// would (existence, active, one shared phase, phase-within-period) and then
// counts distinct approved children across the offerings. Dedup key is the
// request child — within one phase a child holds one request-child identity,
// which is also the provenance tag the union resync seeds rows under.
func (m *BookingMaterialization) CombinedOfferingSourceCounts(ctx context.Context, offeringIDs []int64, calendarPeriodID *int64) (*careplan.OfferingSourceCombinedCounts, error) {
	sources, err := m.catalog.LoadOfferingSources(ctx, offeringIDs, calendarPeriodID, false)
	if err != nil {
		return nil, err
	}
	// Vanished ids were dropped by the loader; count only the survivors, the
	// same set a save's resync would union. None surviving means zero counts.
	countedIDs := make([]int64, 0, len(sources.Offerings))
	for _, offering := range sources.Offerings {
		countedIDs = append(countedIDs, offering.ID)
	}
	if len(countedIDs) == 0 {
		return &careplan.OfferingSourceCombinedCounts{GradeCounts: map[int]int{}, Students: []careplan.OfferingSourceStudent{}}, nil
	}
	period, err := m.offeringSourcePeriod(ctx, calendarPeriodID, "offering source counts")
	if err != nil {
		return nil, err
	}
	children, err := m.deps.Enrollment.ApprovedChildren(ctx, countedIDs, m.offeringSourceCountedFrom(period))
	if err != nil {
		return nil, fmt.Errorf("offering source counts: list approved children: %w", err)
	}
	counts := &careplan.OfferingSourceCombinedCounts{
		GradeCounts: map[int]int{},
		Students:    make([]careplan.OfferingSourceStudent, 0, len(children)),
	}
	seen := make(map[int64]bool, len(children))
	for _, child := range children {
		if seen[child.Link.RequestChildID] {
			continue
		}
		seen[child.Link.RequestChildID] = true
		counts.TotalCount++
		counts.GradeCounts[gradeBucket(child.GradeLevel)]++
		counts.Students = append(counts.Students, careplan.OfferingSourceStudent{
			StudentID:   child.StudentID,
			SchoolClass: child.SchoolClass,
		})
	}
	return counts, nil
}

// offeringSourceCountedFrom mirrors what a resync for the selected period
// would seed: its effective date never lies before the period starts, so for
// a future period a link that ends before the period begins contributes
// nothing. For a running (or past) period today stays the boundary — links
// that already ended are no longer plannable either way.
func (m *BookingMaterialization) offeringSourceCountedFrom(period *careplan.LinkedPeriod) calendar.Date {
	countedFrom := m.todayDate()
	if period != nil && period.StartDate.After(countedFrom) {
		countedFrom = period.StartDate
	}
	return countedFrom
}

// offeringSourcePeriod loads the calendar period a selection is checked
// against; nil without a period. A missing period is the Timetable's
// offering-source refusal.
func (m *BookingMaterialization) offeringSourcePeriod(ctx context.Context, calendarPeriodID *int64, operation string) (*careplan.LinkedPeriod, error) {
	if calendarPeriodID == nil {
		return nil, nil
	}
	period, err := m.catalog.deps.Calendar.FindPeriod(ctx, *calendarPeriodID)
	if err != nil {
		if errors.Is(err, ports.ErrCatalogRowNotFound) {
			return nil, m.catalog.deps.SourceRules.Reject(fmt.Sprintf("calendar period %d not found", *calendarPeriodID))
		}
		return nil, fmt.Errorf("%s: load calendar period: %w", operation, err)
	}
	return &period, nil
}

// ListOfferingSourceOptions returns the offerings an admin may pick as a
// Regeltermin source. With a calendar period given, only offerings whose
// phase's service window lies within that period qualify — a source outside
// the Planungszeitraum could never materialize a single occurrence.
func (m *BookingMaterialization) ListOfferingSourceOptions(ctx context.Context, calendarPeriodID *int64) ([]careplan.OfferingSourceOption, error) {
	offerings, err := m.catalog.listTenantRecords(ctx)
	if err != nil {
		return nil, fmt.Errorf("offering source options: list offerings: %w", err)
	}
	phases, period, err := m.offeringSourcePhases(ctx, calendarPeriodID)
	if err != nil {
		return nil, err
	}
	selected := make([]careplan.CareOffering, 0, len(offerings))
	offeringIDs := make([]int64, 0, len(offerings))
	for _, offering := range offerings {
		if _, ok := phases[offering.PhaseID]; !offering.IsActive || !ok {
			continue
		}
		selected = append(selected, offering)
		offeringIDs = append(offeringIDs, offering.ID)
	}
	children, err := m.deps.Enrollment.ApprovedChildren(ctx, offeringIDs, m.offeringSourceCountedFrom(period))
	if err != nil {
		return nil, fmt.Errorf("offering source options: list approved children: %w", err)
	}
	counts := groupOfferingGradeCounts(children)
	templates, err := m.deps.Templates.TemplatesSourcedFrom(ctx, offeringIDs)
	if err != nil {
		return nil, fmt.Errorf("offering source options: list sourced templates: %w", err)
	}
	templatesByOffering := sourcedTemplatesByOffering(templates)
	options := make([]careplan.OfferingSourceOption, 0, len(selected))
	for _, offering := range selected {
		options = append(options, offeringSourceOption(offering, phases[offering.PhaseID], counts[offering.ID], templatesByOffering[offering.ID]))
	}
	return options, nil
}

func offeringSourceOption(
	offering careplan.CareOffering,
	phase careplan.OfferingPhase,
	count *offeringGradeCount,
	templates []ports.SourcedTemplate,
) careplan.OfferingSourceOption {
	option := careplan.OfferingSourceOption{
		ID:                     offering.ID,
		Name:                   offering.Name,
		PhaseID:                offering.PhaseID,
		PhaseName:              phase.Name,
		PhaseServiceStart:      phase.ServiceStart,
		GradeCounts:            map[int]int{},
		SourcedTemplates:       []careplan.OfferingSourcedTemplate{},
		LegacyLinkedTemplateID: offering.ActivityGroupID,
	}
	if count != nil {
		option.TotalCount = count.total
		option.GradeCounts = count.byGrade
	}
	for _, tmpl := range templates {
		option.SourcedTemplates = append(option.SourcedTemplates, careplan.OfferingSourcedTemplate{
			ID:            tmpl.ID,
			Name:          tmpl.Name,
			GradeLevels:   tmpl.SourceGradeLevels,
			SchoolClasses: tmpl.SourceSchoolClasses,
		})
	}
	return option
}

// offeringSourcePhases returns the tenant's phases keyed by id, restricted to
// those fitting the calendar period when one is given. The loaded period is
// returned alongside so the caller can scope its child counts to it.
func (m *BookingMaterialization) offeringSourcePhases(ctx context.Context, calendarPeriodID *int64) (map[int64]careplan.OfferingPhase, *careplan.LinkedPeriod, error) {
	phases, err := m.deps.Enrollment.Phases(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("offering source options: list phases: %w", err)
	}
	period, err := m.offeringSourcePeriod(ctx, calendarPeriodID, "offering source options")
	if err != nil {
		return nil, nil, err
	}
	byID := make(map[int64]careplan.OfferingPhase, len(phases))
	for _, phase := range phases {
		if careplan.ValidatePhaseWithinPeriod(phase, period) != nil {
			continue
		}
		byID[phase.ID] = phase
	}
	return byID, period, nil
}

type offeringGradeCount struct {
	total   int
	byGrade map[int]int
}

// groupOfferingGradeCounts aggregates approved children per offering into
// distinct-child totals and per-grade buckets (0 = no derivable grade).
func groupOfferingGradeCounts(children []ports.ApprovedOfferingChild) map[int64]*offeringGradeCount {
	counts := make(map[int64]*offeringGradeCount)
	seen := make(map[int64]map[int64]bool)
	for _, child := range children {
		offeringID := child.Link.CareOfferingID
		if seen[offeringID] == nil {
			seen[offeringID] = make(map[int64]bool)
		}
		if seen[offeringID][child.Link.RequestChildID] {
			continue
		}
		seen[offeringID][child.Link.RequestChildID] = true
		count := counts[offeringID]
		if count == nil {
			count = &offeringGradeCount{byGrade: map[int]int{}}
			counts[offeringID] = count
		}
		count.total++
		count.byGrade[gradeBucket(child.GradeLevel)]++
	}
	return counts
}

// gradeBucket is the Jahrgang a child counts under; 0 collects children whose
// school class carries no grade number.
func gradeBucket(grade *int16) int {
	if grade == nil {
		return 0
	}
	return int(*grade)
}
