package enrollment

import (
	"context"
	"fmt"
	"slices"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// RosterMaintenanceMode names how children reach a Regeltermin's roster after
// it exists (#3140). It describes the effective path, not whether every
// background run succeeded.
type RosterMaintenanceMode string

const (
	// RosterMaintenanceAutomatic: every later approval or correction of a
	// feeding Betreuungsangebot is written into the roster and into the
	// already materialized future occurrences.
	RosterMaintenanceAutomatic RosterMaintenanceMode = "automatic"
	// RosterMaintenancePartial: an offering feeds the roster, but part of the
	// configured audience does not follow (a class, grade or group target is
	// resolved only when an occurrence is created, or one of several source
	// offerings is inactive).
	RosterMaintenancePartial RosterMaintenanceMode = "partial"
	// RosterMaintenanceManual: nothing adds later children; staff do.
	RosterMaintenanceManual RosterMaintenanceMode = "manual"
)

// RosterMaintenanceOffering is one offering named in the explanation.
type RosterMaintenanceOffering struct {
	ID   int64
	Name string
}

// TemplateRosterMaintenanceInput carries the facts for one template. The
// template-side facts (source rule, dynamic targets) come from the caller's
// template read; the offering-side facts from TemplateRosterMaintenanceFeeds.
type TemplateRosterMaintenanceInput struct {
	SourceCareOfferingIDs []int64
	SourceGradeLevels     []int
	SourceSchoolClasses   []string
	// HasDynamicTargets is true when the template targets a Klasse, a
	// Jahrgang or a Gruppe. Those resolve once per created occurrence.
	HasDynamicTargets bool
	Feeds             TemplateRosterFeeds
}

// TemplateRosterFeeds is the offering-side state of one template.
type TemplateRosterFeeds struct {
	// CareOfferingsEnabled mirrors enrollment.care_offerings_enabled. While
	// it is off, no approval materializes offering rosters.
	CareOfferingsEnabled bool
	// Sources resolves SourceCareOfferingIDs that still exist. Vanished ids
	// are dropped by every resync, so they are absent here as well.
	Sources []TemplateRosterFeedOffering
	// LinkedOfferings are offerings whose own timetable link points at any
	// segment of the template's split series.
	LinkedOfferings []TemplateRosterFeedOffering
}

// TemplateRosterFeedOffering is an offering together with its active flag.
type TemplateRosterFeedOffering struct {
	ID       int64
	Name     string
	IsActive bool
	// IsInvalid is true for a source that resync rejects because its enrollment
	// phase does not fit the selected planning period or the source rule mixes
	// enrollment phases.
	IsInvalid bool
}

// TemplateRosterMaintenance is the derived indicator state.
type TemplateRosterMaintenance struct {
	Mode RosterMaintenanceMode
	// Offerings feed the roster effectively (active sources when the source
	// rule is intact, plus active linked offerings), in source order first.
	Offerings []RosterMaintenanceOffering
	// GradeLevels / SchoolClasses restrict the source offerings; empty when
	// no source feeds the roster.
	GradeLevels   []int
	SchoolClasses []string
	// InactiveOfferings are configured sources or linked offerings that are
	// switched off. One inactive source stops the whole source rule: the
	// resync rejects it. An inactive link also prevents it from maintaining
	// later approvals.
	InactiveOfferings []RosterMaintenanceOffering
	// InvalidOfferings are active configured sources or linked offerings that
	// resync cannot use.
	InvalidOfferings []RosterMaintenanceOffering
	// DynamicTargetsManual is true when a Klasse/Jahrgang/Gruppe target
	// exists; children joining it later are not added to existing occurrences.
	DynamicTargetsManual bool
	// CareOfferingsDisabled is true when configured offerings exist but the
	// school switched Betreuungsangebote off.
	CareOfferingsDisabled bool
}

// DeriveTemplateRosterMaintenance maps the facts to the indicator. A class,
// grade or group target alone never counts as automatic: those children are
// resolved when an occurrence is created and never added afterwards.
func DeriveTemplateRosterMaintenance(in TemplateRosterMaintenanceInput) TemplateRosterMaintenance {
	result := TemplateRosterMaintenance{
		Mode:                 RosterMaintenanceManual,
		DynamicTargetsManual: in.HasDynamicTargets,
	}
	configured := len(in.Feeds.Sources) > 0 || len(in.Feeds.LinkedOfferings) > 0
	if !in.Feeds.CareOfferingsEnabled {
		result.CareOfferingsDisabled = configured
		return result
	}

	sourceIntact := len(in.Feeds.Sources) > 0
	inactiveSeen := make(map[int64]bool)
	invalidSeen := make(map[int64]bool)
	for _, source := range in.Feeds.Sources {
		if !source.IsActive {
			sourceIntact = false
			result.InactiveOfferings = append(result.InactiveOfferings, RosterMaintenanceOffering{ID: source.ID, Name: source.Name})
			inactiveSeen[source.ID] = true
			continue
		}
		if source.IsInvalid {
			sourceIntact = false
			result.InvalidOfferings = append(result.InvalidOfferings, RosterMaintenanceOffering{ID: source.ID, Name: source.Name})
			invalidSeen[source.ID] = true
		}
	}
	seen := make(map[int64]bool)
	if sourceIntact {
		for _, source := range in.Feeds.Sources {
			seen[source.ID] = true
			result.Offerings = append(result.Offerings, RosterMaintenanceOffering{ID: source.ID, Name: source.Name})
		}
		result.GradeLevels = append([]int(nil), in.SourceGradeLevels...)
		result.SchoolClasses = append([]string(nil), in.SourceSchoolClasses...)
	}
	for _, linked := range in.Feeds.LinkedOfferings {
		if !linked.IsActive {
			if !inactiveSeen[linked.ID] {
				result.InactiveOfferings = append(result.InactiveOfferings, RosterMaintenanceOffering{ID: linked.ID, Name: linked.Name})
				inactiveSeen[linked.ID] = true
			}
			continue
		}
		if linked.IsInvalid {
			if !invalidSeen[linked.ID] {
				result.InvalidOfferings = append(result.InvalidOfferings, RosterMaintenanceOffering{ID: linked.ID, Name: linked.Name})
				invalidSeen[linked.ID] = true
			}
			continue
		}
		if seen[linked.ID] {
			continue
		}
		seen[linked.ID] = true
		result.Offerings = append(result.Offerings, RosterMaintenanceOffering{ID: linked.ID, Name: linked.Name})
	}

	if len(result.Offerings) == 0 {
		return result
	}
	if in.HasDynamicTargets || len(result.InactiveOfferings) > 0 || len(result.InvalidOfferings) > 0 {
		result.Mode = RosterMaintenancePartial
		return result
	}
	result.Mode = RosterMaintenanceAutomatic
	return result
}

// TemplateRosterFeedQuery identifies one template and its stored source ids.
type TemplateRosterFeedQuery struct {
	TemplateID            int64
	CalendarPeriodID      *int64
	SourceCareOfferingIDs []int64
}

// TemplateRosterMaintenanceFeeds resolves, for every requested template, the
// source offerings, the offerings linked to any segment of its split series,
// and the care-offerings setting. It costs a constant number of reads
// regardless of how many templates are asked for.
func (s *decisionService) TemplateRosterMaintenanceFeeds(ctx context.Context, templates []TemplateRosterFeedQuery) (map[int64]TemplateRosterFeeds, error) {
	result := make(map[int64]TemplateRosterFeeds, len(templates))
	if len(templates) == 0 {
		return result, nil
	}
	if s.CareOfferingRepo == nil || s.ActivityGroupRepo == nil {
		return nil, fmt.Errorf("template roster maintenance: repositories are not configured")
	}
	enabled, err := s.resolveDecisionBool(ctx, configModel.KeyEnrollmentCareOfferingsEnabled, true)
	if err != nil {
		return nil, fmt.Errorf("template roster maintenance: resolve care offerings setting: %w", err)
	}
	offerings, err := s.CareOfferingRepo.ListByTenant(ctx)
	if err != nil {
		return nil, fmt.Errorf("template roster maintenance: list offerings: %w", err)
	}

	offeringsByID := make(map[int64]*enrollmentModels.CareOffering, len(offerings))
	validationOfferingIDs := make(map[int64]bool)
	for _, template := range templates {
		for _, id := range template.SourceCareOfferingIDs {
			validationOfferingIDs[id] = true
		}
	}
	groupIDs := make([]int64, 0, len(templates)+len(offerings))
	for _, template := range templates {
		groupIDs = append(groupIDs, template.TemplateID)
	}
	for _, offering := range offerings {
		if offering == nil {
			continue
		}
		offeringsByID[offering.ID] = offering
		if offering.ActivityGroupID != nil {
			groupIDs = append(groupIDs, *offering.ActivityGroupID)
			validationOfferingIDs[offering.ID] = true
		}
	}
	invalidOfferings := make(map[int64]map[int64]bool)
	periodsByID := make(map[int64]*scheduleModels.CalendarPeriod)
	phasesByID := make(map[int64]*enrollmentOwner.Phase)
	if len(validationOfferingIDs) > 0 {
		if s.Phases == nil || s.CalendarPeriodRepo == nil {
			return nil, fmt.Errorf("template roster maintenance: source validation repositories are not configured")
		}
		periods, err := s.CalendarPeriodRepo.FindByTenantID(ctx)
		if err != nil {
			return nil, fmt.Errorf("template roster maintenance: list calendar periods: %w", err)
		}
		periodsByID = make(map[int64]*scheduleModels.CalendarPeriod, len(periods))
		for _, period := range periods {
			if period != nil {
				periodsByID[period.ID] = period
			}
		}
		phaseIDs := make([]int64, 0, len(validationOfferingIDs))
		for id := range validationOfferingIDs {
			if offering := offeringsByID[id]; offering != nil {
				phaseIDs = append(phaseIDs, offering.PhaseID)
			}
		}
		slices.Sort(phaseIDs)
		phaseIDs = slices.Compact(phaseIDs)
		phases, err := s.Phases.PhasesByID(ctx, phaseIDs)
		if err != nil {
			return nil, fmt.Errorf("template roster maintenance: load offering phases: %w", err)
		}
		phasesByID = make(map[int64]*enrollmentOwner.Phase, len(phases))
		for _, phase := range phases {
			if phase != nil {
				phasesByID[phase.ID] = phase
			}
		}
		for _, query := range templates {
			if len(query.SourceCareOfferingIDs) == 0 {
				continue
			}
			var period *scheduleModels.CalendarPeriod
			if query.CalendarPeriodID != nil {
				period = periodsByID[*query.CalendarPeriodID]
				if period == nil {
					return nil, fmt.Errorf("template roster maintenance: calendar period %d not found", *query.CalendarPeriodID)
				}
			}
			var sourcePhaseID int64
			mixedPhases := false
			for _, id := range query.SourceCareOfferingIDs {
				offering := offeringsByID[id]
				if offering == nil {
					continue
				}
				invalid := phasesByID[offering.PhaseID] == nil
				if sourcePhaseID != 0 && sourcePhaseID != offering.PhaseID {
					mixedPhases = true
					invalid = true
				}
				if sourcePhaseID == 0 {
					sourcePhaseID = offering.PhaseID
				}
				if !invalid && phaseWithinTemplatePeriod(phasesByID[offering.PhaseID], period) != nil {
					invalid = true
				}
				if invalid {
					if invalidOfferings[query.TemplateID] == nil {
						invalidOfferings[query.TemplateID] = make(map[int64]bool)
					}
					invalidOfferings[query.TemplateID][id] = true
				}
			}
			if mixedPhases {
				if invalidOfferings[query.TemplateID] == nil {
					invalidOfferings[query.TemplateID] = make(map[int64]bool)
				}
				for _, id := range query.SourceCareOfferingIDs {
					if offeringsByID[id] != nil {
						invalidOfferings[query.TemplateID][id] = true
					}
				}
			}
		}
	}
	slices.Sort(groupIDs)
	groupIDs = slices.Compact(groupIDs)
	groups, err := s.ActivityGroupRepo.FindByIDs(ctx, groupIDs)
	if err != nil {
		return nil, fmt.Errorf("template roster maintenance: load series roots: %w", err)
	}
	// Split segments share one root: the original keeps a NULL root and every
	// successor points at it. The legacy fan-out drafts on every segment, so
	// a link on any segment feeds the whole series.
	rootByGroup := make(map[int64]int64, len(groups))
	for _, group := range groups {
		if group == nil {
			continue
		}
		root := group.ID
		if group.SeriesRootID != nil {
			root = *group.SeriesRootID
		}
		rootByGroup[group.ID] = root
	}
	linkedByRoot := make(map[int64][]TemplateRosterFeedOffering)
	for _, offering := range offerings {
		if offering == nil || offering.ActivityGroupID == nil {
			continue
		}
		root, ok := rootByGroup[*offering.ActivityGroupID]
		if !ok {
			continue
		}
		linkedByRoot[root] = append(linkedByRoot[root], feedOffering(offering))
	}

	for _, query := range templates {
		var period *scheduleModels.CalendarPeriod
		root, ok := rootByGroup[query.TemplateID]
		if !ok {
			root = query.TemplateID
		}
		linkedOfferings := linkedByRoot[root]
		if len(linkedOfferings) > 0 && query.CalendarPeriodID != nil {
			period = periodsByID[*query.CalendarPeriodID]
			if period == nil {
				return nil, fmt.Errorf("template roster maintenance: calendar period %d not found", *query.CalendarPeriodID)
			}
		}
		for _, linked := range linkedOfferings {
			offering := offeringsByID[linked.ID]
			if offering == nil {
				continue
			}
			if phasesByID[offering.PhaseID] == nil || phaseWithinTemplatePeriod(phasesByID[offering.PhaseID], period) != nil {
				if invalidOfferings[query.TemplateID] == nil {
					invalidOfferings[query.TemplateID] = make(map[int64]bool)
				}
				invalidOfferings[query.TemplateID][linked.ID] = true
			}
		}

		feeds := TemplateRosterFeeds{CareOfferingsEnabled: enabled}
		for _, id := range query.SourceCareOfferingIDs {
			if offering, ok := offeringsByID[id]; ok {
				feed := feedOffering(offering)
				feed.IsInvalid = invalidOfferings[query.TemplateID][id]
				feeds.Sources = append(feeds.Sources, feed)
			}
		}
		for _, linked := range linkedOfferings {
			linked.IsInvalid = invalidOfferings[query.TemplateID][linked.ID]
			feeds.LinkedOfferings = append(feeds.LinkedOfferings, linked)
		}
		result[query.TemplateID] = feeds
	}
	return result, nil
}

func feedOffering(offering *enrollmentModels.CareOffering) TemplateRosterFeedOffering {
	return TemplateRosterFeedOffering{ID: offering.ID, Name: offering.Name, IsActive: offering.IsActive}
}
