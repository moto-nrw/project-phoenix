package compose

import (
	"context"

	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// The Vorlagen list (#1839): every schedule row of a template carries the
// template's display roster, its dynamic targets and the roster of its worst
// occurrence, so the list shows the staffing shortfall a block will really
// have.

func (d *timetableData) ListTemplateEntries(ctx context.Context, templateID *int64, childrenPerStaffRatio int) ([]timetable.TemplateListEntry, error) {
	rows, err := d.deps.Templates.ListTemplateRows(ctx, templateID)
	if err != nil {
		return nil, err
	}
	// Detail has no period parameter, so every active period is evaluated;
	// the read still applies materialization's deterministic period choice
	// for globally unpinned templates on overlapping dates.
	return d.templateEntries(ctx, rows, nil, distinctTemplateIDs(rows), childrenPerStaffRatio)
}

// ListTemplateEntriesForTemplatePeriod is the detail read model of one
// editable template period, with period-scoped roster and capacity.
func (d *timetableData) ListTemplateEntriesForTemplatePeriod(ctx context.Context, templateID, periodID int64, childrenPerStaffRatio int) ([]timetable.TemplateListEntry, error) {
	rows, err := d.deps.Templates.ListTemplateRowsForTemplatePeriod(ctx, templateID, periodID)
	if err != nil {
		return nil, err
	}
	return d.templateEntries(ctx, rows, &periodID, []int64{templateID}, childrenPerStaffRatio)
}

func (d *timetableData) ListTemplateEntriesForPeriod(ctx context.Context, periodID *int64, childrenPerStaffRatio int) ([]timetable.TemplateListEntry, error) {
	rows, err := d.deps.Templates.ListTemplateRowsForPeriod(ctx, periodID)
	if err != nil {
		return nil, err
	}
	return d.templateEntries(ctx, rows, periodID, distinctTemplateIDs(rows), childrenPerStaffRatio)
}

// ListTemplateWeekdayRoster is a thin read: grouping the rows into weekday
// assignments is a presentation concern of the caller.
func (d *timetableData) ListTemplateWeekdayRoster(ctx context.Context, templateID, calendarPeriodID *int64) ([]timetable.TemplateWeekdayRosterRow, error) {
	rows, err := d.deps.Templates.ListTemplateWeekdayRoster(ctx, templateID, calendarPeriodID)
	if err != nil {
		return nil, err
	}
	roster := make([]timetable.TemplateWeekdayRosterRow, 0, len(rows))
	for _, row := range rows {
		roster = append(roster, timetable.TemplateWeekdayRosterRow{
			TemplateID: row.TemplateID, Weekday: row.Weekday, Kind: row.Kind, PersonID: row.PersonID, IsPrimary: row.IsPrimary,
		})
	}
	return roster, nil
}

func (d *timetableData) templateEntries(ctx context.Context, rows []activitiesModels.TemplateListRow, periodID *int64, templateIDs []int64, childrenPerStaffRatio int) ([]timetable.TemplateListEntry, error) {
	setDisplayRosterCapacity(rows)
	if err := d.attachTemplateTargets(ctx, rows); err != nil {
		return nil, err
	}
	if len(templateIDs) > 0 {
		occurrences, err := d.deps.Templates.ListTemplateCapacityOccurrences(ctx, periodID, templateIDs)
		if err != nil {
			return nil, err
		}
		applyWorstTemplateCapacity(rows, templateIDs, occurrences, childrenPerStaffRatio)
	}
	entries := make([]timetable.TemplateListEntry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, templateListEntryOf(row))
	}
	return entries, nil
}

// attachTemplateTargets adds the dynamic targets and counts a target's
// children into the template's roster.
func (d *timetableData) attachTemplateTargets(ctx context.Context, rows []activitiesModels.TemplateListRow) error {
	templateIDs := distinctTemplateIDs(rows)
	targetsByGroup, err := d.deps.Templates.FindTargetsByGroupIDs(ctx, templateIDs)
	if err != nil {
		return err
	}
	if !hasTemplateTargets(targetsByGroup) {
		return nil
	}
	targetStudentsByGroup, err := d.deps.Templates.FindTargetStudentIDsByGroupIDs(ctx, templateIDs)
	if err != nil {
		return err
	}
	for i := range rows {
		rows[i].Targets = targetsByGroup[rows[i].TemplateID]
		rows[i].EnrollmentCount = unionCount(rows[i].StudentIDs, targetStudentsByGroup[rows[i].TemplateID])
	}
	return nil
}

func hasTemplateTargets(targetsByGroup map[int64][]*activitiesModels.GroupTarget) bool {
	for _, targets := range targetsByGroup {
		if len(targets) > 0 {
			return true
		}
	}
	return false
}

func unionCount(left, right []int64) int {
	ids := make(map[int64]struct{}, len(left)+len(right))
	for _, id := range left {
		ids[id] = struct{}{}
	}
	for _, id := range right {
		ids[id] = struct{}{}
	}
	return len(ids)
}

func setDisplayRosterCapacity(rows []activitiesModels.TemplateListRow) {
	for i := range rows {
		rows[i].CapacityEnrollmentCount = rows[i].EnrollmentCount
		rows[i].CapacitySupervisorCount = rows[i].SupervisorCount
	}
}

func distinctTemplateIDs(rows []activitiesModels.TemplateListRow) []int64 {
	ids := make([]int64, 0, len(rows))
	seen := make(map[int64]struct{}, len(rows))
	for _, row := range rows {
		if _, exists := seen[row.TemplateID]; exists {
			continue
		}
		seen[row.TemplateID] = struct{}{}
		ids = append(ids, row.TemplateID)
	}
	return ids
}

// applyWorstTemplateCapacity sets each template's capacity counts to its
// worst occurrence. The manual Personalbedarf override (#1839) is
// per-template, so it drives the worst-occurrence choice too: scoring by the
// derived requirement alone could pick a fine-looking occurrence while
// another is understaffed under the override.
func applyWorstTemplateCapacity(rows []activitiesModels.TemplateListRow, templateIDs []int64, occurrences []activitiesModels.TemplateCapacityOccurrence, childrenPerStaffRatio int) {
	included := make(map[int64]struct{}, len(templateIDs))
	for _, templateID := range templateIDs {
		included[templateID] = struct{}{}
	}
	byTemplate := make(map[int64][]activitiesModels.TemplateCapacityOccurrence)
	for _, occurrence := range occurrences {
		if _, ok := included[occurrence.TemplateID]; ok {
			byTemplate[occurrence.TemplateID] = append(byTemplate[occurrence.TemplateID], occurrence)
		}
	}
	overrideByTemplate := make(map[int64]*int, len(byTemplate))
	for i := range rows {
		if _, ok := included[rows[i].TemplateID]; ok {
			overrideByTemplate[rows[i].TemplateID] = requiredStaffOverrideOf(rows[i])
		}
	}
	worst := make(map[int64]activitiesModels.TemplateCapacityOccurrence, len(byTemplate))
	for templateID, candidates := range byTemplate {
		if candidate, ok := worstTemplateOccurrence(candidates, overrideByTemplate[templateID], childrenPerStaffRatio); ok {
			worst[templateID] = candidate
		}
	}
	for i := range rows {
		if _, ok := included[rows[i].TemplateID]; !ok {
			continue
		}
		candidate, found := worst[rows[i].TemplateID]
		rows[i].CapacityOccurrenceFound = found
		rows[i].CapacityEnrollmentCount = candidate.EnrollmentCount
		rows[i].CapacitySupervisorCount = candidate.SupervisorCount
	}
}

// requiredStaffOverrideOf reads a row's nullable required_staff as the
// override timetable.EffectiveRequiredStaff expects (NULL → nil).
func requiredStaffOverrideOf(row activitiesModels.TemplateListRow) *int {
	if !row.RequiredStaff.Valid {
		return nil
	}
	v := int(row.RequiredStaff.Int64)
	return &v
}

// worstTemplateOccurrence skips zero-enrollment occurrences only when the
// requirement is derived: with no children the derived demand is 0. Under a
// manual override the demand is constant, so an empty, unstaffed occurrence
// can legitimately be the worst (e.g. 0/3).
func worstTemplateOccurrence(occurrences []activitiesModels.TemplateCapacityOccurrence, override *int, childrenPerStaffRatio int) (activitiesModels.TemplateCapacityOccurrence, bool) {
	hasPositiveDemand := false
	for _, occurrence := range occurrences {
		if timetable.EffectiveRequiredStaff(override, occurrence.EnrollmentCount, childrenPerStaffRatio) > 0 {
			hasPositiveDemand = true
			break
		}
	}
	var worst activitiesModels.TemplateCapacityOccurrence
	found := false
	for _, candidate := range occurrences {
		if override == nil && hasPositiveDemand && candidate.EnrollmentCount == 0 {
			continue
		}
		if !found || templateOccurrenceIsWorse(candidate, worst, override, childrenPerStaffRatio) {
			worst = candidate
			found = true
		}
	}
	return worst, found
}

type templateCapacityScore struct {
	severity int
	shortage int
	surplus  int
	required int
}

func scoreTemplateOccurrence(occurrence activitiesModels.TemplateCapacityOccurrence, override *int, childrenPerStaffRatio int) templateCapacityScore {
	required := timetable.EffectiveRequiredStaff(override, occurrence.EnrollmentCount, childrenPerStaffRatio)
	assigned := occurrence.SupervisorCount
	score := templateCapacityScore{required: required}
	if assigned < required {
		score.shortage = required - assigned
		if assigned == 0 {
			score.severity = 2 // danger
		} else {
			score.severity = 1 // warning
		}
	} else {
		score.surplus = assigned - required
	}
	return score
}

func templateOccurrenceIsWorse(candidate, current activitiesModels.TemplateCapacityOccurrence, override *int, childrenPerStaffRatio int) bool {
	candidateScore := scoreTemplateOccurrence(candidate, override, childrenPerStaffRatio)
	currentScore := scoreTemplateOccurrence(current, override, childrenPerStaffRatio)
	if candidateScore.severity != currentScore.severity {
		return candidateScore.severity > currentScore.severity
	}
	if candidateScore.shortage != currentScore.shortage {
		return candidateScore.shortage > currentScore.shortage
	}
	if candidateScore.surplus != currentScore.surplus {
		return candidateScore.surplus < currentScore.surplus
	}
	if candidateScore.required != currentScore.required {
		return candidateScore.required > currentScore.required
	}
	return candidate.OccurrenceDate.Before(current.OccurrenceDate)
}

// templateListEntryOf maps a retained template list row onto the owner's
// list entry.
func templateListEntryOf(row activitiesModels.TemplateListRow) timetable.TemplateListEntry {
	entry := timetable.TemplateListEntry{TemplateListRow: timetable.TemplateListRow{
		TemplateID: row.TemplateID, Name: row.Name, Type: row.Type,
		CategoryID: row.CategoryID, CategoryName: row.CategoryName,
		PlanningTrackID: nullInt64(row.PlanningTrackID), PlanningTrackName: row.PlanningTrackName,
		PlanningTrackColor: row.PlanningTrackColor, PlanningTrackOrder: nullInt64(row.PlanningTrackOrder),
		RoomID: nullInt64(row.RoomID), RoomName: nullString(row.RoomName),
		EducationGroupID: nullInt64(row.EducationGroupID), EducationGroupName: nullString(row.EducationGroupName),
		IsOpen: row.IsOpen, MaxParticipants: row.MaxParticipants, RequiredStaff: nullInt64(row.RequiredStaff),
		TemplateCalendarPeriodID: nullInt64(row.TemplateCalendarPeriodID), TargetGroupType: row.TargetGroupType,
		TargetGradeLevel: nullInt16(row.TargetGradeLevel), TargetSchoolClass: nullString(row.TargetSchoolClass),
		SourceCareOfferingIDsJSON: row.SourceCareOfferingIDsJSON, SourceGradeLevelsJSON: row.SourceGradeLevelsJSON,
		SourceSchoolClassesJSON: row.SourceSchoolClassesJSON, ListKind: nullString(row.ListKind), Notes: nullString(row.Notes),
		ShiftTypeID: nullInt64(row.ShiftTypeID), ShiftTypeName: row.ShiftTypeName, ShiftTypeColor: row.ShiftTypeColor,
		EnrollmentCount: row.EnrollmentCount, SupervisorCount: row.SupervisorCount,
		CapacityEnrollmentCount: row.CapacityEnrollmentCount, CapacitySupervisorCount: row.CapacitySupervisorCount,
		CapacityOccurrenceFound: row.CapacityOccurrenceFound, StudentIDs: row.StudentIDs, StaffIDs: row.StaffIDs,
		PrimaryStaffID: nullInt64(row.PrimaryStaffID), ScheduleID: row.ScheduleID, Weekday: row.Weekday,
		StartTime: nullString(row.StartTime), EndTime: nullString(row.EndTime), WeekPattern: row.WeekPattern,
		CalendarPeriodID: nullInt64(row.CalendarPeriodID), ScheduleValidFrom: nullString(row.ScheduleValidFrom),
		ScheduleValidUntil: nullString(row.ScheduleValidUntil),
	}}
	// Corrupt jsonb cannot happen via the write path (validated slices);
	// degrade to "no filter" instead of failing the whole list read.
	if ids, err := row.ParseSourceCareOfferingIDs(); err == nil {
		entry.SourceCareOfferingIDs = ids
	}
	if levels, err := row.ParseSourceGradeLevels(); err == nil {
		entry.SourceGradeLevels = levels
	}
	if classes, err := row.ParseSourceSchoolClasses(); err == nil {
		entry.SourceSchoolClasses = classes
	}
	for _, target := range row.Targets {
		if target == nil {
			continue
		}
		entry.Targets = append(entry.Targets, timetable.GroupTarget{
			ID: target.ID, TenantID: target.TenantID, CreatedAt: target.CreatedAt, UpdatedAt: target.UpdatedAt,
			ActivityGroupID: target.ActivityGroupID, TargetGroupType: target.TargetGroupType,
			TargetGradeLevel: target.TargetGradeLevel, TargetSchoolClass: target.TargetSchoolClass,
			EducationGroupID: target.EducationGroupID, EducationGroupName: target.EducationGroupName,
		})
	}
	return entry
}

func nullInt64(value activitiesModels.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	return &value.Int64
}

func nullInt16(value activitiesModels.NullInt16) *int16 {
	if !value.Valid {
		return nil
	}
	return &value.Int16
}

func nullString(value activitiesModels.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}
