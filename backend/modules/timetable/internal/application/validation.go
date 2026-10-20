package application

import (
	"regexp"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

const (
	categoryNameMaxLength        = 60
	categoryDescriptionMaxLength = 255
)

var categoryColorPattern = regexp.MustCompile(`^#([A-Fa-f0-9]{6}|[A-Fa-f0-9]{3})$`)

var pickupClockPattern = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

func invalidActivityInstanceWeekdays(weekdays []int) bool {
	for _, weekday := range weekdays {
		if !validWeekday(weekday) {
			return true
		}
	}
	return false
}

func validInstanceStaff(input timetable.InstanceStaffInput) bool {
	return input.InstanceID > 0 && input.StaffID > 0 &&
		(input.RoomID == nil || *input.RoomID > 0) && (input.SickAbsenceID == nil || *input.SickAbsenceID > 0)
}

func validInstanceStaffFilter(filter timetable.InstanceStaffFilter) bool {
	orders := 0
	for _, ordered := range []bool{filter.OrderByCreated, filter.OrderByInstanceAndCreated, filter.OrderByActivityTime, filter.OrderByActivityDateTime} {
		if ordered {
			orders++
		}
	}
	return filter.Limit >= 0 && filter.Offset >= 0 && orders <= 1 &&
		!hasInvalidID(filter.IDs) && !hasInvalidID(filter.InstanceIDs) && !hasInvalidID(filter.StaffIDs) &&
		(filter.SickAbsenceID == nil || *filter.SickAbsenceID > 0) &&
		validOptionalDate(filter.Date) && validOptionalDate(filter.FromDate) && validOptionalDate(filter.ToDate)
}

func validInstanceStaffColumns(columns []string) bool {
	if len(columns) == 0 || hasDuplicateString(columns) {
		return false
	}
	for _, column := range columns {
		if column != "is_primary" && column != "sick_absence_id" {
			return false
		}
	}
	return true
}

func normalizeCategory(name, description, color *string) error {
	*name = strings.TrimSpace(*name)
	*description = strings.TrimSpace(*description)
	*color = strings.TrimSpace(*color)
	if *name == "" || len([]rune(*name)) > categoryNameMaxLength || len([]rune(*description)) > categoryDescriptionMaxLength {
		return timetable.ErrInvalidCategory
	}
	if *color != "" && !strings.HasPrefix(*color, "#") {
		*color = "#" + *color
	}
	if *color != "" && !categoryColorPattern.MatchString(*color) {
		return timetable.ErrInvalidCategory
	}
	return nil
}

func reservedCategoryName(name string) bool {
	return strings.EqualFold(name, timetable.WCCategoryName) || strings.EqualFold(name, timetable.SchulhofCategoryName) ||
		strings.EqualFold(name, timetable.OpenRoomCategoryName)
}

func hasInvalidID(ids []int64) bool {
	for _, id := range ids {
		if id <= 0 {
			return true
		}
	}
	return false
}

func validWeekday(value int) bool {
	return value >= timetable.WeekdayMonday && value <= timetable.WeekdaySunday
}

func normalizeSchedule(input *timetable.ScheduleInput) bool {
	if input.ActivityGroupID <= 0 || !validWeekday(input.Weekday) {
		return false
	}
	return (input.ValidFrom == nil || timetable.ValidDate(*input.ValidFrom)) &&
		(input.ValidUntil == nil || timetable.ValidDate(*input.ValidUntil))
}

func normalizePlannedSupervisor(input *timetable.PlannedSupervisorInput) bool {
	if input.StaffID <= 0 || input.GroupID <= 0 || (input.Weekday != nil && !validWeekday(*input.Weekday)) {
		return false
	}
	return (input.ValidFrom == "" || timetable.ValidDate(input.ValidFrom)) &&
		(input.ValidUntil == nil || timetable.ValidDate(*input.ValidUntil))
}

func validStudentEnrollment(input timetable.StudentEnrollmentInput) bool {
	if input.StudentID <= 0 || input.ActivityGroupID <= 0 || !timetable.ValidDate(input.ValidFrom) ||
		(input.ValidUntil != nil && !timetable.ValidDate(*input.ValidUntil)) ||
		(input.CalendarPeriodID != nil && *input.CalendarPeriodID <= 0) ||
		(input.EnrollmentRequestChildID != nil && *input.EnrollmentRequestChildID <= 0) ||
		(input.Weekday != nil && !validWeekday(*input.Weekday)) {
		return false
	}
	if input.AttendanceStatus != nil && !validAttendanceStatus(*input.AttendanceStatus) {
		return false
	}
	return validSelectedWeekdays(input.SelectedWeekdays)
}

// validSelectedWeekdays reports whether every selected weekday is valid and
// appears once.
func validSelectedWeekdays(weekdays []int) bool {
	seen := make(map[int]struct{}, len(weekdays))
	for _, weekday := range weekdays {
		if !validWeekday(weekday) {
			return false
		}
		if _, exists := seen[weekday]; exists {
			return false
		}
		seen[weekday] = struct{}{}
	}
	return true
}

func validAttendanceStatus(value string) bool {
	return value == timetable.AttendancePresent || value == timetable.AttendanceAbsent || value == timetable.AttendanceExcused || value == timetable.AttendanceUnknown
}

func validTimeframe(input timetable.TimeframeInput) bool {
	start, err := time.Parse("15:04:05", input.StartTime)
	if err != nil {
		return false
	}
	if input.EndTime == nil {
		return true
	}
	end, err := time.Parse("15:04:05", *input.EndTime)
	return err == nil && end.After(start)
}

func validClock(value string) bool {
	_, err := time.Parse("15:04:05", value)
	return err == nil
}

func validActivityExceptionFilter(filter timetable.ActivityExceptionFilter) bool {
	return filter.Limit >= 0 && filter.Offset >= 0 &&
		(filter.ActivityGroupID == nil || *filter.ActivityGroupID > 0) &&
		validOptionalDate(filter.ExceptionDate) && validOptionalDate(filter.FromDate) &&
		validOptionalDate(filter.ToDate) && validOptionalDate(filter.BeforeDate)
}

func validOptionalDate(value *string) bool {
	return value == nil || timetable.ValidDate(*value)
}

func validActivityException(input timetable.ActivityExceptionInput) bool {
	if input.ActivityGroupID <= 0 || !timetable.ValidDate(input.ExceptionDate) ||
		(input.Reason != nil && len(*input.Reason) > timetable.ActivityExceptionReasonMaxLength) {
		return false
	}
	switch input.ExceptionType {
	case timetable.ActivityExceptionCancelled:
		return input.StartTime == nil && input.EndTime == nil && input.RoomID == nil
	case timetable.ActivityExceptionModified:
		return validActivityExceptionOverride(input)
	default:
		return false
	}
}

func validActivityExceptionOverride(input timetable.ActivityExceptionInput) bool {
	if input.StartTime == nil && input.EndTime == nil && input.RoomID == nil {
		return false
	}
	if input.StartTime != nil && !validClock(*input.StartTime) {
		return false
	}
	if input.EndTime != nil && !validClock(*input.EndTime) {
		return false
	}
	if input.StartTime == nil || input.EndTime == nil {
		return true
	}
	start, _ := time.Parse("15:04:05", *input.StartTime)
	end, _ := time.Parse("15:04:05", *input.EndTime)
	return end.After(start)
}

func validActivityInstanceFilter(filter timetable.ActivityInstanceFilter) bool {
	if filter.Limit < 0 || filter.Offset < 0 || hasInvalidID(filter.IDs) ||
		hasInvalidID(filter.ActivityGroupIDs) ||
		(filter.ActivityGroupID != nil && *filter.ActivityGroupID <= 0) ||
		!validOptionalDate(filter.Date) || !validOptionalDate(filter.FromDate) || !validOptionalDate(filter.ToDate) {
		return false
	}
	for _, date := range filter.Dates {
		if !timetable.ValidDate(date) {
			return false
		}
	}
	return filter.Status == "" || validActivityInstanceStatus(filter.Status)
}

func normalizeActivityInstance(input timetable.ActivityInstanceInput) (timetable.ActivityInstanceInput, bool) {
	if input.ListKind != nil && *input.ListKind == "" {
		input.ListKind = nil
	}
	if input.Title == "" || len(input.Title) > timetable.ActivityInstanceTitleMaxLength ||
		!timetable.ValidDate(input.Date) || !validClock(input.StartTime) || !validClock(input.EndTime) ||
		input.RoomID < 0 || !validActivityInstanceStatus(input.Status) ||
		(input.RequiredStaff != nil && *input.RequiredStaff < 0) || !normalizeListKind(&input.ListKind) {
		return input, false
	}
	start, _ := time.Parse("15:04:05", input.StartTime)
	end, _ := time.Parse("15:04:05", input.EndTime)
	if !end.After(start) {
		return input, false
	}
	if input.IdempotencyKey != nil {
		if strings.TrimSpace(*input.IdempotencyKey) == "" || len(*input.IdempotencyKey) > timetable.ActivityInstanceIdempotencyKeyMaxLength {
			return input, false
		}
	}
	return input, true
}

func hasDuplicateString(values []string) bool {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			return true
		}
		seen[value] = struct{}{}
	}
	return false
}

func normalizePlanningTrack(input timetable.PlanningTrackInput) (timetable.PlanningTrackInput, bool) {
	input.Name = strings.TrimSpace(input.Name)
	valid := input.Name != "" && len(input.Name) <= 100 && timetable.ValidPlanningTrackColor(input.Color) && input.SortOrder >= 0
	return input, valid
}

func normalizeRecurrenceRule(input timetable.RecurrenceRuleInput) (timetable.RecurrenceRuleInput, bool) {
	input.Frequency = strings.ToLower(input.Frequency)
	if !validRecurrenceFrequency(input.Frequency) || input.IntervalCount < 1 {
		return input, false
	}
	for index, weekday := range input.Weekdays {
		input.Weekdays[index] = strings.ToUpper(weekday)
		if !validRecurrenceWeekday(input.Weekdays[index]) {
			return input, false
		}
	}
	for _, day := range input.MonthDays {
		if day < 1 || day > 31 {
			return input, false
		}
	}
	return input, (input.Count == nil || *input.Count > 0) && (input.EndDate == nil || input.Count == nil)
}

func validRecurrenceFrequency(value string) bool {
	return value == "daily" || value == "weekly" || value == "monthly" || value == "yearly"
}

func validRecurrenceWeekday(value string) bool {
	switch value {
	case "MON", "TUE", "WED", "THU", "FRI", "SAT", "SUN":
		return true
	default:
		return false
	}
}

func hasDuplicateID(ids []int64) bool {
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if _, exists := seen[id]; exists {
			return true
		}
		seen[id] = struct{}{}
	}
	return false
}

func invalidCareExitRemovals(removals []timetable.CareExitEnrollmentRemoval) bool {
	for _, removal := range removals {
		if removal.ID <= 0 || removal.TenantID <= 0 || removal.StudentID <= 0 {
			return true
		}
		if removal.WasDeleted && (removal.ActivityGroupID <= 0 || !timetable.ValidDate(removal.ValidFrom)) {
			return true
		}
		if (removal.ValidUntil != nil && !timetable.ValidDate(*removal.ValidUntil)) ||
			(removal.PreviousValidUntil != nil && !timetable.ValidDate(*removal.PreviousValidUntil)) {
			return true
		}
	}
	return false
}

func normalizeGroup(input *timetable.GroupInput) error {
	if input.Name == "" || input.MaxParticipants < 0 || input.CategoryID <= 0 ||
		(input.RequiredStaff != nil && *input.RequiredStaff < 0) {
		return timetable.ErrInvalidGroup
	}
	if input.Type == "" {
		input.Type = timetable.GroupTypeActivity
	}
	if !timetable.IsValidGroupType(input.Type) || !normalizeListKind(&input.ListKind) {
		return timetable.ErrInvalidGroup
	}
	if input.TargetGroupType == "" {
		input.TargetGroupType = timetable.TargetGroupTypeNone
	}
	if !validGroupTarget(input) || !normalizeOfferingSource(input) || !validDutyGroup(input) {
		return timetable.ErrInvalidGroup
	}
	return nil
}

func normalizeTemplateUpdate(input *timetable.TemplateUpdate) bool {
	group := timetable.GroupInput{
		Name: input.Name, Type: input.Type, CategoryID: input.CategoryID,
		EducationGroupID: input.EducationGroupID, MaxParticipants: input.MaxParticipants,
		RequiredStaff: input.RequiredStaff, ListKind: input.ListKind, IsTemplate: true,
		CalendarPeriodID: input.CalendarPeriodID, TargetGroupType: input.TargetGroupType,
		TargetGradeLevel: input.TargetGradeLevel, TargetSchoolClass: input.TargetSchoolClass,
		SourceCareOfferingIDs: input.SourceCareOfferingIDs, SourceGradeLevels: input.SourceGradeLevels,
		SourceSchoolClasses: input.SourceSchoolClasses, Notes: input.Notes,
	}
	if normalizeGroup(&group) != nil {
		return false
	}
	input.Type, input.ListKind, input.TargetGroupType = group.Type, group.ListKind, group.TargetGroupType
	input.TargetSchoolClass = group.TargetSchoolClass
	input.SourceCareOfferingIDs, input.SourceGradeLevels = group.SourceCareOfferingIDs, group.SourceGradeLevels
	input.SourceSchoolClasses = group.SourceSchoolClasses
	return true
}

func normalizeListKind(value **string) bool {
	if *value == nil {
		return true
	}
	if **value == "" {
		*value = nil
		return true
	}
	switch **value {
	case timetable.ListKindEdgeHours, timetable.ListKindLearningTime, timetable.ListKindActivity, timetable.ListKindMensa:
		return true
	default:
		return false
	}
}

func validGroupTarget(input *timetable.GroupInput) bool {
	switch input.TargetGroupType {
	case timetable.TargetGroupTypeGrade:
		return input.TargetGradeLevel != nil && *input.TargetGradeLevel >= 1 && *input.TargetGradeLevel <= 13 && input.TargetSchoolClass == nil
	case timetable.TargetGroupTypeSchoolClass:
		if input.TargetSchoolClass == nil || strings.TrimSpace(*input.TargetSchoolClass) == "" || input.TargetGradeLevel != nil {
			return false
		}
		trimmed := strings.TrimSpace(*input.TargetSchoolClass)
		input.TargetSchoolClass = &trimmed
		return true
	case timetable.TargetGroupTypeEducationGroup:
		return input.EducationGroupID != nil && input.TargetGradeLevel == nil && input.TargetSchoolClass == nil
	case timetable.TargetGroupTypeOffering, timetable.TargetGroupTypeNone:
		return input.TargetGradeLevel == nil && input.TargetSchoolClass == nil
	default:
		return false
	}
}

func normalizeOfferingSource(input *timetable.GroupInput) bool {
	if len(input.SourceCareOfferingIDs) == 0 {
		input.SourceCareOfferingIDs = nil
		if len(input.SourceGradeLevels) > 0 || len(input.SourceSchoolClasses) > 0 {
			return false
		}
		input.SourceGradeLevels, input.SourceSchoolClasses = nil, nil
		return true
	}
	if input.TargetGroupType != timetable.TargetGroupTypeOffering || hasInvalidOrDuplicateIDs(input.SourceCareOfferingIDs) ||
		hasInvalidOrDuplicateGrades(input.SourceGradeLevels) {
		return false
	}
	classes, ok := normalizedSourceClasses(input.SourceSchoolClasses)
	if !ok || (len(input.SourceGradeLevels) > 0 && len(classes) > 0) {
		return false
	}
	input.SourceSchoolClasses = classes
	if len(input.SourceGradeLevels) == 0 {
		input.SourceGradeLevels = nil
	}
	return true
}

func hasInvalidOrDuplicateIDs(ids []int64) bool {
	seen := make(map[int64]bool, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			return true
		}
		seen[id] = true
	}
	return false
}

func hasInvalidOrDuplicateGrades(grades []int) bool {
	seen := make(map[int]bool, len(grades))
	for _, grade := range grades {
		if grade < 1 || grade > 13 || seen[grade] {
			return true
		}
		seen[grade] = true
	}
	return false
}

func normalizedSourceClasses(classes []string) ([]string, bool) {
	if len(classes) == 0 {
		return nil, true
	}
	result := make([]string, 0, len(classes))
	seen := make(map[string]bool, len(classes))
	for _, class := range classes {
		trimmed := strings.TrimSpace(class)
		key := strings.ToLower(trimmed)
		if trimmed == "" || seen[key] {
			return nil, false
		}
		seen[key] = true
		result = append(result, trimmed)
	}
	return result, true
}

func normalizeGroupTargets(groupID int64, targets []timetable.GroupTargetInput) ([]timetable.GroupTargetInput, error) {
	if groupID <= 0 {
		return nil, timetable.ErrInvalidGroupTarget
	}
	result := make([]timetable.GroupTargetInput, len(targets))
	var targetType string
	for index, target := range targets {
		normalized, err := normalizeGroupTarget(target)
		if err != nil || (targetType != "" && normalized.TargetGroupType != targetType) {
			return nil, timetable.ErrInvalidGroupTarget
		}
		targetType = normalized.TargetGroupType
		result[index] = normalized
	}
	return result, nil
}

func normalizeGroupTarget(target timetable.GroupTargetInput) (timetable.GroupTargetInput, error) {
	switch target.TargetGroupType {
	case timetable.TargetGroupTypeGrade:
		if target.TargetGradeLevel == nil || *target.TargetGradeLevel < 1 || *target.TargetGradeLevel > 13 || target.TargetSchoolClass != nil || target.EducationGroupID != nil {
			return timetable.GroupTargetInput{}, timetable.ErrInvalidGroupTarget
		}
	case timetable.TargetGroupTypeSchoolClass:
		if target.TargetSchoolClass == nil || strings.TrimSpace(*target.TargetSchoolClass) == "" || target.TargetGradeLevel != nil || target.EducationGroupID != nil {
			return timetable.GroupTargetInput{}, timetable.ErrInvalidGroupTarget
		}
		trimmed := strings.TrimSpace(*target.TargetSchoolClass)
		target.TargetSchoolClass = &trimmed
	case timetable.TargetGroupTypeEducationGroup:
		if target.EducationGroupID == nil || *target.EducationGroupID <= 0 || target.TargetGradeLevel != nil || target.TargetSchoolClass != nil {
			return timetable.GroupTargetInput{}, timetable.ErrInvalidGroupTarget
		}
	default:
		return timetable.GroupTargetInput{}, timetable.ErrInvalidGroupTarget
	}
	return target, nil
}

// validActivityInstanceStatus accepts the planning states only: whether a
// block runs or ended is Student Presence's session.
func validActivityInstanceStatus(value string) bool {
	return value == timetable.InstanceStatusPlanned || value == timetable.InstanceStatusCancelled
}

// validActivityInstanceColumns names the planning columns a patch may write.
func validActivityInstanceColumns(columns []string) bool {
	for _, column := range columns {
		switch column {
		case "date", "activity_group_id", "calendar_period_id", "title", "description",
			"start_time", "end_time", "room_id", "required_staff", "status",
			"list_kind", "is_spontaneous", "understaffed_ack", "understaffed_note", "cancel_reason",
			"notes", "created_by":
		default:
			return false
		}
	}
	return true
}

func validInstanceStudent(input timetable.InstanceStudentInput) bool {
	return input.InstanceID > 0 && input.StudentID > 0 && (input.RoomID == nil || *input.RoomID > 0)
}

func validInstanceStudentFilter(filter timetable.InstanceStudentFilter) bool {
	orders := 0
	for _, ordered := range []bool{filter.OrderByCreated, filter.OrderByInstanceStudent,
		filter.OrderByStudentActivityTime, filter.OrderByActivityDateTime} {
		if ordered {
			orders++
		}
	}
	return filter.Limit >= 0 && filter.Offset >= 0 && orders <= 1 &&
		!hasInvalidID(filter.IDs) && !hasInvalidID(filter.InstanceIDs) && !hasInvalidID(filter.StudentIDs) &&
		validOptionalDate(filter.Date) && validOptionalDate(filter.FromDate) && validOptionalDate(filter.ToDate) &&
		(filter.CurrentTime == nil || (filter.Date != nil && validClock(*filter.CurrentTime))) &&
		(filter.FromClock == nil || validClock(*filter.FromClock))
}

func validRosterArchiveEntries(entries []timetable.RosterArchiveEntry) bool {
	for _, entry := range entries {
		if entry.ParticipantID <= 0 || entry.Attendance.Status == "" {
			return false
		}
	}
	return true
}

func validPickupClock(value string) bool { return pickupClockPattern.MatchString(value) }

func validRecurrenceRuleSort(value string) bool {
	return value == "" || value == "id" || value == "frequency" || value == "interval_count" ||
		value == "end_date" || value == "count" || value == "created_at" || value == "updated_at"
}

// validDutyGroup applies the duty rule to a group write; other types pass.
func validDutyGroup(input *timetable.GroupInput) bool {
	return input.Type != timetable.GroupTypeDuty || timetable.ValidateTemplateShape(timetable.TemplateShape{
		Type: input.Type, TargetGroupType: input.TargetGroupType, EducationGroupID: input.EducationGroupID,
		HasOfferingSource: len(input.SourceCareOfferingIDs) > 0, MaxParticipants: input.MaxParticipants,
		ListKind: input.ListKind,
	}) == nil
}
