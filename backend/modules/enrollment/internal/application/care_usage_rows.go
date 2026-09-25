package application

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

func normalizeCareUsageFilters(filters enrollment.CareUsageFilters) enrollment.CareUsageFilters {
	filters.Status = strings.ToLower(strings.TrimSpace(filters.Status))
	if filters.Status == "" {
		filters.Status = enrollment.ChildStatusApproved
	}
	filters.Search = strings.TrimSpace(filters.Search)
	filters.Weekday = strings.ToLower(strings.TrimSpace(filters.Weekday))
	filters.PickupTime = strings.TrimSpace(filters.PickupTime)
	filters.CareOfferingIDs = dedupePositiveInt64(filters.CareOfferingIDs)
	if filters.CareOfferingIDsSet && filters.CareOfferingIDs == nil {
		filters.CareOfferingIDs = []int64{}
	}
	return filters
}

func validateCareUsageFilters(filters enrollment.CareUsageFilters) error {
	if filters.Status != "all" && !validChildStatusFilters[filters.Status] {
		return fmt.Errorf("%w: unsupported status %q", enrollment.ErrReportInvalidFilter, filters.Status)
	}
	if filters.DayCount != nil && (*filters.DayCount < 0 || *filters.DayCount > 7) {
		return fmt.Errorf("%w: day_count must be between 0 and 7", enrollment.ErrReportInvalidFilter)
	}
	if filters.Weekday != "" && !enrollment.ValidWeekdays[filters.Weekday] {
		return fmt.Errorf("%w: weekday must be one of mon/tue/wed/thu/fri", enrollment.ErrReportInvalidFilter)
	}
	if filters.PickupTime != "" {
		if _, err := time.Parse("15:04", filters.PickupTime); err != nil {
			return fmt.Errorf("%w: pickup_time must be HH:MM", enrollment.ErrReportInvalidFilter)
		}
	}
	return nil
}

var validChildStatusFilters = map[string]bool{
	enrollment.ChildStatusSubmitted:          true,
	enrollment.ChildStatusUnderReview:        true,
	enrollment.ChildStatusApproved:           true,
	enrollment.ChildStatusWaitlisted:         true,
	enrollment.ChildStatusRejected:           true,
	enrollment.ChildStatusWithdrawn:          true,
	enrollment.ChildStatusPendingRenewal:     true,
	enrollment.ChildStatusAutoRenewed:        true,
	enrollment.ChildStatusPendingAdminReview: true,
}

func careUsageAppliedFilters(filters enrollment.CareUsageFilters) enrollment.CareUsageAppliedFilters {
	return enrollment.CareUsageAppliedFilters{
		PhaseID:         filters.PhaseID,
		Status:          filters.Status,
		CareOfferingIDs: filters.CareOfferingIDs,
		DayCount:        filters.DayCount,
		GradeLevel:      filters.GradeLevel,
		Weekday:         filters.Weekday,
		PickupTime:      filters.PickupTime,
		Search:          filters.Search,
	}
}

func normalizedCareUsageOfferingIDs(ids []int64, offerings []*enrollmentModels.CareOffering, explicit bool) []int64 {
	if explicit {
		normalized := dedupePositiveInt64(ids)
		if normalized == nil {
			return []int64{}
		}
		return normalized
	}
	out := make([]int64, 0, len(offerings))
	for _, offering := range offerings {
		if offering.CountsAsCare {
			out = append(out, offering.ID)
		}
	}
	return out
}

func makeIDSet(ids []int64) map[int64]bool {
	set := make(map[int64]bool, len(ids))
	for _, id := range ids {
		if id > 0 {
			set[id] = true
		}
	}
	return set
}

// dedupePositiveInt64 keeps the positive ids once, sorted. Empty input stays
// nil; input without a positive id becomes an empty slice.
func dedupePositiveInt64(ids []int64) []int64 {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[int64]bool, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id > 0 && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out
}

func careUsageRow(req *enrollmentModels.Request, child *reportChild, links []*enrollment.RequestChildOfferingRecord, offeringByID map[int64]*enrollmentModels.CareOffering, includedOfferingIDs map[int64]bool, pickupByDay map[string]string) enrollment.CareUsageRow {
	daySet := map[string]bool{}
	rowOfferings := make([]enrollment.CareUsageRowOffering, 0, len(links))
	for _, link := range links {
		rowOffering := careUsageRowOffering(link, offeringByID[link.CareOfferingID])
		if includedOfferingIDs[link.CareOfferingID] {
			for _, day := range rowOffering.Days {
				daySet[day] = true
			}
		}
		rowOfferings = append(rowOfferings, rowOffering)
	}
	sort.SliceStable(rowOfferings, func(i, j int) bool {
		return rowOfferings[i].Name < rowOfferings[j].Name
	})
	effectiveDays := sortedDayCodes(slices.Collect(maps.Keys(daySet)))
	if pickupByDay == nil {
		pickupByDay = map[string]string{}
	}
	return enrollment.CareUsageRow{
		RequestID:         req.ID,
		ChildID:           child.ID,
		ChildFirstName:    child.FirstName,
		ChildLastName:     child.LastName,
		DateOfBirth:       string(child.DateOfBirth),
		TargetGradeLevel:  child.TargetGradeLevel,
		TargetSchoolClass: child.TargetSchoolClass,
		Status:            child.Status,
		Offerings:         rowOfferings,
		EffectiveDays:     effectiveDays,
		DayCount:          len(effectiveDays),
		PickupByDay:       pickupByDay,
		GuardianFirstName: req.GuardianFirstName,
		GuardianLastName:  req.GuardianLastName,
		GuardianEmail:     req.GuardianEmail,
		GuardianPhone:     req.GuardianPhone,
		SubmittedAt:       req.SubmittedAt,
	}
}

// careUsageRowOffering renders one booked offering; a fixed-days offering
// without selected days books its available days.
func careUsageRowOffering(link *enrollment.RequestChildOfferingRecord, offering *enrollmentModels.CareOffering) enrollment.CareUsageRowOffering {
	name := "Angebot #" + strconv.FormatInt(link.CareOfferingID, 10)
	daysOfWeekMode := ""
	days := link.SelectedDays
	source := "selected"
	if offering != nil {
		name = offering.Name
		daysOfWeekMode = offering.DaysOfWeekMode
		if len(days) == 0 && offering.DaysOfWeekMode == enrollmentModels.DaysOfWeekModeFixed {
			days = offering.AvailableDays
			source = "available"
		}
	}
	return enrollment.CareUsageRowOffering{
		ID:                    link.CareOfferingID,
		Name:                  name,
		Days:                  sortedDayCodes(days),
		DaysSource:            source,
		DaysOfWeekMode:        daysOfWeekMode,
		ManualSelectedDays:    sortedDayCodes(link.ManualSelectedDays),
		AutomaticSelectedDays: sortedDayCodes(link.AutomaticSelectedDays),
	}
}

func careUsageBookedPickupDays(row enrollment.CareUsageRow, filters enrollment.CareUsageFilters) []string {
	return careUsageDays(row, filters, func(day string) bool {
		return strings.TrimSpace(row.PickupByDay[day]) != ""
	})
}

func careUsageBookedDays(row enrollment.CareUsageRow, filters enrollment.CareUsageFilters) []string {
	return careUsageDays(row, filters, func(string) bool { return true })
}

// careUsageDays collects the booked day codes from the row's (filtered)
// offerings, falling back to the effective days when none match; include
// narrows the set (e.g. to days that carry a pickup time).
func careUsageDays(row enrollment.CareUsageRow, filters enrollment.CareUsageFilters, include func(day string) bool) []string {
	daySet := map[string]bool{}
	includedOfferingIDs := makeIDSet(filters.CareOfferingIDs)
	for _, offering := range row.Offerings {
		if filters.CareOfferingIDsSet && !includedOfferingIDs[offering.ID] {
			continue
		}
		addIncludedDays(daySet, offering.Days, include)
	}
	if len(daySet) == 0 {
		addIncludedDays(daySet, row.EffectiveDays, include)
	}
	return sortedDayCodes(slices.Collect(maps.Keys(daySet)))
}

func addIncludedDays(daySet map[string]bool, days []string, include func(day string) bool) {
	for _, day := range days {
		day = strings.ToLower(strings.TrimSpace(day))
		if day != "" && include(day) {
			daySet[day] = true
		}
	}
}

func careUsageRowMatches(row enrollment.CareUsageRow, filters enrollment.CareUsageFilters) bool {
	if filters.Status != "all" && row.Status != filters.Status {
		return false
	}
	if filters.CareOfferingIDsSet && !careUsageRowHasAnyOffering(row, filters.CareOfferingIDs) {
		return false
	}
	if filters.DayCount != nil && row.DayCount != *filters.DayCount {
		return false
	}
	if filters.GradeLevel != nil && (row.TargetGradeLevel == nil || *row.TargetGradeLevel != *filters.GradeLevel) {
		return false
	}
	return careUsageRowMatchesDays(row, filters) && careUsageRowMatchesSearch(row, filters.Search)
}

func careUsageRowMatchesDays(row enrollment.CareUsageRow, filters enrollment.CareUsageFilters) bool {
	if filters.Weekday != "" {
		if !slices.Contains(careUsageBookedDays(row, filters), filters.Weekday) {
			return false
		}
		if filters.PickupTime != "" && row.PickupByDay[filters.Weekday] != filters.PickupTime {
			return false
		}
	}
	if filters.PickupTime == "" {
		return true
	}
	for _, day := range careUsageBookedPickupDays(row, filters) {
		if row.PickupByDay[day] == filters.PickupTime {
			return true
		}
	}
	return false
}

func careUsageRowMatchesSearch(row enrollment.CareUsageRow, search string) bool {
	if search == "" {
		return true
	}
	haystack := strings.ToLower(strings.Join([]string{
		row.ChildFirstName,
		row.ChildLastName,
		row.GuardianFirstName,
		row.GuardianLastName,
		row.GuardianEmail,
	}, " "))
	return strings.Contains(haystack, strings.ToLower(search))
}

func careUsageRowHasAnyOffering(row enrollment.CareUsageRow, offeringIDs []int64) bool {
	if len(offeringIDs) == 0 {
		return false
	}
	selected := make(map[int64]bool, len(row.Offerings))
	for _, offering := range row.Offerings {
		selected[offering.ID] = true
	}
	for _, id := range offeringIDs {
		if selected[id] {
			return true
		}
	}
	return false
}

func careUsageOfferingOptions(offerings []*enrollmentModels.CareOffering) []enrollment.CareUsageOfferingOption {
	options := make([]enrollment.CareUsageOfferingOption, 0, len(offerings))
	for _, offering := range offerings {
		options = append(options, enrollment.CareUsageOfferingOption{
			ID:           offering.ID,
			Name:         offering.Name,
			CountsAsCare: offering.CountsAsCare,
		})
	}
	sort.SliceStable(options, func(i, j int) bool {
		return options[i].Name < options[j].Name
	})
	return options
}

func careUsageOfferingStats(stats map[int64]*enrollment.CareUsageOfferingStat) []enrollment.CareUsageOfferingStat {
	out := make([]enrollment.CareUsageOfferingStat, 0, len(stats))
	for _, stat := range stats {
		out = append(out, *stat)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Children != out[j].Children {
			return out[i].Children > out[j].Children
		}
		return out[i].OfferingName < out[j].OfferingName
	})
	return out
}

func careUsageGradeOptions(seen map[int16]bool) []int16 {
	out := make([]int16, 0, len(seen))
	for grade := range seen {
		out = append(out, grade)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func initDayCountMap() map[string]int {
	return map[string]int{"0": 0, "1": 0, "2": 0, "3": 0, "4": 0, "5": 0, "6": 0, "7": 0}
}

func initWeekdayPickupTimeMap() map[string]map[string]int {
	return map[string]map[string]int{
		"mon": {},
		"tue": {},
		"wed": {},
		"thu": {},
		"fri": {},
	}
}

var dayOrder = map[string]int{
	"mon": 1,
	"tue": 2,
	"wed": 3,
	"thu": 4,
	"fri": 5,
	"sat": 6,
	"sun": 7,
}

func sortedDayCodes(days []string) []string {
	if len(days) == 0 {
		return []string{}
	}
	seen := make(map[string]bool, len(days))
	out := make([]string, 0, len(days))
	for _, day := range days {
		canonical := strings.ToLower(strings.TrimSpace(day))
		if canonical == "" || seen[canonical] {
			continue
		}
		seen[canonical] = true
		out = append(out, canonical)
	}
	sort.SliceStable(out, func(i, j int) bool {
		oi, okI := dayOrder[out[i]]
		oj, okJ := dayOrder[out[j]]
		if okI && okJ {
			return oi < oj
		}
		if okI != okJ {
			return okI
		}
		return out[i] < out[j]
	})
	return out
}

func sortedPickupTimes(seen map[string]bool) []string {
	out := make([]string, 0, len(seen))
	for pickupTime := range seen {
		out = append(out, pickupTime)
	}
	sort.Strings(out)
	return out
}
