package enrollment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

var (
	ErrReportPhaseNotFound  = errors.New("enrollment report phase not found")
	ErrReportExportTooLarge = errors.New("enrollment report has too many rows for a single export")
	ErrReportInvalidFilter  = errors.New("enrollment report filter is invalid")
)

const maxReportRows = 10000

type CareUsageFilters struct {
	PhaseID            int64   `json:"phase_id"`
	Status             string  `json:"status,omitempty"`
	CareOfferingIDs    []int64 `json:"care_offering_ids,omitempty"`
	CareOfferingIDsSet bool    `json:"-"`
	DayCount           *int    `json:"day_count,omitempty"`
	GradeLevel         *int16  `json:"grade_level,omitempty"`
	Weekday            string  `json:"weekday,omitempty"`
	PickupTime         string  `json:"pickup_time,omitempty"`
	Search             string  `json:"search,omitempty"`
}

type CareUsageReport struct {
	Phase         CareUsagePhase          `json:"phase"`
	Filters       CareUsageAppliedFilters `json:"filters"`
	Totals        CareUsageTotals         `json:"totals"`
	ByOffering    []CareUsageOfferingStat `json:"by_offering"`
	FilterOptions CareUsageFilterOptions  `json:"filter_options"`
	Rows          []CareUsageRow          `json:"rows"`
}

type CareUsagePhase struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type CareUsageAppliedFilters struct {
	PhaseID         int64   `json:"phase_id"`
	Status          string  `json:"status"`
	CareOfferingIDs []int64 `json:"care_offering_ids"`
	DayCount        *int    `json:"day_count,omitempty"`
	GradeLevel      *int16  `json:"grade_level,omitempty"`
	Weekday         string  `json:"weekday,omitempty"`
	PickupTime      string  `json:"pickup_time,omitempty"`
	Search          string  `json:"search,omitempty"`
}

type CareUsageTotals struct {
	Children            int                       `json:"children"`
	ByDayCount          map[string]int            `json:"by_day_count"`
	ByWeekdayPickupTime map[string]map[string]int `json:"by_weekday_pickup_time"`
}

type CareUsageOfferingStat struct {
	OfferingID   int64          `json:"offering_id"`
	OfferingName string         `json:"offering_name"`
	Children     int            `json:"children"`
	ByDayCount   map[string]int `json:"by_day_count"`
}

type CareUsageFilterOptions struct {
	Offerings   []CareUsageOfferingOption `json:"offerings"`
	GradeLevels []int16                   `json:"grade_levels"`
	PickupTimes []string                  `json:"pickup_times"`
}

type CareUsageOfferingOption struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	CountsAsCare bool   `json:"counts_as_care"`
}

type CareUsageRow struct {
	RequestID         int64                  `json:"request_id"`
	ChildID           int64                  `json:"child_id"`
	ChildFirstName    string                 `json:"child_first_name"`
	ChildLastName     string                 `json:"child_last_name"`
	DateOfBirth       string                 `json:"date_of_birth"`
	TargetGradeLevel  *int16                 `json:"target_grade_level,omitempty"`
	Status            string                 `json:"status"`
	Offerings         []CareUsageRowOffering `json:"offerings"`
	EffectiveDays     []string               `json:"effective_days"`
	DayCount          int                    `json:"day_count"`
	PickupByDay       map[string]string      `json:"pickup_by_day"`
	GuardianFirstName string                 `json:"guardian_first_name"`
	GuardianLastName  string                 `json:"guardian_last_name"`
	GuardianEmail     string                 `json:"guardian_email"`
	GuardianPhone     *string                `json:"guardian_phone,omitempty"`
	SubmittedAt       time.Time              `json:"submitted_at"`
}

type CareUsageRowOffering struct {
	ID                    int64    `json:"id"`
	Name                  string   `json:"name"`
	Days                  []string `json:"days"`
	DaysSource            string   `json:"days_source"`
	DaysOfWeekMode        string   `json:"days_of_week_mode"`
	ManualSelectedDays    []string `json:"manual_selected_days,omitempty"`
	AutomaticSelectedDays []string `json:"automatic_selected_days,omitempty"`
}

type ReportService interface {
	CareUsage(ctx context.Context, filters CareUsageFilters) (*CareUsageReport, error)
	ExportCareUsage(ctx context.Context, filters CareUsageFilters, actorAccountID int64, actorRole, format string) (*CareUsageReport, error)
}

type ReportServiceConfig struct {
	RequestRepo              enrollmentModels.RequestRepository
	RequestChildRepo         enrollmentModels.RequestChildRepository
	RequestChildOfferingRepo enrollmentModels.RequestChildOfferingRepository
	CareOfferingRepo         enrollmentModels.CareOfferingRepository
	FormSchemaRepo           enrollmentModels.FormSchemaRepository
	PhaseRepo                enrollmentModels.PhaseRepository
	DataAccessLogRepo        auditModels.DataAccessLogRepository
}

type reportService struct {
	requestRepo              enrollmentModels.RequestRepository
	requestChildRepo         enrollmentModels.RequestChildRepository
	requestChildOfferingRepo enrollmentModels.RequestChildOfferingRepository
	careOfferingRepo         enrollmentModels.CareOfferingRepository
	formSchemaRepo           enrollmentModels.FormSchemaRepository
	phaseRepo                enrollmentModels.PhaseRepository
	dataAccessLogRepo        auditModels.DataAccessLogRepository
}

func NewReportService(cfg ReportServiceConfig) ReportService {
	return &reportService{
		requestRepo:              cfg.RequestRepo,
		requestChildRepo:         cfg.RequestChildRepo,
		requestChildOfferingRepo: cfg.RequestChildOfferingRepo,
		careOfferingRepo:         cfg.CareOfferingRepo,
		formSchemaRepo:           cfg.FormSchemaRepo,
		phaseRepo:                cfg.PhaseRepo,
		dataAccessLogRepo:        cfg.DataAccessLogRepo,
	}
}

func (s *reportService) CareUsage(ctx context.Context, filters CareUsageFilters) (*CareUsageReport, error) {
	filters = normalizeCareUsageFilters(filters)
	if err := validateCareUsageFilters(filters); err != nil {
		return nil, err
	}
	if filters.PhaseID <= 0 {
		return nil, fmt.Errorf("care usage report: phase_id required")
	}

	phase, err := s.phaseRepo.FindByID(ctx, filters.PhaseID)
	if err != nil {
		return nil, fmt.Errorf("care usage report: phase %d: %w", filters.PhaseID, ErrReportPhaseNotFound)
	}
	requests, err := s.requestRepo.ListAdmin(ctx, enrollmentModels.RequestListFilters{PhaseID: filters.PhaseID})
	if err != nil {
		return nil, fmt.Errorf("care usage report: list requests: %w", err)
	}
	if len(requests) > maxExportRequests {
		return nil, fmt.Errorf("care usage report: %d requests: %w", len(requests), ErrReportExportTooLarge)
	}

	reqIDs := make([]int64, 0, len(requests))
	requestByID := make(map[int64]*enrollmentModels.Request, len(requests))
	for _, req := range requests {
		reqIDs = append(reqIDs, req.ID)
		requestByID[req.ID] = req
	}
	children, err := s.requestChildRepo.ListByRequestIDs(ctx, reqIDs)
	if err != nil {
		return nil, fmt.Errorf("care usage report: list children: %w", err)
	}
	if len(children) > maxReportRows {
		return nil, fmt.Errorf("care usage report: %d children: %w", len(children), ErrReportExportTooLarge)
	}

	childIDs := make([]int64, 0, len(children))
	for _, child := range children {
		childIDs = append(childIDs, child.ID)
	}
	links, err := s.requestChildOfferingRepo.ListByRequestChildIDs(ctx, childIDs)
	if err != nil {
		return nil, fmt.Errorf("care usage report: list child offerings: %w", err)
	}
	offerings, err := s.careOfferingRepo.ListByPhase(ctx, filters.PhaseID)
	if err != nil {
		return nil, fmt.Errorf("care usage report: list offerings: %w", err)
	}

	offeringByID := make(map[int64]*enrollmentModels.CareOffering, len(offerings))
	for _, offering := range offerings {
		offeringByID[offering.ID] = offering
	}
	schemas, err := s.loadCareUsageSchemas(ctx, requests)
	if err != nil {
		return nil, err
	}
	filters.CareOfferingIDs = normalizedCareUsageOfferingIDs(filters.CareOfferingIDs, offerings, filters.CareOfferingIDsSet)
	includedOfferingIDs := makeIDSet(filters.CareOfferingIDs)
	linksByChild := make(map[int64][]*enrollmentModels.RequestChildOffering, len(childIDs))
	for _, link := range links {
		linksByChild[link.RequestChildID] = append(linksByChild[link.RequestChildID], link)
	}

	report := &CareUsageReport{
		Phase:   CareUsagePhase{ID: phase.ID, Name: phase.Name},
		Filters: careUsageAppliedFilters(filters),
		Totals: CareUsageTotals{
			ByDayCount:          initDayCountMap(),
			ByWeekdayPickupTime: initWeekdayPickupTimeMap(),
		},
		FilterOptions: CareUsageFilterOptions{
			Offerings: careUsageOfferingOptions(offerings),
		},
	}
	gradeSeen := map[int16]bool{}
	pickupTimeSeen := map[string]bool{}
	offeringStats := make(map[int64]*CareUsageOfferingStat, len(offerings))

	for _, child := range children {
		req := requestByID[child.RequestID]
		if req == nil {
			continue
		}
		pickupByDay, err := careUsagePickupByDay(req, child, schemas)
		if err != nil {
			return nil, fmt.Errorf("care usage report: child %d pickup schedule: %w", child.ID, err)
		}
		row := careUsageRow(req, child, linksByChild[child.ID], offeringByID, includedOfferingIDs, pickupByDay)
		if child.TargetGradeLevel != nil {
			gradeSeen[*child.TargetGradeLevel] = true
		}
		for _, pickupTime := range row.PickupByDay {
			if pickupTime != "" {
				pickupTimeSeen[pickupTime] = true
			}
		}
		if !careUsageRowMatches(row, filters) {
			continue
		}
		report.Rows = append(report.Rows, row)
		report.Totals.Children++
		report.Totals.ByDayCount[strconv.Itoa(row.DayCount)]++
		for _, day := range row.EffectiveDays {
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
			stat := offeringStats[offering.ID]
			if stat == nil {
				stat = &CareUsageOfferingStat{
					OfferingID:   offering.ID,
					OfferingName: offering.Name,
					ByDayCount:   initDayCountMap(),
				}
				offeringStats[offering.ID] = stat
			}
			stat.Children++
			stat.ByDayCount[strconv.Itoa(row.DayCount)]++
		}
	}

	sort.SliceStable(report.Rows, func(i, j int) bool {
		if report.Rows[i].ChildLastName != report.Rows[j].ChildLastName {
			return strings.ToLower(report.Rows[i].ChildLastName) < strings.ToLower(report.Rows[j].ChildLastName)
		}
		return strings.ToLower(report.Rows[i].ChildFirstName) < strings.ToLower(report.Rows[j].ChildFirstName)
	})
	report.ByOffering = careUsageOfferingStats(offeringStats)
	report.FilterOptions.GradeLevels = careUsageGradeOptions(gradeSeen)
	report.FilterOptions.PickupTimes = sortedPickupTimes(pickupTimeSeen)
	return report, nil
}

func (s *reportService) loadCareUsageSchemas(ctx context.Context, requests []*enrollmentModels.Request) (map[int64]*enrollmentModels.FormSchema, error) {
	schemas := make(map[int64]*enrollmentModels.FormSchema)
	for _, req := range requests {
		if req == nil || req.SchemaID == nil {
			continue
		}
		if _, ok := schemas[*req.SchemaID]; ok {
			continue
		}
		if s.formSchemaRepo == nil {
			return nil, fmt.Errorf("care usage report: form schema repo not configured")
		}
		schema, err := s.formSchemaRepo.FindByID(ctx, *req.SchemaID)
		if err != nil {
			return nil, fmt.Errorf("care usage report: load schema %d: %w", *req.SchemaID, err)
		}
		schemas[*req.SchemaID] = schema
	}
	return schemas, nil
}

func (s *reportService) ExportCareUsage(ctx context.Context, filters CareUsageFilters, actorAccountID int64, actorRole, format string) (*CareUsageReport, error) {
	report, err := s.CareUsage(ctx, filters)
	if err != nil {
		return nil, err
	}
	if err := s.recordCareUsageExportAudit(ctx, report, actorAccountID, actorRole, format); err != nil {
		return nil, err
	}
	return report, nil
}

func normalizeCareUsageFilters(filters CareUsageFilters) CareUsageFilters {
	filters.Status = strings.ToLower(strings.TrimSpace(filters.Status))
	if filters.Status == "" {
		filters.Status = enrollmentModels.ChildStatusApproved
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

func validateCareUsageFilters(filters CareUsageFilters) error {
	if filters.Status != "all" {
		if _, ok := validChildStatusFilters[filters.Status]; !ok {
			return fmt.Errorf("%w: unsupported status %q", ErrReportInvalidFilter, filters.Status)
		}
	}
	if filters.DayCount != nil && (*filters.DayCount < 0 || *filters.DayCount > 7) {
		return fmt.Errorf("%w: day_count must be between 0 and 7", ErrReportInvalidFilter)
	}
	if filters.Weekday != "" && !enrollmentModels.ValidWeekdays[filters.Weekday] {
		return fmt.Errorf("%w: weekday must be one of mon/tue/wed/thu/fri", ErrReportInvalidFilter)
	}
	if filters.PickupTime != "" {
		if _, err := time.Parse("15:04", filters.PickupTime); err != nil {
			return fmt.Errorf("%w: pickup_time must be HH:MM", ErrReportInvalidFilter)
		}
	}
	return nil
}

var validChildStatusFilters = map[string]bool{
	enrollmentModels.ChildStatusSubmitted:          true,
	enrollmentModels.ChildStatusUnderReview:        true,
	enrollmentModels.ChildStatusApproved:           true,
	enrollmentModels.ChildStatusWaitlisted:         true,
	enrollmentModels.ChildStatusRejected:           true,
	enrollmentModels.ChildStatusWithdrawn:          true,
	enrollmentModels.ChildStatusPendingRenewal:     true,
	enrollmentModels.ChildStatusAutoRenewed:        true,
	enrollmentModels.ChildStatusPendingAdminReview: true,
}

func careUsageAppliedFilters(filters CareUsageFilters) CareUsageAppliedFilters {
	return CareUsageAppliedFilters{
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

func dedupePositiveInt64(ids []int64) []int64 {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[int64]bool, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func careUsageRow(req *enrollmentModels.Request, child *enrollmentModels.RequestChild, links []*enrollmentModels.RequestChildOffering, offeringByID map[int64]*enrollmentModels.CareOffering, includedOfferingIDs map[int64]bool, pickupByDayArgs ...map[string]string) CareUsageRow {
	daySet := map[string]bool{}
	rowOfferings := make([]CareUsageRowOffering, 0, len(links))
	for _, link := range links {
		offering := offeringByID[link.CareOfferingID]
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
		days = sortedDayCodes(days)
		if includedOfferingIDs[link.CareOfferingID] {
			for _, day := range days {
				daySet[day] = true
			}
		}
		rowOfferings = append(rowOfferings, CareUsageRowOffering{
			ID:                    link.CareOfferingID,
			Name:                  name,
			Days:                  days,
			DaysSource:            source,
			DaysOfWeekMode:        daysOfWeekMode,
			ManualSelectedDays:    sortedDayCodes(link.ManualSelectedDays),
			AutomaticSelectedDays: sortedDayCodes(link.AutomaticSelectedDays),
		})
	}
	sort.SliceStable(rowOfferings, func(i, j int) bool {
		return rowOfferings[i].Name < rowOfferings[j].Name
	})
	effectiveDays := make([]string, 0, len(daySet))
	for day := range daySet {
		effectiveDays = append(effectiveDays, day)
	}
	effectiveDays = sortedDayCodes(effectiveDays)
	pickupByDay := map[string]string{}
	if len(pickupByDayArgs) > 0 && pickupByDayArgs[0] != nil {
		pickupByDay = pickupByDayArgs[0]
	}
	return CareUsageRow{
		RequestID:         req.ID,
		ChildID:           child.ID,
		ChildFirstName:    child.FirstName,
		ChildLastName:     child.LastName,
		DateOfBirth:       child.DateOfBirth.Format("2006-01-02"),
		TargetGradeLevel:  child.TargetGradeLevel,
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

func careUsagePickupByDay(req *enrollmentModels.Request, child *enrollmentModels.RequestChild, schemas map[int64]*enrollmentModels.FormSchema) (map[string]string, error) {
	out := map[string]string{}
	if req == nil || req.SchemaID == nil || child == nil || child.CustomData == nil {
		return out, nil
	}
	schema := schemas[*req.SchemaID]
	if schema == nil {
		return out, nil
	}
	fields := careUsagePickupScheduleFields(schema)
	for _, field := range fields {
		raw, ok := child.CustomData[field.Key]
		if !ok || raw == nil {
			continue
		}
		schedule, err := decodeCareUsageWeekdaySchedule(raw)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", field.Key, err)
		}
		for day, pickupTime := range schedule {
			pickupTime = strings.TrimSpace(pickupTime)
			if pickupTime == "" {
				continue
			}
			out[day] = pickupTime
		}
	}
	return out, nil
}

func careUsagePickupScheduleFields(schema *enrollmentModels.FormSchema) []enrollmentModels.FormField {
	if schema == nil {
		return nil
	}
	fields := make([]enrollmentModels.FormField, 0)
	for _, field := range schema.Fields {
		if field.Target != enrollmentModels.TargetSchedulePickup ||
			field.Type != enrollmentModels.FormFieldWeekdaySchedule ||
			!field.AppliesToCh {
			continue
		}
		fields = append(fields, field)
	}
	sort.SliceStable(fields, func(i, j int) bool {
		return fields[i].SortOrder < fields[j].SortOrder
	})
	return fields
}

func decodeCareUsageWeekdaySchedule(raw any) (enrollmentModels.WeekdaySchedule, error) {
	var schedule enrollmentModels.WeekdaySchedule
	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(encoded, &schedule); err != nil {
		return nil, err
	}
	if schedule == nil {
		schedule = enrollmentModels.WeekdaySchedule{}
	}
	if err := schedule.Validate(); err != nil {
		return nil, err
	}
	return schedule, nil
}

func careUsageRowMatches(row CareUsageRow, filters CareUsageFilters) bool {
	if filters.Status != "all" && row.Status != filters.Status {
		return false
	}
	if filters.CareOfferingIDsSet && !careUsageRowHasAnyOffering(row, filters.CareOfferingIDs) {
		return false
	}
	if filters.DayCount != nil && row.DayCount != *filters.DayCount {
		return false
	}
	if filters.GradeLevel != nil {
		if row.TargetGradeLevel == nil || *row.TargetGradeLevel != *filters.GradeLevel {
			return false
		}
	}
	if filters.Weekday != "" {
		if !containsString(row.EffectiveDays, filters.Weekday) {
			return false
		}
		if filters.PickupTime != "" {
			return row.PickupByDay[filters.Weekday] == filters.PickupTime
		}
	}
	if filters.PickupTime != "" {
		found := false
		for _, day := range row.EffectiveDays {
			if row.PickupByDay[day] == filters.PickupTime {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if filters.Search != "" {
		haystack := strings.ToLower(strings.Join([]string{
			row.ChildFirstName,
			row.ChildLastName,
			row.GuardianFirstName,
			row.GuardianLastName,
			row.GuardianEmail,
		}, " "))
		if !strings.Contains(haystack, strings.ToLower(filters.Search)) {
			return false
		}
	}
	return true
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func careUsageRowHasAnyOffering(row CareUsageRow, offeringIDs []int64) bool {
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

func careUsageOfferingOptions(offerings []*enrollmentModels.CareOffering) []CareUsageOfferingOption {
	options := make([]CareUsageOfferingOption, 0, len(offerings))
	for _, offering := range offerings {
		options = append(options, CareUsageOfferingOption{
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

func careUsageOfferingStats(stats map[int64]*CareUsageOfferingStat) []CareUsageOfferingStat {
	out := make([]CareUsageOfferingStat, 0, len(stats))
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

func (s *reportService) recordCareUsageExportAudit(ctx context.Context, report *CareUsageReport, actorAccountID int64, actorRole, format string) error {
	if s.dataAccessLogRepo == nil {
		return fmt.Errorf("care usage report export audit: data access log repo not configured")
	}
	if report == nil {
		return fmt.Errorf("care usage report export audit: report required")
	}
	if actorAccountID <= 0 {
		return fmt.Errorf("care usage report export audit: actor account id required")
	}
	if strings.TrimSpace(actorRole) == "" {
		actorRole = "unknown"
	}
	phase, err := s.phaseRepo.FindByID(ctx, report.Phase.ID)
	if err != nil {
		return fmt.Errorf("care usage report export audit: phase %d: %w", report.Phase.ID, err)
	}
	entry := &auditModels.DataAccessLog{
		ActorAccountID: actorAccountID,
		ActorRole:      actorRole,
		ResourceType:   auditModels.ResourceTypeEnrollmentPhaseExport,
		RangeStart:     phase.ServiceStartDate.BerlinMidnight(),
		RangeEnd:       phase.ServiceEndDate.EndOfDay(),
		AccessedAt:     time.Now(),
	}
	entry.SetMetadata("phase_id", report.Phase.ID)
	entry.SetMetadata("report", "care_usage")
	entry.SetMetadata("format", format)
	entry.SetMetadata("status_filter", report.Filters.Status)
	entry.SetMetadata("care_offering_ids", report.Filters.CareOfferingIDs)
	if report.Filters.DayCount != nil {
		entry.SetMetadata("day_count", *report.Filters.DayCount)
	} else {
		entry.SetMetadata("day_count", nil)
	}
	entry.SetMetadata("grade_level", report.Filters.GradeLevel)
	entry.SetMetadata("weekday", report.Filters.Weekday)
	entry.SetMetadata("pickup_time", report.Filters.PickupTime)
	entry.SetMetadata("search", report.Filters.Search)
	entry.SetMetadata("child_count", report.Totals.Children)
	if err := s.dataAccessLogRepo.Create(ctx, entry); err != nil {
		return fmt.Errorf("care usage report export audit write: %w", err)
	}
	return nil
}
