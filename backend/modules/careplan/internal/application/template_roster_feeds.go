package application

import (
	"context"
	"fmt"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// TemplateRosterMaintenanceFeeds resolves, for every requested template, the
// source offerings, the offerings linked to any segment of its split series,
// and the care-offerings setting (#3140). It costs a constant number of reads
// regardless of how many templates are asked for.
func (m *BookingMaterialization) TemplateRosterMaintenanceFeeds(ctx context.Context, templates []careplan.TemplateRosterFeedQuery) (map[int64]careplan.TemplateRosterFeeds, error) {
	result := make(map[int64]careplan.TemplateRosterFeeds, len(templates))
	if len(templates) == 0 {
		return result, nil
	}
	enabled, err := m.careOfferingsEnabled(ctx)
	if err != nil {
		return nil, fmt.Errorf("template roster maintenance: resolve care offerings setting: %w", err)
	}
	offerings, err := m.catalog.listTenantRecords(ctx)
	if err != nil {
		return nil, fmt.Errorf("template roster maintenance: list offerings: %w", err)
	}
	index := newRosterFeedIndex(offerings, templates)
	validation, err := m.loadRosterFeedValidation(ctx, index)
	if err != nil {
		return nil, err
	}
	invalid, err := validation.invalidSources(templates, index.offerings)
	if err != nil {
		return nil, err
	}
	rootByGroup, err := m.rosterFeedSeriesRoots(ctx, index.groupIDs)
	if err != nil {
		return nil, err
	}
	linkedByRoot := linkedOfferingsByRoot(offerings, rootByGroup)
	for _, query := range templates {
		root, ok := rootByGroup[query.TemplateID]
		if !ok {
			root = query.TemplateID
		}
		feeds, err := validation.templateFeeds(query, linkedByRoot[root], index.offerings, invalid.forTemplate(query.TemplateID))
		if err != nil {
			return nil, err
		}
		feeds.CareOfferingsEnabled = enabled
		result[query.TemplateID] = feeds
	}
	return result, nil
}

// rosterFeedIndex is the offering side every template's feeds are read from:
// the tenant's offerings by id, the offerings whose source or link needs the
// period and phase check, and the groups whose series roots are read.
type rosterFeedIndex struct {
	offerings             map[int64]*careplan.CareOffering
	validationOfferingIDs map[int64]bool
	groupIDs              []int64
}

func newRosterFeedIndex(offerings []careplan.CareOffering, templates []careplan.TemplateRosterFeedQuery) rosterFeedIndex {
	index := rosterFeedIndex{
		offerings:             offeringsByID(offerings),
		validationOfferingIDs: make(map[int64]bool),
		groupIDs:              make([]int64, 0, len(templates)+len(offerings)),
	}
	for _, template := range templates {
		for _, id := range template.SourceCareOfferingIDs {
			index.validationOfferingIDs[id] = true
		}
		index.groupIDs = append(index.groupIDs, template.TemplateID)
	}
	for _, offering := range offerings {
		if offering.ActivityGroupID != nil {
			index.groupIDs = append(index.groupIDs, *offering.ActivityGroupID)
			index.validationOfferingIDs[offering.ID] = true
		}
	}
	slices.Sort(index.groupIDs)
	index.groupIDs = slices.Compact(index.groupIDs)
	return index
}

// rosterFeedValidation holds the calendar periods and the phases of the
// offerings the feeds validate; both stay empty when nothing needs the check.
type rosterFeedValidation struct {
	periods map[int64]careplan.LinkedPeriod
	phases  map[int64]careplan.OfferingPhase
}

func (m *BookingMaterialization) loadRosterFeedValidation(ctx context.Context, index rosterFeedIndex) (rosterFeedValidation, error) {
	validation := rosterFeedValidation{periods: map[int64]careplan.LinkedPeriod{}, phases: map[int64]careplan.OfferingPhase{}}
	if len(index.validationOfferingIDs) == 0 {
		return validation, nil
	}
	periods, err := m.deps.Periods.Periods(ctx)
	if err != nil {
		return rosterFeedValidation{}, fmt.Errorf("template roster maintenance: list calendar periods: %w", err)
	}
	for _, period := range periods {
		validation.periods[period.ID] = period
	}
	phaseIDs := make([]int64, 0, len(index.validationOfferingIDs))
	for id := range index.validationOfferingIDs {
		if offering := index.offerings[id]; offering != nil {
			phaseIDs = append(phaseIDs, offering.PhaseID)
		}
	}
	slices.Sort(phaseIDs)
	phaseIDs = slices.Compact(phaseIDs)
	phases, err := m.deps.Enrollment.PhasesByID(ctx, phaseIDs)
	if err != nil {
		return rosterFeedValidation{}, fmt.Errorf("template roster maintenance: load offering phases: %w", err)
	}
	for _, phase := range phases {
		validation.phases[phase.ID] = phase
	}
	return validation, nil
}

// period returns the template's period pin; nil without one. A pin that
// names no period of the tenant fails the read.
func (v rosterFeedValidation) period(calendarPeriodID *int64) (*careplan.LinkedPeriod, error) {
	if calendarPeriodID == nil {
		return nil, nil
	}
	period, ok := v.periods[*calendarPeriodID]
	if !ok {
		return nil, fmt.Errorf("template roster maintenance: calendar period %d not found", *calendarPeriodID)
	}
	return &period, nil
}

// offeringFits reports whether the offering's phase is known and lies within
// the period (nil period passes).
func (v rosterFeedValidation) offeringFits(offering *careplan.CareOffering, period *careplan.LinkedPeriod) bool {
	phase, ok := v.phases[offering.PhaseID]
	return ok && careplan.ValidatePhaseWithinPeriod(phase, period) == nil
}

// invalidOfferings marks, per template, the offerings resync cannot use.
type invalidOfferings map[int64]map[int64]bool

func (i invalidOfferings) mark(templateID, offeringID int64) {
	if i[templateID] == nil {
		i[templateID] = make(map[int64]bool)
	}
	i[templateID][offeringID] = true
}

func (i invalidOfferings) forTemplate(templateID int64) map[int64]bool {
	return i[templateID]
}

// invalidSources marks the source offerings of each template that resync
// rejects: an unknown phase, a phase outside the period pin, or a source rule
// mixing phases, which invalidates every source of the template.
func (v rosterFeedValidation) invalidSources(templates []careplan.TemplateRosterFeedQuery, offerings map[int64]*careplan.CareOffering) (invalidOfferings, error) {
	invalid := make(invalidOfferings)
	for _, query := range templates {
		if len(query.SourceCareOfferingIDs) == 0 {
			continue
		}
		period, err := v.period(query.CalendarPeriodID)
		if err != nil {
			return nil, err
		}
		if v.markInvalidSources(invalid, query, offerings, period) {
			for _, id := range query.SourceCareOfferingIDs {
				if offerings[id] != nil {
					invalid.mark(query.TemplateID, id)
				}
			}
		}
	}
	return invalid, nil
}

// markInvalidSources marks the invalid sources of one template and reports
// whether the source rule mixes phases.
func (v rosterFeedValidation) markInvalidSources(invalid invalidOfferings, query careplan.TemplateRosterFeedQuery, offerings map[int64]*careplan.CareOffering, period *careplan.LinkedPeriod) bool {
	var sourcePhaseID int64
	mixedPhases := false
	for _, id := range query.SourceCareOfferingIDs {
		offering := offerings[id]
		if offering == nil {
			continue
		}
		_, known := v.phases[offering.PhaseID]
		isInvalid := !known
		if sourcePhaseID != 0 && sourcePhaseID != offering.PhaseID {
			mixedPhases, isInvalid = true, true
		}
		if sourcePhaseID == 0 {
			sourcePhaseID = offering.PhaseID
		}
		if !isInvalid && !v.offeringFits(offering, period) {
			isInvalid = true
		}
		if isInvalid {
			invalid.mark(query.TemplateID, id)
		}
	}
	return mixedPhases
}

// rosterFeedSeriesRoots maps every group to the root of its split series.
// Split segments share one root: the original keeps a NULL root and every
// successor points at it. The legacy fan-out drafts on every segment, so a
// link on any segment feeds the whole series.
func (m *BookingMaterialization) rosterFeedSeriesRoots(ctx context.Context, groupIDs []int64) (map[int64]int64, error) {
	groups, err := m.deps.Templates.Groups(ctx, groupIDs)
	if err != nil {
		return nil, fmt.Errorf("template roster maintenance: load series roots: %w", err)
	}
	rootByGroup := make(map[int64]int64, len(groups))
	for _, group := range groups {
		root := group.ID
		if group.SeriesRootID != nil {
			root = *group.SeriesRootID
		}
		rootByGroup[group.ID] = root
	}
	return rootByGroup, nil
}

func linkedOfferingsByRoot(offerings []careplan.CareOffering, rootByGroup map[int64]int64) map[int64][]careplan.TemplateRosterFeedOffering {
	linkedByRoot := make(map[int64][]careplan.TemplateRosterFeedOffering)
	for _, offering := range offerings {
		if offering.ActivityGroupID == nil {
			continue
		}
		root, ok := rootByGroup[*offering.ActivityGroupID]
		if !ok {
			continue
		}
		linkedByRoot[root] = append(linkedByRoot[root], feedOffering(offering))
	}
	return linkedByRoot
}

// templateFeeds assembles one template's feeds: its surviving sources and the
// offerings linked to its series, each marked invalid where resync cannot use
// it.
func (v rosterFeedValidation) templateFeeds(
	query careplan.TemplateRosterFeedQuery,
	linked []careplan.TemplateRosterFeedOffering,
	offerings map[int64]*careplan.CareOffering,
	invalid map[int64]bool,
) (careplan.TemplateRosterFeeds, error) {
	var period *careplan.LinkedPeriod
	if len(linked) > 0 {
		var err error
		if period, err = v.period(query.CalendarPeriodID); err != nil {
			return careplan.TemplateRosterFeeds{}, err
		}
	}
	// A linked offering resync cannot use is marked before the sources are
	// read, so a source that is also linked carries the same verdict.
	for _, feed := range linked {
		if offering := offerings[feed.ID]; offering != nil && !v.offeringFits(offering, period) {
			invalid = markLinkedInvalid(invalid, feed.ID)
		}
	}
	feeds := careplan.TemplateRosterFeeds{}
	for _, id := range query.SourceCareOfferingIDs {
		if offering := offerings[id]; offering != nil {
			feed := feedOffering(*offering)
			feed.IsInvalid = invalid[id]
			feeds.Sources = append(feeds.Sources, feed)
		}
	}
	for _, feed := range linked {
		feed.IsInvalid = invalid[feed.ID]
		feeds.LinkedOfferings = append(feeds.LinkedOfferings, feed)
	}
	return feeds, nil
}

func markLinkedInvalid(invalid map[int64]bool, offeringID int64) map[int64]bool {
	if invalid == nil {
		invalid = make(map[int64]bool)
	}
	invalid[offeringID] = true
	return invalid
}

func feedOffering(offering careplan.CareOffering) careplan.TemplateRosterFeedOffering {
	return careplan.TemplateRosterFeedOffering{ID: offering.ID, Name: offering.Name, IsActive: offering.IsActive}
}
