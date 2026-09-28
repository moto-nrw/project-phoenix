package careplan

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

// TemplateRosterFeedQuery identifies one template and its stored source ids.
type TemplateRosterFeedQuery struct {
	TemplateID            int64
	CalendarPeriodID      *int64
	SourceCareOfferingIDs []int64
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
	marks := rosterFeedMarks{inactive: make(map[int64]bool), invalid: make(map[int64]bool)}
	seen := make(map[int64]bool)
	if marks.classifySources(&result, in.Feeds.Sources) {
		for _, source := range in.Feeds.Sources {
			seen[source.ID] = true
			result.Offerings = append(result.Offerings, RosterMaintenanceOffering{ID: source.ID, Name: source.Name})
		}
		result.GradeLevels = append([]int(nil), in.SourceGradeLevels...)
		result.SchoolClasses = append([]string(nil), in.SourceSchoolClasses...)
	}
	marks.addLinkedOfferings(&result, in.Feeds.LinkedOfferings, seen)
	result.Mode = rosterMaintenanceMode(result, in.HasDynamicTargets)
	return result
}

// rosterFeedMarks remembers which offerings the explanation already names as
// inactive or invalid, so a source that is also linked is named once.
type rosterFeedMarks struct {
	inactive map[int64]bool
	invalid  map[int64]bool
}

// classifySources names the inactive and invalid sources and reports whether
// the source rule is intact: one inactive or invalid source stops it.
func (m rosterFeedMarks) classifySources(result *TemplateRosterMaintenance, sources []TemplateRosterFeedOffering) bool {
	intact := len(sources) > 0
	for _, source := range sources {
		offering := RosterMaintenanceOffering{ID: source.ID, Name: source.Name}
		if !source.IsActive {
			intact = false
			result.InactiveOfferings = append(result.InactiveOfferings, offering)
			m.inactive[source.ID] = true
			continue
		}
		if source.IsInvalid {
			intact = false
			result.InvalidOfferings = append(result.InvalidOfferings, offering)
			m.invalid[source.ID] = true
		}
	}
	return intact
}

// addLinkedOfferings adds the linked offerings that feed the roster and names
// the inactive and invalid ones not yet named.
func (m rosterFeedMarks) addLinkedOfferings(result *TemplateRosterMaintenance, linked []TemplateRosterFeedOffering, seen map[int64]bool) {
	for _, feed := range linked {
		offering := RosterMaintenanceOffering{ID: feed.ID, Name: feed.Name}
		switch {
		case !feed.IsActive:
			if !m.inactive[feed.ID] {
				result.InactiveOfferings = append(result.InactiveOfferings, offering)
				m.inactive[feed.ID] = true
			}
		case feed.IsInvalid:
			if !m.invalid[feed.ID] {
				result.InvalidOfferings = append(result.InvalidOfferings, offering)
				m.invalid[feed.ID] = true
			}
		case !seen[feed.ID]:
			seen[feed.ID] = true
			result.Offerings = append(result.Offerings, offering)
		}
	}
}

func rosterMaintenanceMode(result TemplateRosterMaintenance, hasDynamicTargets bool) RosterMaintenanceMode {
	if len(result.Offerings) == 0 {
		return RosterMaintenanceManual
	}
	if hasDynamicTargets || len(result.InactiveOfferings) > 0 || len(result.InvalidOfferings) > 0 {
		return RosterMaintenancePartial
	}
	return RosterMaintenanceAutomatic
}
