package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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

func (d *timetableData) templateEntries(ctx context.Context, rows []timetable.TemplateListRow, periodID *int64, templateIDs []int64, childrenPerStaffRatio int) ([]timetable.TemplateListEntry, error) {
	if err := d.attachTemplateOwnerNames(ctx, rows); err != nil {
		return nil, err
	}
	setDisplayRosterCapacity(rows)
	targets, err := d.attachTemplateTargets(ctx, rows)
	if err != nil {
		return nil, err
	}
	if len(templateIDs) > 0 {
		occurrences, err := d.templateCapacityOccurrences(ctx, periodID, templateIDs)
		if err != nil {
			return nil, err
		}
		applyWorstTemplateCapacity(rows, templateIDs, occurrences, childrenPerStaffRatio)
	}
	entries := make([]timetable.TemplateListEntry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, templateListEntryOf(row, targets[row.TemplateID]))
	}
	return entries, nil
}

// attachTemplateOwnerNames names the rooms and Schichtarten (#3418) and the
// education groups of the rows, which other owners hold. Every row gets an
// education group name, empty without a group, as the former COALESCE did.
func (d *timetableData) attachTemplateOwnerNames(ctx context.Context, rows []timetable.TemplateListRow) error {
	directories := d.deps.Directories
	if directories.RoomNames == nil || directories.ShiftTypes == nil {
		return errors.New("timetable compatibility: room and shift-type directories are required")
	}
	roomIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		if row.RoomID != nil {
			roomIDs = append(roomIDs, *row.RoomID)
		}
	}
	roomNames, err := directories.RoomNames(ctx, dedupeTemplateIDs(roomIDs))
	if err != nil {
		return err
	}
	shifts, err := directories.ShiftTypes(ctx)
	if err != nil {
		return err
	}
	shiftByID := make(map[int64]struct{ name, color string }, len(shifts))
	for _, shift := range shifts {
		shiftByID[shift.ID] = struct{ name, color string }{shift.Name, shift.Color}
	}
	for index := range rows {
		if rows[index].RoomID != nil {
			name := roomNames[*rows[index].RoomID]
			rows[index].RoomName = &name
		}
		if rows[index].ShiftTypeID != nil {
			shift := shiftByID[*rows[index].ShiftTypeID]
			rows[index].ShiftTypeName, rows[index].ShiftTypeColor = shift.name, shift.color
		}
	}
	pointers := make([]*timetable.TemplateListRow, len(rows))
	for index := range rows {
		pointers[index] = &rows[index]
	}
	_, err = enrichGroupNames(ctx, directories.GroupNames, pointers,
		func(row *timetable.TemplateListRow) int64 { return optionalGroupID(row.EducationGroupID) },
		func(row *timetable.TemplateListRow, name string) { row.EducationGroupName = &name },
		"", "template rows")
	return err
}

// templateCapacityOccurrences reads the staffing snapshot of every
// occurrence of the templates in School Calendar's active periods.
func (d *timetableData) templateCapacityOccurrences(ctx context.Context, periodID *int64, templateIDs []int64) ([]timetable.TemplateCapacityOccurrence, error) {
	if d.deps.Directories.ActivePeriods == nil {
		return nil, errors.New("timetable compatibility: school calendar is required")
	}
	periods, err := d.deps.Directories.ActivePeriods(ctx)
	if err != nil {
		return nil, err
	}
	return d.deps.Templates.ListTemplateCapacityOccurrences(ctx, periodID, templateIDs, periods)
}

// attachTemplateTargets adds the dynamic targets, named with their
// education groups, and counts a target's children into the template's
// roster.
func (d *timetableData) attachTemplateTargets(ctx context.Context, rows []timetable.TemplateListRow) (map[int64][]timetable.GroupTarget, error) {
	templateIDs := distinctTemplateIDs(rows)
	targetsByGroup, err := d.deps.Templates.ListGroupTargets(ctx, templateIDs)
	if err != nil {
		return nil, err
	}
	all := make([]*timetable.GroupTarget, 0, len(targetsByGroup))
	for groupID := range targetsByGroup {
		for index := range targetsByGroup[groupID] {
			all = append(all, &targetsByGroup[groupID][index])
		}
	}
	if _, err := enrichGroupNames(ctx, d.deps.Directories.GroupNames, all,
		func(row *timetable.GroupTarget) int64 { return optionalGroupID(row.EducationGroupID) },
		func(row *timetable.GroupTarget, name string) { row.EducationGroupName = name },
		"", "template targets"); err != nil {
		return nil, err
	}
	if !hasTemplateTargets(targetsByGroup) {
		return targetsByGroup, nil
	}
	targetStudentsByGroup, err := d.deps.Templates.ListTargetStudentIDs(ctx, templateIDs)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].EnrollmentCount = unionCount(rows[i].StudentIDs, targetStudentsByGroup[rows[i].TemplateID])
	}
	return targetsByGroup, nil
}

// enrichGroupNames fills a display name on every row from the School
// Structure owner with one batched query. A group the owner cannot see in
// the caller's transaction yields the fallback, mirroring the former
// LEFT JOIN + COALESCE; a query failure is returned, never swallowed.
func enrichGroupNames[T any](
	ctx context.Context,
	groupNames func(context.Context, []int64) (map[int64]string, error),
	rows []T,
	groupID func(T) int64,
	setName func(T, string),
	fallback string,
	op string,
) ([]T, error) {
	if len(rows) == 0 {
		return rows, nil
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, groupID(row))
	}
	names, err := groupNames(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("load groups for %s: %w", op, err)
	}
	for index := range rows {
		name, found := names[groupID(rows[index])]
		if !found {
			name = fallback
		}
		setName(rows[index], name)
	}
	return rows, nil
}

func optionalGroupID(id *int64) int64 {
	if id == nil {
		return 0
	}
	return *id
}

func dedupeTemplateIDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func hasTemplateTargets(targetsByGroup map[int64][]timetable.GroupTarget) bool {
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

func setDisplayRosterCapacity(rows []timetable.TemplateListRow) {
	for i := range rows {
		rows[i].CapacityEnrollmentCount = rows[i].EnrollmentCount
		rows[i].CapacitySupervisorCount = rows[i].SupervisorCount
	}
}

func distinctTemplateIDs(rows []timetable.TemplateListRow) []int64 {
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
func applyWorstTemplateCapacity(rows []timetable.TemplateListRow, templateIDs []int64, occurrences []timetable.TemplateCapacityOccurrence, childrenPerStaffRatio int) {
	included := make(map[int64]struct{}, len(templateIDs))
	for _, templateID := range templateIDs {
		included[templateID] = struct{}{}
	}
	byTemplate := make(map[int64][]timetable.TemplateCapacityOccurrence)
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
	worst := make(map[int64]timetable.TemplateCapacityOccurrence, len(byTemplate))
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
func requiredStaffOverrideOf(row timetable.TemplateListRow) *int {
	if row.RequiredStaff == nil {
		return nil
	}
	v := int(*row.RequiredStaff)
	return &v
}

// worstTemplateOccurrence skips zero-enrollment occurrences only when the
// requirement is derived: with no children the derived demand is 0. Under a
// manual override the demand is constant, so an empty, unstaffed occurrence
// can legitimately be the worst (e.g. 0/3).
func worstTemplateOccurrence(occurrences []timetable.TemplateCapacityOccurrence, override *int, childrenPerStaffRatio int) (timetable.TemplateCapacityOccurrence, bool) {
	hasPositiveDemand := false
	for _, occurrence := range occurrences {
		if timetable.EffectiveRequiredStaff(override, occurrence.EnrollmentCount, childrenPerStaffRatio) > 0 {
			hasPositiveDemand = true
			break
		}
	}
	var worst timetable.TemplateCapacityOccurrence
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

func scoreTemplateOccurrence(occurrence timetable.TemplateCapacityOccurrence, override *int, childrenPerStaffRatio int) templateCapacityScore {
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

func templateOccurrenceIsWorse(candidate, current timetable.TemplateCapacityOccurrence, override *int, childrenPerStaffRatio int) bool {
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
	return calendar.Date(candidate.OccurrenceDate).Before(calendar.Date(current.OccurrenceDate))
}

// templateListEntryOf adds the dynamic targets and the decoded
// offering-source rule to a template list row.
func templateListEntryOf(row timetable.TemplateListRow, targets []timetable.GroupTarget) timetable.TemplateListEntry {
	entry := timetable.TemplateListEntry{TemplateListRow: row, Targets: targets}
	// Corrupt jsonb cannot happen via the write path (validated slices);
	// degrade to "no filter" instead of failing the whole list read.
	if ids, err := decodeTemplateSource[int64](row.SourceCareOfferingIDsJSON); err == nil {
		entry.SourceCareOfferingIDs = ids
	}
	if levels, err := decodeTemplateSource[int](row.SourceGradeLevelsJSON); err == nil {
		entry.SourceGradeLevels = levels
	}
	if classes, err := decodeTemplateSource[string](row.SourceSchoolClassesJSON); err == nil {
		entry.SourceSchoolClasses = classes
	}
	return entry
}

// decodeTemplateSource decodes the text form of an offering-source jsonb
// column; an empty column or an empty array is no filter.
func decodeTemplateSource[T any](text string) ([]T, error) {
	if text == "" {
		return nil, nil
	}
	var values []T
	if err := json.Unmarshal([]byte(text), &values); err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, nil
	}
	return values, nil
}
