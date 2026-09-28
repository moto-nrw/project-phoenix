package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// careEnrollmentDraft is the roster row one child's bookings plan on one
// activity group before it is written.
type careEnrollmentDraft struct {
	activityGroupID  int64
	calendarPeriodID *int64
	selectedWeekday  map[int]bool
	allWeekdays      bool
	// legacyOwned marks a draft written by the legacy ActivityGroupID feed
	// (#1651). The sourced feed never merges into such a draft: the explicit
	// legacy link owns the child's row on that template, and mixing both
	// feeds' weekdays into one row would leave the resync unable to tell
	// which days belong to which feed (#2147 review).
	legacyOwned bool
	// scheduleValidFrom/Until bound the draft's rows to the template segment's
	// recurrence envelope (#2147 review): a split predecessor must not receive
	// rows covering dates after its capped valid_until, nor a successor rows
	// before its start. Only the sourced-template feed sets them; the legacy
	// ActivityGroupID feed keeps its pre-#2137 phase-wide rows (nil = open).
	scheduleValidFrom  *calendar.Date
	scheduleValidUntil *calendar.Date
	// linkValidFrom/Until bound a sourced draft's rows to the window the
	// child's offering link is actually valid in (#2147 review): a dated
	// switch into or out of the offering must not plan the child outside it.
	// The legacy feed leaves them nil — its dated flows cap rows via the
	// adjustment split instead.
	linkValidFrom  *calendar.Date
	linkValidUntil *calendar.Date
	// studentValidUntil is the child's exclusive care end. Unlike link bounds,
	// it also constrains the legacy feed, so a later resync cannot recreate
	// rows after a completed care exit.
	studentValidUntil *calendar.Date
}

// draftSources are the templates and schedules the sourced feed drafts on,
// loaded once per selection.
type draftSources struct {
	byOffering map[int64][]ports.SourcedTemplate
	schedules  map[int64][]careplan.LinkedSchedule
}

func (m *BookingMaterialization) buildCareEnrollmentDrafts(
	ctx context.Context,
	requestChildID, studentID int64,
	links []*careplan.BookedOffering,
	offerings []careplan.CareOffering,
	phase careplan.OfferingPhase,
) (map[int64]*careEnrollmentDraft, map[int64]ports.SourcedTemplate, error) {
	offeringByID := offeringsByID(offerings)
	gradeLevel, studentValidUntil, err := m.studentCareDraftBounds(ctx, studentID)
	if err != nil {
		return nil, nil, err
	}
	sources, err := m.loadSourcedTemplateDraftInputs(ctx, bookedOfferingIDs(links))
	if err != nil {
		return nil, nil, err
	}
	drafts := make(map[int64]*careEnrollmentDraft)
	// Templates sourcing several offerings are not drafted per link — see
	// addSourcedTemplateDrafts — the caller resyncs them after persisting.
	multiSource := make(map[int64]ports.SourcedTemplate)
	for _, link := range links {
		offering := offeringByID[link.CareOfferingID]
		if offering == nil {
			m.deps.Logger.Warn("decision: care offering missing for child link",
				slog.Int64("request_child_id", requestChildID),
				slog.Int64("care_offering_id", link.CareOfferingID))
			continue
		}
		if err := m.addLegacyLinkedGroupDrafts(ctx, drafts, offering, link, phase); err != nil {
			return nil, nil, err
		}
		feed := sourcedDraftFeed{offering: offering, link: link, phase: phase, gradeLevel: gradeLevel}
		if err := m.addSourcedTemplateDrafts(ctx, drafts, multiSource, feed, sources); err != nil {
			return nil, nil, err
		}
	}
	for _, draft := range drafts {
		draft.studentValidUntil = cloneOptionalDate(studentValidUntil)
	}
	return drafts, multiSource, nil
}

func (m *BookingMaterialization) loadSourcedTemplateDraftInputs(ctx context.Context, offeringIDs []int64) (draftSources, error) {
	templates, err := m.deps.Templates.TemplatesSourcedFrom(ctx, offeringIDs)
	if err != nil {
		return draftSources{}, fmt.Errorf("decision: list sourced templates: %w", err)
	}
	groupIDs := make([]int64, 0, len(templates))
	for _, template := range templates {
		groupIDs = append(groupIDs, template.ID)
	}
	var schedules []careplan.LinkedSchedule
	if len(groupIDs) > 0 {
		schedules, err = m.catalog.deps.Timetable.GroupSchedules(ctx, groupIDs)
		if err != nil {
			return draftSources{}, fmt.Errorf("decision: load sourced template schedules: %w", err)
		}
	}
	return draftSources{byOffering: sourcedTemplatesByOffering(templates), schedules: linkedSchedulesByGroup(schedules)}, nil
}

func sourcedTemplatesByOffering(templates []ports.SourcedTemplate) map[int64][]ports.SourcedTemplate {
	result := make(map[int64][]ports.SourcedTemplate)
	for _, template := range templates {
		for _, offeringID := range template.SourceCareOfferingIDs {
			result[offeringID] = append(result[offeringID], template)
		}
	}
	return result
}

// studentCareDraftBounds derives the child's grade filter and exclusive care
// end. A missing row yields open bounds for legacy compatibility.
func (m *BookingMaterialization) studentCareDraftBounds(ctx context.Context, studentID int64) (*int16, *calendar.Date, error) {
	if studentID <= 0 {
		return nil, nil, nil
	}
	student, err := m.deps.Students.Student(ctx, studentID, false)
	if err != nil {
		if errors.Is(err, ports.ErrBookingRowNotFound) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("decision: load student for care draft bounds: %w", err)
	}
	var validUntil *calendar.Date
	if student.EnrolledUntil != nil {
		end := student.EnrolledUntil.AddDays(1)
		validUntil = &end
	}
	return student.GradeLevel, validUntil, nil
}

// addLegacyLinkedGroupDrafts is the pre-#2137 offering→template feed: the one
// activity group the offering points at via ActivityGroupID.
func (m *BookingMaterialization) addLegacyLinkedGroupDrafts(
	ctx context.Context,
	drafts map[int64]*careEnrollmentDraft,
	offering *careplan.CareOffering,
	link *careplan.BookedOffering,
	phase careplan.OfferingPhase,
) error {
	if offering.ActivityGroupID == nil || *offering.ActivityGroupID == 0 {
		return nil
	}
	segments, err := m.catalog.ResolveLinkedSegments(ctx, *offering.ActivityGroupID, phase)
	if err != nil {
		return fmt.Errorf("decision: validate linked activity group for care offering %d: %w", link.CareOfferingID, err)
	}
	isTemplate := len(segments) > 0 && segments[0].Group.IsTemplate
	if isTemplate && len(offering.AvailableDays) == 0 && len(link.SelectedDays) == 0 {
		return fmt.Errorf("decision: care offering %d links to a timetable template but has no selected or available days", link.CareOfferingID)
	}
	days, err := effectiveOfferingDaysForEnrollment(offering, link)
	if err != nil {
		return fmt.Errorf("decision: resolve selected days for care offering %d: %w", link.CareOfferingID, err)
	}
	if err := m.catalog.ValidateMaterializable(ctx, segments, phase, days); err != nil {
		return fmt.Errorf("decision: care offering %d is not materializable: %w", link.CareOfferingID, err)
	}
	return addLegacySegmentDrafts(drafts, segments, days, link.CareOfferingID)
}

// addLegacySegmentDrafts drafts the legacy link's days on every segment of
// its series.
func addLegacySegmentDrafts(drafts map[int64]*careEnrollmentDraft, segments []careplan.LinkedSegment, days []string, offeringID int64) error {
	for _, segment := range segments {
		// The explicit legacy link owns the child's row on this template. When
		// another link's sourced feed drafted it first, that draft is REPLACED,
		// not merged: one row mixing both feeds' weekdays could never be told
		// apart again by the resync's provenance check (#2147 review).
		if draft := drafts[segment.Group.ID]; draft != nil && !draft.legacyOwned {
			delete(drafts, segment.Group.ID)
		}
		if err := mergeCareEnrollmentDraft(drafts, segment, days, offeringID); err != nil {
			return err
		}
		// The legacy feed plans phase-wide rows on every overlapping segment
		// (pre-#2137 behavior).
		if draft := drafts[segment.Group.ID]; draft != nil {
			draft.legacyOwned = true
			draft.scheduleValidFrom, draft.scheduleValidUntil = nil, nil
			draft.linkValidFrom, draft.linkValidUntil = nil, nil
		}
	}
	return nil
}

// sourcedDraftFeed is one booking link feeding the templates that source its
// offering.
type sourcedDraftFeed struct {
	offering   *careplan.CareOffering
	link       *careplan.BookedOffering
	phase      careplan.OfferingPhase
	gradeLevel *int16
}

// addSourcedTemplateDrafts is the #2137 feed: every template that declares
// this offering as its roster source pulls the child in when the template's
// Jahrgang filter matches. Misconfigured templates (no schedules, no
// resolvable period, period drifted away from the phase) are skipped with a
// warning instead of failing the approval — the editor surfaces the mismatch,
// and blocking every approval on one broken template would be worse.
//
// Templates sourcing SEVERAL offerings are not drafted here: the per-link
// draft merge cannot express the union's subtraction shape (a child linked to
// two of the template's sources would get one row mixing both links' weekdays
// under the first link's window). They are collected into multiSource and
// reconciled through ResyncTemplateOfferingRoster after the drafts persist —
// one authoritative union writer instead of two diverging ones.
func (m *BookingMaterialization) addSourcedTemplateDrafts(
	ctx context.Context,
	drafts map[int64]*careEnrollmentDraft,
	multiSource map[int64]ports.SourcedTemplate,
	feed sourcedDraftFeed,
	sources draftSources,
) error {
	templates := sources.byOffering[feed.offering.ID]
	if len(templates) == 0 {
		return nil
	}
	days, err := effectiveOfferingDaysForEnrollment(feed.offering, feed.link)
	if err != nil {
		return fmt.Errorf("decision: resolve selected days for care offering %d: %w", feed.link.CareOfferingID, err)
	}
	for _, tmpl := range templates {
		if !gradeFilterMatches(tmpl.SourceGradeLevels, feed.gradeLevel) {
			continue
		}
		if len(tmpl.SourceCareOfferingIDs) > 1 {
			multiSource[tmpl.ID] = tmpl
			continue
		}
		segment, ok := m.sourcedTemplateSegment(ctx, tmpl, feed, sources.schedules[tmpl.ID])
		if !ok {
			continue
		}
		if err := addSourcedSegmentDraft(drafts, segment, sources.schedules[tmpl.ID], feed, days); err != nil {
			return err
		}
	}
	return nil
}

// sourcedTemplateSegment resolves a single-source template into the segment
// the feed drafts on. ok is false for a template the approval skips.
func (m *BookingMaterialization) sourcedTemplateSegment(
	ctx context.Context,
	tmpl ports.SourcedTemplate,
	feed sourcedDraftFeed,
	schedules []careplan.LinkedSchedule,
) (careplan.LinkedSegment, bool) {
	if len(schedules) == 0 || !careplan.SchedulesOverlapPhase(schedules, feed.phase) {
		m.logSkippedSourcedTemplate(tmpl.ID, []int64{feed.offering.ID}, "no schedule overlaps the enrollment phase", nil)
		return careplan.LinkedSegment{}, false
	}
	group := careplan.LinkedGroup{ID: tmpl.ID, IsTemplate: true, CalendarPeriodID: tmpl.CalendarPeriodID}
	period, err := m.catalog.ResolveTemplatePeriod(ctx, group)
	if err != nil {
		m.logSkippedSourcedTemplate(tmpl.ID, []int64{feed.offering.ID}, "calendar period not resolvable", err)
		return careplan.LinkedSegment{}, false
	}
	if err := careplan.ValidatePhaseWithinPeriod(feed.phase, &period); err != nil {
		m.logSkippedSourcedTemplate(tmpl.ID, []int64{feed.offering.ID}, "phase outside template period", err)
		return careplan.LinkedSegment{}, false
	}
	return careplan.LinkedSegment{Group: group, Period: &period, Schedules: schedules}, true
}

func addSourcedSegmentDraft(
	drafts map[int64]*careEnrollmentDraft,
	segment careplan.LinkedSegment,
	schedules []careplan.LinkedSchedule,
	feed sourcedDraftFeed,
	days []string,
) error {
	templateID := segment.Group.ID
	if existing := drafts[templateID]; existing != nil && existing.legacyOwned {
		// The legacy ActivityGroupID feed already plans this child on the
		// template and owns the row (see addLegacyLinkedGroupDrafts) — the
		// sourced feed never merges into it.
		return nil
	}
	_, existed := drafts[templateID]
	if err := mergeCareEnrollmentDraft(drafts, segment, days, feed.link.CareOfferingID); err != nil {
		return err
	}
	if existed {
		return nil
	}
	// Every segment of a split series sources the offering, so the drafted
	// rows must stop at each segment's schedule envelope — otherwise an
	// approval after a split plans the capped predecessor for dates after it
	// ended (#2147 review). The child's offering-link window bounds the rows
	// the same way: a link ending mid-phase must not plan the child after
	// leaving the offering.
	draft := drafts[templateID]
	draft.scheduleValidFrom, draft.scheduleValidUntil = scheduleValidityBounds(schedules)
	draft.linkValidFrom = cloneOptionalDate(feed.link.ValidFrom)
	draft.linkValidUntil = cloneOptionalDate(feed.link.ValidUntil)
	return nil
}

func mergeCareEnrollmentDraft(
	drafts map[int64]*careEnrollmentDraft,
	segment careplan.LinkedSegment,
	days []string,
	offeringID int64,
) error {
	var periodID *int64
	if segment.Period != nil {
		periodID = &segment.Period.ID
	}
	draft := drafts[segment.Group.ID]
	if draft != nil && !sameOptionalInt64(draft.calendarPeriodID, periodID) {
		return fmt.Errorf("decision: care offering %d resolves to conflicting calendar_period_id", offeringID)
	}
	if draft == nil {
		draft = &careEnrollmentDraft{
			activityGroupID:  segment.Group.ID,
			calendarPeriodID: periodID,
			selectedWeekday:  make(map[int]bool),
		}
		drafts[segment.Group.ID] = draft
	}
	if !segment.Group.IsTemplate || len(days) == 0 {
		draft.allWeekdays = true
		return nil
	}
	if draft.allWeekdays {
		return nil
	}
	for _, day := range days {
		weekday, ok := offeringDayWeekday(day)
		if !ok {
			return fmt.Errorf("decision: invalid selected day %q for care offering %d", day, offeringID)
		}
		draft.selectedWeekday[weekday] = true
	}
	return nil
}

func sameOptionalInt64(left, right *int64) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}

func effectiveOfferingDaysForEnrollment(offering *careplan.CareOffering, link *careplan.BookedOffering) ([]string, error) {
	if len(link.SelectedDays) > 0 {
		return link.SelectedDays, nil
	}
	switch offering.DaysOfWeekMode {
	case daysOfWeekModeFixed:
		return offering.AvailableDays, nil
	case daysOfWeekModeParentChoice:
		return nil, fmt.Errorf("parent-choice offering has no selected_days")
	default:
		return nil, fmt.Errorf("unknown days_of_week_mode %q", offering.DaysOfWeekMode)
	}
}

func sortedWeekdaySet(days map[int]bool) []int {
	out := make([]int, 0, len(days))
	for day := 1; day <= 7; day++ {
		if days[day] {
			out = append(out, day)
		}
	}
	return out
}

// scheduleValidityBounds returns the union recurrence envelope of a template's
// schedule rows. A side is nil (open) when any row is open on that side;
// otherwise the widest bound wins. Segment rows share one envelope per the
// split-series invariant — the union keeps drifted rows honest instead of
// trusting an arbitrary one.
func scheduleValidityBounds(schedules []careplan.LinkedSchedule) (validFrom, validUntil *calendar.Date) {
	fromOpen, untilOpen := false, false
	for _, sch := range schedules {
		switch {
		case sch.ValidFrom == nil:
			fromOpen, validFrom = true, nil
		case !fromOpen && (validFrom == nil || sch.ValidFrom.Before(*validFrom)):
			validFrom = cloneOptionalDate(sch.ValidFrom)
		}
		switch {
		case sch.ValidUntil == nil:
			untilOpen, validUntil = true, nil
		case !untilOpen && (validUntil == nil || sch.ValidUntil.After(*validUntil)):
			validUntil = cloneOptionalDate(sch.ValidUntil)
		}
	}
	return validFrom, validUntil
}

// gradeFilterMatches reports whether a child with the given grade level (nil
// = not derivable from the school class) passes a template's Jahrgang
// filter. An empty filter admits every child; a set filter never admits a
// child without grade data.
func gradeFilterMatches(levels []int, grade *int16) bool {
	if len(levels) == 0 {
		return true
	}
	if grade == nil {
		return false
	}
	for _, level := range levels {
		if level == int(*grade) {
			return true
		}
	}
	return false
}

// sourceClassFilterMatches reports whether a child's school class passes a
// template's Klassenfilter (#2482). An empty filter admits every child; a set
// filter matches case- and whitespace-insensitively and never admits a child
// without a class — silently planning a child with no class data into a
// Klassen-Termin would hide data problems, the same rule the Jahrgang filter
// applies to a missing grade.
func sourceClassFilterMatches(classes []string, schoolClass string) bool {
	if len(classes) == 0 {
		return true
	}
	wanted := strings.ToLower(strings.TrimSpace(schoolClass))
	if wanted == "" {
		return false
	}
	for _, class := range classes {
		if strings.ToLower(strings.TrimSpace(class)) == wanted {
			return true
		}
	}
	return false
}
