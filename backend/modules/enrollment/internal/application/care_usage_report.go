package application

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	offeringContract "github.com/moto-nrw/project-phoenix/modules/enrollment/offeringcontract"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// careUsageInputs are the owner reads one care usage report combines.
type careUsageInputs struct {
	phaseID      int64
	phase        *enrollmentNative.Phase
	requestByID  map[int64]*Request
	children     []*reportChild
	offerings    []*offeringContract.CareOfferingRecord
	offeringByID map[int64]*offeringContract.CareOfferingRecord
	linksByChild map[int64][]*enrollmentNative.RequestChildOfferingRecord
	schemas      map[int64]*enrollmentNative.FormSchema
	offeringDate calendar.Date
}

// careUsageTally accumulates the filter options over every child, matching
// or not, and the per-offering statistics over the matching ones.
type careUsageTally struct {
	gradeSeen      map[int16]bool
	pickupTimeSeen map[string]bool
	offeringStats  map[int64]*enrollmentNative.CareUsageOfferingStat
}

func (s *Reports) CareUsage(ctx context.Context, filters enrollmentNative.CareUsageFilters) (*enrollmentNative.CareUsageReport, error) {
	return s.careUsage(ctx, filters, false)
}

func (s *Reports) ExportCareUsage(ctx context.Context, filters enrollmentNative.CareUsageFilters, actorAccountID int64, actorRole, format string, compact bool) (*enrollmentNative.CareUsageReport, error) {
	report, err := s.careUsage(ctx, filters, compact)
	if err != nil {
		return nil, err
	}
	if err := s.recordCareUsageExportAudit(ctx, report, actorAccountID, actorRole, format, compact); err != nil {
		return nil, err
	}
	return report, nil
}

func (s *Reports) careUsage(ctx context.Context, filters enrollmentNative.CareUsageFilters, enrichCompact bool) (*enrollmentNative.CareUsageReport, error) {
	filters = normalizeCareUsageFilters(filters)
	if err := validateCareUsageFilters(filters); err != nil {
		return nil, err
	}
	if filters.PhaseID <= 0 {
		return nil, fmt.Errorf("care usage report: phase_id required")
	}
	in, err := s.loadCareUsageInputs(ctx, filters.PhaseID)
	if err != nil {
		return nil, err
	}
	filters.CareOfferingIDs = normalizedCareUsageOfferingIDs(filters.CareOfferingIDs, in.offerings, filters.CareOfferingIDsSet)
	report, err := buildCareUsageReport(in, filters)
	if err != nil {
		return nil, err
	}
	if enrichCompact {
		if err := s.enrichCompactCareUsage(ctx, report, in); err != nil {
			return nil, err
		}
	}
	return report, nil
}

func (s *Reports) loadCareUsageInputs(ctx context.Context, phaseID int64) (*careUsageInputs, error) {
	phase, err := s.deps.Phases.Phase(ctx, phaseID)
	if err != nil {
		return nil, fmt.Errorf("care usage report: phase %d: %w", phaseID, enrollmentNative.ErrReportPhaseNotFound)
	}
	requests, err := listReportRequests(ctx, s.deps.Requests, enrollmentNative.RequestListFilters{PhaseID: phaseID})
	if err != nil {
		return nil, fmt.Errorf("care usage report: list requests: %w", err)
	}
	if len(requests) > maxExportRequests {
		return nil, fmt.Errorf("care usage report: %d requests: %w", len(requests), enrollmentNative.ErrReportExportTooLarge)
	}
	in := &careUsageInputs{phaseID: phaseID, phase: phase, requestByID: make(map[int64]*Request, len(requests))}
	reqIDs := make([]int64, 0, len(requests))
	for _, req := range requests {
		reqIDs = append(reqIDs, req.ID)
		in.requestByID[req.ID] = req
	}
	if in.children, err = listReportChildren(ctx, s.deps.Children, reqIDs); err != nil {
		return nil, fmt.Errorf("care usage report: list children: %w", err)
	}
	if len(in.children) > maxReportRows {
		return nil, fmt.Errorf("care usage report: %d children: %w", len(in.children), enrollmentNative.ErrReportExportTooLarge)
	}
	if err := s.loadCareUsageSelections(ctx, in); err != nil {
		return nil, err
	}
	if in.schemas, err = s.loadSchemas(ctx, requests); err != nil {
		return nil, err
	}
	return in, nil
}

// loadCareUsageSelections reads the children's offering selections on the
// report's offering date and the phase's offerings.
func (s *Reports) loadCareUsageSelections(ctx context.Context, in *careUsageInputs) error {
	childIDs := make([]int64, 0, len(in.children))
	for _, child := range in.children {
		childIDs = append(childIDs, child.ID)
	}
	in.offeringDate = enrollmentNative.ReportOfferingDate(s.today(), in.phase)
	values, err := s.deps.Children.RequestChildOfferingsForChildrenAtDate(ctx, childIDs, enrollmentNative.Date(in.offeringDate))
	links := enrollmentNative.RequestChildOfferingRecordsOf(values)
	if err != nil {
		return fmt.Errorf("care usage report: list child offerings: %w", err)
	}
	if in.offerings, err = s.deps.Offerings.ListByPhase(ctx, in.phaseID); err != nil {
		return fmt.Errorf("care usage report: list offerings: %w", err)
	}
	in.offeringByID = make(map[int64]*offeringContract.CareOfferingRecord, len(in.offerings))
	for _, offering := range in.offerings {
		in.offeringByID[offering.ID] = offering
	}
	in.linksByChild = make(map[int64][]*enrollmentNative.RequestChildOfferingRecord, len(childIDs))
	for _, link := range links {
		in.linksByChild[link.RequestChildID] = append(in.linksByChild[link.RequestChildID], link)
	}
	return nil
}

func (s *Reports) loadSchemas(ctx context.Context, requests []*Request) (map[int64]*enrollmentNative.FormSchema, error) {
	schemas, err := loadReportSchemas(ctx, s.deps.Schemas, requests)
	if err != nil {
		return nil, fmt.Errorf("care usage report: load schemas: %w", err)
	}
	return schemas, nil
}

func buildCareUsageReport(in *careUsageInputs, filters enrollmentNative.CareUsageFilters) (*enrollmentNative.CareUsageReport, error) {
	report := &enrollmentNative.CareUsageReport{
		Phase:   enrollmentNative.CareUsagePhase{ID: in.phase.ID, Name: in.phase.Name},
		Filters: careUsageAppliedFilters(filters),
		Totals: enrollmentNative.CareUsageTotals{
			ByDayCount:          initDayCountMap(),
			ByWeekdayPickupTime: initWeekdayPickupTimeMap(),
		},
		FilterOptions: enrollmentNative.CareUsageFilterOptions{
			Offerings: careUsageOfferingOptions(in.offerings),
		},
	}
	includedOfferingIDs := makeIDSet(filters.CareOfferingIDs)
	tally := careUsageTally{
		gradeSeen:      map[int16]bool{},
		pickupTimeSeen: map[string]bool{},
		offeringStats:  make(map[int64]*enrollmentNative.CareUsageOfferingStat, len(in.offerings)),
	}
	for _, child := range in.children {
		req := in.requestByID[child.RequestID]
		if req == nil {
			continue
		}
		pickupByDay, err := careUsagePickupByDay(req, child, in.schemas)
		if err != nil {
			return nil, fmt.Errorf("care usage report: child %d pickup schedule: %w", child.ID, err)
		}
		row := careUsageRow(req, child, in.linksByChild[child.ID], in.offeringByID, includedOfferingIDs, pickupByDay)
		tally.observeOptions(child, row)
		if !careUsageRowMatches(row, filters) {
			continue
		}
		report.Rows = append(report.Rows, row)
		tally.count(report, row, filters)
	}
	sort.SliceStable(report.Rows, func(i, j int) bool {
		return compareGermanNames(
			report.Rows[i].ChildLastName, report.Rows[i].ChildFirstName,
			report.Rows[j].ChildLastName, report.Rows[j].ChildFirstName,
		) < 0
	})
	report.ByOffering = careUsageOfferingStats(tally.offeringStats)
	report.FilterOptions.GradeLevels = careUsageGradeOptions(tally.gradeSeen)
	report.FilterOptions.PickupTimes = sortedPickupTimes(tally.pickupTimeSeen)
	return report, nil
}

func (t careUsageTally) observeOptions(child *reportChild, row enrollmentNative.CareUsageRow) {
	if child.TargetGradeLevel != nil {
		t.gradeSeen[*child.TargetGradeLevel] = true
	}
	for _, pickupTime := range row.PickupByDay {
		if pickupTime != "" {
			t.pickupTimeSeen[pickupTime] = true
		}
	}
}

func (t careUsageTally) count(report *enrollmentNative.CareUsageReport, row enrollmentNative.CareUsageRow, filters enrollmentNative.CareUsageFilters) {
	report.Totals.Children++
	report.Totals.ByDayCount[strconv.Itoa(row.DayCount)]++
	for _, day := range careUsageBookedPickupDays(row, filters) {
		pickupTime := row.PickupByDay[day]
		if pickupTime == "" {
			continue
		}
		if report.Totals.ByWeekdayPickupTime[day] == nil {
			report.Totals.ByWeekdayPickupTime[day] = map[string]int{}
		}
		report.Totals.ByWeekdayPickupTime[day][pickupTime]++
	}
	for _, offering := range row.Offerings {
		stat := t.offeringStats[offering.ID]
		if stat == nil {
			stat = &enrollmentNative.CareUsageOfferingStat{
				OfferingID:   offering.ID,
				OfferingName: offering.Name,
				ByDayCount:   initDayCountMap(),
			}
			t.offeringStats[offering.ID] = stat
		}
		stat.Children++
		stat.ByDayCount[strconv.Itoa(row.DayCount)]++
	}
}

// enrichCompactCareUsage adds the compact export's display columns: the
// care days, every request guardian and the maintained Kind-Gehzeit.
func (s *Reports) enrichCompactCareUsage(ctx context.Context, report *enrollmentNative.CareUsageReport, in *careUsageInputs) error {
	requestIDs := make([]int64, 0, len(report.Rows))
	childByID := make(map[int64]*reportChild, len(in.children))
	for _, child := range in.children {
		childByID[child.ID] = child
	}
	studentIDs := make([]int64, 0, len(report.Rows))
	for _, row := range report.Rows {
		requestIDs = append(requestIDs, row.RequestID)
		if id := careUsageStudentID(childByID[row.ChildID]); id != 0 {
			studentIDs = append(studentIDs, id)
		}
	}
	guardiansByRequest := map[int64][]*enrollmentNative.RequestGuardian{}
	if s.deps.Guardians != nil && len(requestIDs) > 0 {
		guardians, err := s.deps.Guardians.RequestGuardians(ctx, requestIDs)
		if err != nil {
			return fmt.Errorf("care usage report: list request guardians: %w", err)
		}
		guardiansByRequest = requestGuardiansByRequestID(guardians)
	}
	schedulePickup, err := s.schedulePickupByStudentIDs(ctx, studentIDs, in.offeringDate)
	if err != nil {
		return fmt.Errorf("care usage report: %w", err)
	}
	careOfferingsEnabled, err := s.careOfferingsEnabled(ctx, "care usage report")
	if err != nil {
		return err
	}
	for i := range report.Rows {
		row := &report.Rows[i]
		row.CareDays = classRosterCareDays(row.EffectiveDays, in.offeringByID, careOfferingsEnabled)
		row.Guardians = classRosterEnrollmentGuardians(in.requestByID[row.RequestID], guardiansByRequest[row.RequestID])
		if byDay := schedulePickup[careUsageStudentID(childByID[row.ChildID])]; byDay != nil {
			row.SchedulePickupByDay = byDay
		}
	}
	return nil
}

// careOfferingsEnabledKey is the setting the class roster and the compact
// care usage export resolve; the error texts name it.
const careOfferingsEnabledKey = "enrollment.care_offerings_enabled"

// careOfferingsEnabled resolves the setting; without the binding it follows
// the registry default (enabled).
func (s *Reports) careOfferingsEnabled(ctx context.Context, errPrefix string) (bool, error) {
	if s.deps.Settings == nil {
		return true, nil
	}
	enabled, err := s.deps.Settings.CareOfferingsEnabled(ctx)
	if err != nil {
		return false, fmt.Errorf("%s: resolve %s: %w", errPrefix, careOfferingsEnabledKey, err)
	}
	return enabled, nil
}

// careUsageStudentID resolves the student of an approved request child
// (rollout-created or matched existing student). Unapproved children return
// 0 because their form snapshot must not be replaced by live student data.
func careUsageStudentID(child *reportChild) int64 {
	if child == nil || child.Status != enrollmentNative.ChildStatusApproved {
		return 0
	}
	if child.CreatedStudentID != nil {
		return *child.CreatedStudentID
	}
	if child.MatchedStudentID != nil {
		return *child.MatchedStudentID
	}
	return 0
}

// classRosterISOWeekdayDay maps pickup-schedule weekdays (ISO, Mon=1) onto
// the report day codes used across the roster maps.
var classRosterISOWeekdayDay = map[int]string{1: "mon", 2: "tue", 3: "wed", 4: "thu", 5: "fri"}

// schedulePickupByStudentIDs maps student ID -> day code -> effective pickup
// time (HH:MM) on date, shared by the class-roster and care-usage reports.
// Without the schedule binding the column stays empty.
func (s *Reports) schedulePickupByStudentIDs(ctx context.Context, studentIDs []int64, date calendar.Date) (map[int64]map[string]string, error) {
	out := map[int64]map[string]string{}
	if s.deps.PickupSchedules == nil || len(studentIDs) == 0 {
		return out, nil
	}
	rows, err := s.deps.PickupSchedules.GetWeeklySchedulesByStudentIDsForDate(ctx, studentIDs, date)
	if err != nil {
		return nil, fmt.Errorf("list pickup schedules: %w", err)
	}
	for _, row := range rows {
		day, ok := classRosterISOWeekdayDay[row.Weekday]
		if !ok {
			continue
		}
		if out[row.StudentID] == nil {
			out[row.StudentID] = map[string]string{}
		}
		out[row.StudentID][day] = row.PickupTime.Format("15:04")
	}
	return out, nil
}
