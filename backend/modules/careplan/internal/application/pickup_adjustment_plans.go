package application

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The ISO weekdays a weekly pickup plan covers.
const (
	pickupWeekdayMonday = 1
	pickupWeekdayFriday = 5
)

// pickupPreviewTokenSize is the byte length of a SHA-256 preview token.
const pickupPreviewTokenSize = 32

func normalizePickupAdjustmentInput(
	input careplan.PickupAdjustmentPreviewInput,
	today calendar.Date,
) (careplan.PickupAdjustmentPreviewInput, map[int]careplan.PickupAdjustmentSchedule, error) {
	if input.StudentID <= 0 {
		return input, nil, fmt.Errorf("%w: student is required", careplan.ErrPickupAdjustmentInvalid)
	}
	if len(input.Schedules) > 0 && len(input.CareDays) == 0 {
		return input, nil, fmt.Errorf("%w: pickup times require care days", careplan.ErrPickupAdjustmentInvalid)
	}
	if input.EffectiveFrom.IsZero() {
		input.EffectiveFrom = today
	}
	if input.EffectiveFrom.Before(today) {
		return input, nil, fmt.Errorf("%w: effective date is in the past", careplan.ErrPickupAdjustmentInvalid)
	}
	input.CareDays = append([]int(nil), input.CareDays...)
	slices.Sort(input.CareDays)
	input.CareDays = slices.Compact(input.CareDays)
	for _, weekday := range input.CareDays {
		if weekday < pickupWeekdayMonday || weekday > pickupWeekdayFriday {
			return input, nil, fmt.Errorf("%w: invalid care weekday %d", careplan.ErrPickupAdjustmentInvalid, weekday)
		}
	}
	if err := normalizePickupArrivalSchedules(&input); err != nil {
		return input, nil, err
	}
	byDay, err := normalizePickupSchedules(input.Schedules)
	if err != nil {
		return input, nil, err
	}
	sort.Slice(input.Schedules, func(i, j int) bool { return input.Schedules[i].Weekday < input.Schedules[j].Weekday })
	return input, byDay, nil
}

// normalizePickupSchedules trims and validates the proposed pickup times in
// place and indexes them by weekday.
func normalizePickupSchedules(schedules []careplan.PickupAdjustmentSchedule) (map[int]careplan.PickupAdjustmentSchedule, error) {
	byDay := make(map[int]careplan.PickupAdjustmentSchedule, len(schedules))
	for i := range schedules {
		row := &schedules[i]
		row.PickupTime = strings.TrimSpace(row.PickupTime)
		if row.Weekday < pickupWeekdayMonday || row.Weekday > pickupWeekdayFriday {
			return nil, fmt.Errorf("%w: invalid weekday %d", careplan.ErrPickupAdjustmentInvalid, row.Weekday)
		}
		if _, duplicate := byDay[row.Weekday]; duplicate {
			return nil, fmt.Errorf("%w: duplicate weekday %d", careplan.ErrPickupAdjustmentInvalid, row.Weekday)
		}
		if _, err := time.Parse("15:04", row.PickupTime); err != nil {
			return nil, fmt.Errorf("%w: invalid pickup time for weekday %d", careplan.ErrPickupAdjustmentInvalid, row.Weekday)
		}
		byDay[row.Weekday] = *row
	}
	return byDay, nil
}

func normalizePickupArrivalSchedules(input *careplan.PickupAdjustmentPreviewInput) error {
	if input.ArrivalSchedules == nil {
		return nil
	}
	rows := append([]careplan.PickupAdjustmentArrivalSchedule(nil), (*input.ArrivalSchedules)...)
	seen := make(map[int]bool, len(rows))
	weekdays := make([]int, 0, len(rows))
	for i := range rows {
		row := &rows[i]
		row.ExpectedArrival = strings.TrimSpace(row.ExpectedArrival)
		if err := validatePickupArrivalRow(*row, seen); err != nil {
			return err
		}
		seen[row.Weekday] = true
		weekdays = append(weekdays, row.Weekday)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Weekday < rows[j].Weekday })
	slices.Sort(weekdays)
	if !slices.Equal(weekdays, input.CareDays) {
		return fmt.Errorf("%w: arrival weekdays must match care days", careplan.ErrPickupAdjustmentInvalid)
	}
	input.ArrivalSchedules = &rows
	return nil
}

func validatePickupArrivalRow(row careplan.PickupAdjustmentArrivalSchedule, seen map[int]bool) error {
	if row.Weekday < pickupWeekdayMonday || row.Weekday > pickupWeekdayFriday || seen[row.Weekday] {
		return fmt.Errorf("%w: invalid or duplicate arrival weekday %d", careplan.ErrPickupAdjustmentInvalid, row.Weekday)
	}
	if row.ExpectedArrival != "" {
		if _, err := time.Parse("15:04", row.ExpectedArrival); err != nil {
			return fmt.Errorf("%w: invalid arrival time for weekday %d", careplan.ErrPickupAdjustmentInvalid, row.Weekday)
		}
	}
	if row.Notes != nil && len(*row.Notes) > 500 {
		return fmt.Errorf("%w: arrival notes are too long", careplan.ErrPickupAdjustmentInvalid)
	}
	return nil
}

func pickupScheduleRows(studentID, createdBy int64, input []careplan.PickupAdjustmentSchedule) ([]*careplan.PickupSchedule, error) {
	rows := make([]*careplan.PickupSchedule, 0, len(input))
	for _, item := range input {
		parsed, err := time.Parse("15:04", item.PickupTime)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid pickup time", careplan.ErrPickupAdjustmentInvalid)
		}
		rows = append(rows, &careplan.PickupSchedule{
			StudentID: studentID, Weekday: item.Weekday, PickupTime: parsed, Notes: item.Notes,
			CreatedBy: createdBy, Source: careplan.ScheduleSourceStaff,
		})
	}
	return rows, nil
}

func pickupArrivalScheduleRows(studentID, createdBy int64, input []careplan.PickupAdjustmentArrivalSchedule) ([]*careplan.ArrivalSchedule, error) {
	rows := make([]*careplan.ArrivalSchedule, 0, len(input))
	for _, item := range input {
		var expectedArrival time.Time
		if item.ExpectedArrival != "" {
			parsed, err := time.Parse("2006-01-02 15:04", "2000-01-01 "+item.ExpectedArrival)
			if err != nil {
				return nil, fmt.Errorf("%w: invalid arrival time", careplan.ErrPickupAdjustmentInvalid)
			}
			expectedArrival = parsed
		}
		rows = append(rows, &careplan.ArrivalSchedule{
			StudentID: studentID, Weekday: item.Weekday, ExpectedArrival: expectedArrival,
			Notes: item.Notes, CreatedBy: createdBy,
		})
	}
	return rows, nil
}

func selectsExactPickupOffering(selections []careplan.OfferingChangeSelection, matches []careplan.PickupOfferingMatch) bool {
	for _, candidate := range matches {
		if offeringSelectionsEqual(selections, candidate.Selections) {
			return true
		}
	}
	return false
}

func offeringSelectionsEqual(left, right []careplan.OfferingChangeSelection) bool {
	if len(left) != len(right) {
		return false
	}
	left, right = cloneOfferingSelections(left), cloneOfferingSelections(right)
	sort.Slice(left, func(i, j int) bool { return left[i].OfferingID < left[j].OfferingID })
	sort.Slice(right, func(i, j int) bool { return right[i].OfferingID < right[j].OfferingID })
	for i := range left {
		if left[i].OfferingID != right[i].OfferingID ||
			!slices.Equal(canonicalDays(left[i].SelectedDays), canonicalDays(right[i].SelectedDays)) {
			return false
		}
	}
	return true
}

func pickupOfferingSelectedDays(item careplan.OfferingChangeCatalogItem, careDays []string) ([]string, bool) {
	if !item.IsActive || item.Selected || !item.CountsAsCare {
		return nil, false
	}
	if item.DaysOfWeekMode == daysOfWeekModeFixed {
		return nil, slices.Equal(canonicalDays(item.AvailableDays), careDays)
	}
	if !canonicalDaySubset(careDays, item.AvailableDays) {
		return nil, false
	}
	return slices.Clone(careDays), true
}

// pickupOfferingCandidateSelections keeps the child's other non-care
// bookings, drops the ones the target excludes through its selection group,
// and adds the target.
func pickupOfferingCandidateSelections(
	catalog *careplan.OfferingChangeCatalog,
	target careplan.OfferingChangeCatalogItem,
	selectedDays []string,
) []careplan.OfferingChangeSelection {
	selections := make([]careplan.OfferingChangeSelection, 0)
	for _, item := range catalog.Items {
		if !item.Selected || item.Automatic || item.CountsAsCare ||
			(target.SelectionGroup != "" && item.SelectionGroup == target.SelectionGroup) {
			continue
		}
		selections = append(selections, careplan.OfferingChangeSelection{
			OfferingID: item.OfferingID, SelectedDays: slices.Clone(item.SelectedDays),
		})
	}
	return append(selections, careplan.OfferingChangeSelection{
		OfferingID: target.OfferingID, SelectedDays: slices.Clone(selectedDays),
	})
}

func materializedPickupMatches(profile map[string]string, proposed map[int]careplan.PickupAdjustmentSchedule) bool {
	if len(profile) != len(proposed) {
		return false
	}
	for weekday, row := range proposed {
		if strings.TrimSpace(profile[canonicalDayForISOWeekday(weekday)]) != row.PickupTime {
			return false
		}
	}
	return true
}

func cloneOfferingSelections(input []careplan.OfferingChangeSelection) []careplan.OfferingChangeSelection {
	result := make([]careplan.OfferingChangeSelection, 0, len(input))
	for _, item := range input {
		result = append(result, careplan.OfferingChangeSelection{OfferingID: item.OfferingID, SelectedDays: slices.Clone(item.SelectedDays)})
	}
	return result
}

func pickupPlanDeviates(careDays []int, proposed map[int]careplan.PickupAdjustmentSchedule, offering careplan.PickupWeek) bool {
	if !pickupPlanHasExactlyDays(proposed, careDays) {
		return true
	}
	offeringDays := make([]int, 0, len(offering))
	for weekday, row := range offering {
		if row != nil {
			offeringDays = append(offeringDays, weekday)
		}
	}
	slices.Sort(offeringDays)
	if !slices.Equal(careDays, offeringDays) {
		return true
	}
	for _, weekday := range careDays {
		row := offering[weekday]
		if row == nil || row.PickupTime.Format("15:04") != proposed[weekday].PickupTime {
			return true
		}
	}
	return false
}

func pickupPlanHasExactlyDays(plan map[int]careplan.PickupAdjustmentSchedule, careDays []int) bool {
	days := make([]int, 0, len(plan))
	for weekday := range plan {
		days = append(days, weekday)
	}
	slices.Sort(days)
	expected := slices.Clone(careDays)
	slices.Sort(expected)
	return slices.Equal(days, expected)
}

// effectiveProposedPickupPlan fills the care days without an explicit time
// from the booked offering.
func effectiveProposedPickupPlan(
	careDays []int,
	explicit map[int]careplan.PickupAdjustmentSchedule,
	offering careplan.PickupWeek,
) map[int]careplan.PickupAdjustmentSchedule {
	result := make(map[int]careplan.PickupAdjustmentSchedule, len(explicit)+len(careDays))
	for weekday, row := range explicit {
		result[weekday] = row
	}
	for _, weekday := range careDays {
		if _, exists := result[weekday]; exists {
			continue
		}
		if row := offering[weekday]; row != nil {
			result[weekday] = careplan.PickupAdjustmentSchedule{
				Weekday: weekday, PickupTime: row.PickupTime.Format("15:04"), Notes: row.Notes,
			}
		}
	}
	return result
}

func hasManualPickupRows(rows []careplan.PickupSchedule) bool {
	return slices.ContainsFunc(rows, func(row careplan.PickupSchedule) bool {
		return row.Source != careplan.ScheduleSourceCareOffering
	})
}

func pickupPlanLabel(week careplan.PickupWeek) string {
	rows := make([]careplan.PickupAdjustmentSchedule, 0, len(week))
	for weekday, row := range week {
		if row != nil {
			rows = append(rows, careplan.PickupAdjustmentSchedule{
				Weekday: weekday, PickupTime: row.PickupTime.Format("15:04"), Notes: row.Notes,
			})
		}
	}
	return proposedPickupPlanLabel(rows)
}

func proposedPickupPlanLabel(rows []careplan.PickupAdjustmentSchedule) string {
	rows = slices.Clone(rows)
	sort.Slice(rows, func(i, j int) bool { return rows[i].Weekday < rows[j].Weekday })
	parts := make([]string, 0, len(rows))
	for _, row := range rows {
		label := fmt.Sprintf("%s %s Uhr", shortGermanWeekday(row.Weekday), row.PickupTime)
		if row.Notes != nil && strings.TrimSpace(*row.Notes) != "" {
			label += fmt.Sprintf(" (Notiz: %s)", *row.Notes)
		}
		parts = append(parts, label)
	}
	if len(parts) == 0 {
		return "Kein Wochenplan"
	}
	return strings.Join(parts, ", ")
}

func proposedPickupPlanMapLabel(rows map[int]careplan.PickupAdjustmentSchedule) string {
	list := make([]careplan.PickupAdjustmentSchedule, 0, len(rows))
	for _, row := range rows {
		list = append(list, row)
	}
	return proposedPickupPlanLabel(list)
}

func shortGermanWeekday(weekday int) string {
	return map[int]string{1: "Mo", 2: "Di", 3: "Mi", 4: "Do", 5: "Fr"}[weekday]
}

func canonicalDaysFromWeekdays(weekdays []int) []string {
	days := make([]string, 0, len(weekdays))
	for _, weekday := range weekdays {
		days = append(days, canonicalDayForISOWeekday(weekday))
	}
	return canonicalDays(days)
}

func canonicalDayForISOWeekday(weekday int) string {
	return map[int]string{1: "mon", 2: "tue", 3: "wed", 4: "thu", 5: "fri", 6: "sat", 7: "sun"}[weekday]
}

func canonicalDaySubset(days, available []string) bool {
	available = canonicalDays(available)
	for _, day := range days {
		if !slices.Contains(available, day) {
			return false
		}
	}
	return true
}

func isoWeekday(date calendar.Date) int {
	weekday := int(date.Weekday())
	if weekday == 0 {
		return 7
	}
	return weekday
}

// pickupTokenContent is everything a preview token binds an apply to. The
// field names are part of the token.
type pickupTokenContent struct {
	TenantID       int64
	Input          careplan.PickupAdjustmentPreviewInput
	Preview        *careplan.PickupAdjustmentPreview
	Current        careplan.PickupWeek
	Offering       careplan.PickupWeek
	CurrentArrival []careplan.PickupAdjustmentArrivalSchedule
}

func (s *PickupAdjustments) pickupAdjustmentToken(content pickupTokenContent) (string, error) {
	encoded, err := json.Marshal(content)
	if err != nil {
		return "", fmt.Errorf("pickup adjustment: build preview token: %w", err)
	}
	return s.deps.Fingerprint(encoded), nil
}

// samePickupAdjustmentToken compares two preview tokens as SHA-256 values.
// The token is a content fingerprint, not a secret.
func samePickupAdjustmentToken(left, right string) bool {
	leftBytes, leftErr := hex.DecodeString(strings.TrimSpace(left))
	rightBytes, rightErr := hex.DecodeString(strings.TrimSpace(right))
	return leftErr == nil && rightErr == nil && len(leftBytes) == pickupPreviewTokenSize &&
		len(rightBytes) == pickupPreviewTokenSize && bytes.Equal(leftBytes, rightBytes)
}
