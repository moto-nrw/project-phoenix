package application

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

func validateDepartureException(v careplan.DepartureException) bool {
	return v.StudentID > 0 && !v.ExceptionDate.IsZero() && careplan.ValidDepartureMode(v.DepartureMode) &&
		validText(v.Reason, 255) && v.CreatedBy != nil && *v.CreatedBy > 0
}

func normalizeStudentScheduleFilter(filter careplan.StudentScheduleFilter) careplan.StudentScheduleFilter {
	filter.IDs = uniquePositive(filter.IDs)
	filter.StudentIDs = uniquePositive(filter.StudentIDs)
	return filter
}

func validWeekday(day int) bool { return day >= 1 && day <= 5 }

func validAuthor(source string, staff int64, guardian *int64) bool {
	return (source == "" || source == careplan.ExceptionSourceStaff) && staff > 0 || source == careplan.ExceptionSourceGuardian && guardian != nil && *guardian > 0
}

func validText(value *string, max int) bool {
	return value == nil || utf8.RuneCountInString(*value) <= max
}

func validateArrivalSchedule(v careplan.ArrivalSchedule) bool {
	return v.StudentID > 0 && validWeekday(v.Weekday) && v.CreatedBy > 0 && validText(v.Notes, 500)
}

func validateArrivalException(v careplan.ArrivalException) bool {
	return v.StudentID > 0 && !v.ExceptionDate.IsZero() && validText(v.Reason, 255) && validAuthor(v.Source, v.CreatedBy, v.CreatedByGuardian)
}

func validateNote(studentID int64, date careplan.Date, content string, createdBy int64) bool {
	return studentID > 0 && !date.IsZero() && strings.TrimSpace(content) != "" && len(content) <= 500 && createdBy > 0
}

// validatePickupNote accepts a dated note or a recurring weekday note (#3369),
// never a row that is both or neither.
func validatePickupNote(v careplan.PickupNote) bool {
	if v.Weekday == 0 {
		return validateNote(v.StudentID, v.NoteDate, v.Content, v.CreatedBy)
	}
	return v.NoteDate.IsZero() && validWeekday(v.Weekday) &&
		v.StudentID > 0 && strings.TrimSpace(v.Content) != "" && len(v.Content) <= 500 && v.CreatedBy > 0
}

func validatePickupSchedule(v careplan.PickupSchedule) bool {
	if v.StudentID <= 0 || !validWeekday(v.Weekday) || v.PickupTime.IsZero() || !validText(v.Notes, 500) {
		return false
	}
	if v.Source == "" || v.Source == careplan.ScheduleSourceStaff {
		return v.CreatedBy > 0
	}
	return v.Source == careplan.ScheduleSourceCareOffering && v.CareOfferingID != nil && *v.CareOfferingID > 0
}

func validatePickupException(v careplan.PickupException) bool {
	if v.StudentID <= 0 || v.ExceptionDate.IsZero() || !validText(v.Reason, 255) || !validText(v.ExcusedReason, 255) || !validAuthor(v.Source, v.CreatedBy, v.CreatedByGuardian) {
		return false
	}
	if v.ExcusedFrom == nil {
		return v.ExcusedReason == nil && v.ExcusedCreatedBy == nil && !v.ExcusedOwnsPickupTime && !v.ExcusedAuto
	}
	if v.ExcusedAuto {
		return v.ExcusedCreatedBy == nil && !v.ExcusedOwnsPickupTime && v.PickupTime != nil
	}
	return v.ExcusedCreatedBy != nil && *v.ExcusedCreatedBy > 0 && (!v.ExcusedOwnsPickupTime || v.PickupTime != nil)
}

func scheduleInvalid() error {
	return invalid(careplan.ErrInvalidStudentSchedule, "student schedule record is invalid")
}

func validWeekdayPickupNotes(notes map[int]string) bool {
	for weekday, content := range notes {
		if !validWeekday(weekday) || strings.TrimSpace(content) == "" || len(content) > 500 {
			return false
		}
	}
	return true
}

func uniqueStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func validCareExitRemoval(value careplan.CareExitRemoval) bool {
	if value.StudentID <= 0 {
		return false
	}
	switch value.Kind {
	case careplan.CareExitRemovalRoster:
		return value.InstanceID != nil && value.Status != nil
	case careplan.CareExitRemovalBooking:
		return value.EnrollmentID != nil && (!value.WasDeleted || value.ActivityGroupID != nil && value.ValidFrom != nil)
	default:
		return false
	}
}

func validCareExitSource(kind string) bool {
	switch kind {
	case careplan.CareExitSourceBooking, careplan.CareExitSourcePickupSchedule, careplan.CareExitSourcePickupException,
		careplan.CareExitSourceArrivalSchedule, careplan.CareExitSourceArrivalException, careplan.CareExitSourceDepartureException:
		return true
	default:
		return false
	}
}

func normalizeCareExit(value *careplan.CareExit) error {
	if value.StudentID <= 0 {
		return invalid(careplan.ErrInvalidCareExit, "student ID is required")
	}
	note, err := careplan.NormalizeCareExitReason(value.Reason, value.ReasonNote)
	if err != nil {
		return err
	}
	value.ReasonNote = note
	return nil
}

var canonicalWeekDays = map[string]struct{}{"mon": {}, "tue": {}, "wed": {}, "thu": {}, "fri": {}, "sat": {}, "sun": {}}

// normalizeCareOfferingFilter deduplicates the ID filters and closes the
// order over the catalog orders; an empty order is the catalog order.
func normalizeCareOfferingFilter(filter careplan.CareOfferingFilter) (careplan.CareOfferingFilter, error) {
	filter.IDs = uniquePositive(filter.IDs)
	filter.PhaseIDs = uniquePositive(filter.PhaseIDs)
	filter.ActivityGroupIDs = uniquePositive(filter.ActivityGroupIDs)
	if filter.Order == "" {
		filter.Order = careplan.OfferingOrderCatalog
	}
	switch filter.Order {
	case careplan.OfferingOrderCatalog, careplan.OfferingOrderID, careplan.OfferingOrderPhaseCatalog:
		return filter, nil
	default:
		return filter, invalid(careplan.ErrInvalidCareOffering, "care offering order is invalid")
	}
}

func normalizeCareOfferingFields(fields careplan.CareOfferingFields) (careplan.CareOfferingFields, error) {
	if err := validateCareOfferingScalars(&fields); err != nil {
		return fields, err
	}
	var err error
	if fields.AvailableDays, err = normalizeDays(fields.AvailableDays); err != nil {
		return fields, err
	}
	if fields.AutoAddGradeLevels, err = normalizeGrades(fields.AutoAddGradeLevels); err != nil {
		return fields, err
	}
	if fields.AvailabilityRule, err = normalizeAvailabilityRule(fields.AvailabilityRule); err != nil {
		return fields, err
	}
	if fields.PickupTimes, err = normalizeOfferingPickupTimes(fields.PickupTimes, fields.AvailableDays); err != nil {
		return fields, err
	}
	return fields, nil
}

func validateCareOfferingScalars(fields *careplan.CareOfferingFields) error {
	if fields.PhaseID <= 0 {
		return invalid(careplan.ErrInvalidCareOffering, "phase ID is required")
	}
	fields.Name = strings.TrimSpace(fields.Name)
	if fields.Name == "" {
		return invalid(careplan.ErrInvalidCareOffering, "care offering name is required")
	}
	if fields.DaysOfWeekMode == "" {
		fields.DaysOfWeekMode = "fixed"
	}
	if fields.DaysOfWeekMode != "fixed" && fields.DaysOfWeekMode != "parent_choice" {
		return invalid(careplan.ErrInvalidCareOffering, "days_of_week_mode must be fixed or parent_choice")
	}
	if (fields.Capacity != nil && *fields.Capacity < 0) || (fields.PriceCents != nil && *fields.PriceCents < 0) {
		return invalid(careplan.ErrInvalidCareOffering, "capacity and price_cents must be non-negative")
	}
	if fields.IsRequired && fields.Capacity != nil {
		return invalid(careplan.ErrInvalidCareOffering, "a required care offering must not have a capacity limit")
	}
	fields.SelectionGroup = strings.TrimSpace(fields.SelectionGroup)
	if fields.SelectionRule == "" {
		fields.SelectionRule = "optional"
	}
	if !slices.Contains([]string{"optional", "exactly_one", "at_least_one", "at_most_one", "at_most_one_per_day"}, fields.SelectionRule) {
		return invalid(careplan.ErrInvalidCareOffering, "selection_rule is invalid")
	}
	if fields.SelectionRule != "optional" && fields.SelectionGroup == "" {
		return invalid(careplan.ErrInvalidCareOffering, "a selection rule requires a selection_group name")
	}
	return nil
}

func normalizeDays(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, invalid(careplan.ErrInvalidCareOffering, "available_days must contain at least one day")
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		day := strings.ToLower(strings.TrimSpace(value))
		if _, ok := canonicalWeekDays[day]; !ok {
			return nil, invalid(careplan.ErrInvalidCareOffering, fmt.Sprintf("available_days entry %q is invalid", value))
		}
		result = append(result, day)
	}
	return result, nil
}

func normalizeGrades(values []int) ([]int, error) {
	result := make([]int, 0, len(values))
	seen := make(map[int]struct{}, len(values))
	for _, value := range values {
		if value < 1 || value > 13 {
			return nil, invalid(careplan.ErrInvalidCareOffering, fmt.Sprintf("invalid grade %d", value))
		}
		if _, duplicate := seen[value]; duplicate {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result, nil
}

func normalizeAvailabilityRule(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var rule availabilityRule
	if err := json.Unmarshal(raw, &rule); err != nil {
		return nil, invalid(careplan.ErrInvalidCareOffering, "availability_rule must be valid JSON")
	}
	if len(rule.Conditions) == 0 {
		return nil, nil
	}
	if rule.Match != "all" && rule.Match != "any" {
		return nil, invalid(careplan.ErrInvalidCareOffering, "availability_rule.match must be all or any")
	}
	for i := range rule.Conditions {
		condition := &rule.Conditions[i]
		if condition.Source != "grade_level" || condition.Operator != "in" && condition.Operator != "not_in" || len(condition.Value) == 0 {
			return nil, invalid(careplan.ErrInvalidCareOffering, fmt.Sprintf("availability_rule condition %d is invalid", i+1))
		}
		grades, err := normalizeGrades(condition.Value)
		if err != nil {
			return nil, err
		}
		slices.Sort(grades)
		condition.Value = grades
	}
	encoded, err := json.Marshal(rule)
	if err != nil {
		return nil, invalid(careplan.ErrInvalidCareOffering, "availability_rule cannot be encoded")
	}
	return encoded, nil
}

func normalizeOfferingPickupTimes(values map[string]string, days []string) (map[string]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	available := make(map[string]struct{}, len(days))
	for _, day := range days {
		available[day] = struct{}{}
	}
	result := make(map[string]string, len(values))
	for rawDay, rawValue := range values {
		day, value := strings.ToLower(strings.TrimSpace(rawDay)), strings.TrimSpace(rawValue)
		if value == "" {
			continue
		}
		if _, ok := canonicalWeekDays[day]; !ok || day == "sat" || day == "sun" {
			return nil, invalid(careplan.ErrInvalidCareOffering, fmt.Sprintf("pickup_times day %q is invalid", rawDay))
		}
		if _, ok := available[day]; !ok {
			return nil, invalid(careplan.ErrInvalidCareOffering, fmt.Sprintf("pickup_times day %q is not available", day))
		}
		parsed, err := time.Parse("15:04", value)
		if err != nil {
			return nil, invalid(careplan.ErrInvalidCareOffering, fmt.Sprintf("pickup_times value for %q must be HH:MM", day))
		}
		result[day] = parsed.Format("15:04")
	}
	if len(result) == 0 {
		return nil, nil
	}
	return result, nil
}

func validateDate(value, label string, kind error) error {
	if value == "" {
		return invalid(kind, label+" is required")
	}
	if _, err := time.Parse(careplan.DateLayout, value); err != nil {
		return invalid(kind, label+" must be a calendar date in YYYY-MM-DD format")
	}
	return nil
}

func validRequiredJSON(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return strings.HasPrefix(trimmed, "{") && json.Valid(raw)
}

func terminalOfferingChangeStatus(status string) bool {
	return status != careplan.OfferingChangePending && validOfferingChangeStatus(status)
}

func validOfferingChangeStatus(status string) bool {
	switch status {
	case careplan.OfferingChangePending, careplan.OfferingChangeApproved, careplan.OfferingChangeRejected,
		careplan.OfferingChangeWithdrawn, careplan.OfferingChangeDone, careplan.OfferingChangeCareEnded:
		return true
	default:
		return false
	}
}

func normalizedTriggers(targetID int64, ids []int64) []int64 {
	ids = uniquePositive(ids)
	result := ids[:0]
	for _, id := range ids {
		if id != targetID {
			result = append(result, id)
		}
	}
	return result
}

func uniquePositive(ids []int64) []int64 {
	if ids == nil {
		return nil
	}
	result := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func invalid(kind error, reason string) error {
	return &careplan.InvalidError{Kind: kind, Reason: reason}
}
