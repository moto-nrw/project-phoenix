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
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
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

type ClassRosterFilters struct {
	PhaseID     int64  `json:"phase_id"`
	SchoolClass string `json:"school_class"`
}

type ClassRosterReport struct {
	Phase   CareUsagePhase            `json:"phase"`
	Filters ClassRosterAppliedFilters `json:"filters"`
	Totals  ClassRosterTotals         `json:"totals"`
	Rows    []ClassRosterRow          `json:"rows"`
}

type ClassRosterAppliedFilters struct {
	PhaseID     int64  `json:"phase_id"`
	SchoolClass string `json:"school_class"`
	Status      string `json:"status"`
}

type ClassRosterTotals struct {
	Students   int `json:"students"`
	Registered int `json:"registered"`
}

type ClassRosterRow struct {
	StudentID         int64                  `json:"student_id"`
	FirstName         string                 `json:"first_name"`
	LastName          string                 `json:"last_name"`
	SchoolClass       string                 `json:"school_class"`
	GroupName         string                 `json:"group_name,omitempty"`
	Registered        bool                   `json:"registered"`
	EnrollmentSummary string                 `json:"enrollment_summary"`
	Offerings         []CareUsageRowOffering `json:"offerings"`
	CareDays          []string               `json:"care_days"`
	ArrivalByDay      map[string]string      `json:"arrival_by_day"`
	PickupByDay       map[string]string      `json:"pickup_by_day"`
	Departure         string                 `json:"departure"`
}

type ReportService interface {
	CareUsage(ctx context.Context, filters CareUsageFilters) (*CareUsageReport, error)
	ExportCareUsage(ctx context.Context, filters CareUsageFilters, actorAccountID int64, actorRole, format string) (*CareUsageReport, error)
	ClassRoster(ctx context.Context, filters ClassRosterFilters) (*ClassRosterReport, error)
	ExportClassRoster(ctx context.Context, filters ClassRosterFilters, actorAccountID int64, actorRole, format string) (*ClassRosterReport, error)
}

type ReportServiceConfig struct {
	RequestRepo              enrollmentModels.RequestRepository
	RequestChildRepo         enrollmentModels.RequestChildRepository
	RequestChildOfferingRepo enrollmentModels.RequestChildOfferingRepository
	CareOfferingRepo         enrollmentModels.CareOfferingRepository
	FormSchemaRepo           enrollmentModels.FormSchemaRepository
	PhaseRepo                enrollmentModels.PhaseRepository
	DataAccessLogRepo        auditModels.DataAccessLogRepository
	StudentRepo              userModels.StudentRepository
	PersonRepo               userModels.PersonRepository
	EducationGroupRepo       educationModels.GroupRepository
}

type reportService struct {
	requestRepo              enrollmentModels.RequestRepository
	requestChildRepo         enrollmentModels.RequestChildRepository
	requestChildOfferingRepo enrollmentModels.RequestChildOfferingRepository
	careOfferingRepo         enrollmentModels.CareOfferingRepository
	formSchemaRepo           enrollmentModels.FormSchemaRepository
	phaseRepo                enrollmentModels.PhaseRepository
	dataAccessLogRepo        auditModels.DataAccessLogRepository
	studentRepo              userModels.StudentRepository
	personRepo               userModels.PersonRepository
	educationGroupRepo       educationModels.GroupRepository
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
		studentRepo:              cfg.StudentRepo,
		personRepo:               cfg.PersonRepo,
		educationGroupRepo:       cfg.EducationGroupRepo,
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

func (s *reportService) ClassRoster(ctx context.Context, filters ClassRosterFilters) (*ClassRosterReport, error) {
	filters = normalizeClassRosterFilters(filters)
	if err := validateClassRosterFilters(filters); err != nil {
		return nil, err
	}
	if s.studentRepo == nil {
		return nil, fmt.Errorf("class roster report: student repo not configured")
	}
	if s.personRepo == nil {
		return nil, fmt.Errorf("class roster report: person repo not configured")
	}
	if s.educationGroupRepo == nil {
		return nil, fmt.Errorf("class roster report: education group repo not configured")
	}

	phase, err := s.phaseRepo.FindByID(ctx, filters.PhaseID)
	if err != nil {
		return nil, fmt.Errorf("class roster report: phase %d: %w", filters.PhaseID, ErrReportPhaseNotFound)
	}
	students, err := s.studentRepo.FindBySchoolClass(ctx, filters.SchoolClass)
	if err != nil {
		return nil, fmt.Errorf("class roster report: list students: %w", err)
	}
	if len(students) > maxReportRows {
		return nil, fmt.Errorf("class roster report: %d students: %w", len(students), ErrReportExportTooLarge)
	}
	persons, err := s.personRepo.FindByIDs(ctx, classRosterPersonIDs(students))
	if err != nil {
		return nil, fmt.Errorf("class roster report: load persons: %w", err)
	}
	groups, err := s.classRosterGroupNames(ctx, students)
	if err != nil {
		return nil, err
	}

	requests, err := s.requestRepo.ListAdmin(ctx, enrollmentModels.RequestListFilters{PhaseID: filters.PhaseID})
	if err != nil {
		return nil, fmt.Errorf("class roster report: list requests: %w", err)
	}
	if len(requests) > maxExportRequests {
		return nil, fmt.Errorf("class roster report: %d requests: %w", len(requests), ErrReportExportTooLarge)
	}
	requestByID := make(map[int64]*enrollmentModels.Request, len(requests))
	reqIDs := make([]int64, 0, len(requests))
	for _, req := range requests {
		if req == nil {
			continue
		}
		requestByID[req.ID] = req
		reqIDs = append(reqIDs, req.ID)
	}
	children, err := s.requestChildRepo.ListByRequestIDs(ctx, reqIDs)
	if err != nil {
		return nil, fmt.Errorf("class roster report: list children: %w", err)
	}
	if len(children) > maxReportRows {
		return nil, fmt.Errorf("class roster report: %d children: %w", len(children), ErrReportExportTooLarge)
	}

	offerings, err := s.careOfferingRepo.ListByPhase(ctx, filters.PhaseID)
	if err != nil {
		return nil, fmt.Errorf("class roster report: list offerings: %w", err)
	}
	offeringByID := make(map[int64]*enrollmentModels.CareOffering, len(offerings))
	for _, offering := range offerings {
		if offering != nil {
			offeringByID[offering.ID] = offering
		}
	}
	schemas, err := s.loadCareUsageSchemas(ctx, requests)
	if err != nil {
		return nil, err
	}

	studentByID := make(map[int64]*userModels.Student, len(students))
	for _, student := range students {
		if student != nil {
			studentByID[student.ID] = student
		}
	}
	enrollmentsByStudent, approvedChildIDs := classRosterApprovedEnrollments(children, requestByID, studentByID)
	links, err := s.requestChildOfferingRepo.ListByRequestChildIDs(ctx, approvedChildIDs)
	if err != nil {
		return nil, fmt.Errorf("class roster report: list child offerings: %w", err)
	}
	classRosterAttachOfferingLinks(enrollmentsByStudent, links)

	rows := make([]ClassRosterRow, 0, len(students))
	for _, student := range students {
		if student == nil {
			continue
		}
		person := persons[student.PersonID]
		row, err := classRosterRow(student, person, classRosterGroupName(student, groups), enrollmentsByStudent[student.ID], offeringByID, schemas)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		lastNameCompare := strings.Compare(strings.ToLower(rows[i].LastName), strings.ToLower(rows[j].LastName))
		if lastNameCompare != 0 {
			return lastNameCompare < 0
		}
		firstNameCompare := strings.Compare(strings.ToLower(rows[i].FirstName), strings.ToLower(rows[j].FirstName))
		if firstNameCompare != 0 {
			return firstNameCompare < 0
		}
		return rows[i].StudentID < rows[j].StudentID
	})

	report := &ClassRosterReport{
		Phase: CareUsagePhase{ID: phase.ID, Name: phase.Name},
		Filters: ClassRosterAppliedFilters{
			PhaseID:     filters.PhaseID,
			SchoolClass: filters.SchoolClass,
			Status:      enrollmentModels.ChildStatusApproved,
		},
		Totals: ClassRosterTotals{Students: len(rows)},
		Rows:   rows,
	}
	for _, row := range rows {
		if row.Registered {
			report.Totals.Registered++
		}
	}
	return report, nil
}

func (s *reportService) ExportClassRoster(ctx context.Context, filters ClassRosterFilters, actorAccountID int64, actorRole, format string) (*ClassRosterReport, error) {
	report, err := s.ClassRoster(ctx, filters)
	if err != nil {
		return nil, err
	}
	if err := s.recordClassRosterExportAudit(ctx, report, actorAccountID, actorRole, format); err != nil {
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

func normalizeClassRosterFilters(filters ClassRosterFilters) ClassRosterFilters {
	filters.SchoolClass = strings.TrimSpace(filters.SchoolClass)
	return filters
}

func validateClassRosterFilters(filters ClassRosterFilters) error {
	if filters.PhaseID <= 0 {
		return fmt.Errorf("%w: phase_id is required", ErrReportInvalidFilter)
	}
	if filters.SchoolClass == "" {
		return fmt.Errorf("%w: school_class is required", ErrReportInvalidFilter)
	}
	return nil
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

type classRosterApprovedEnrollment struct {
	request *enrollmentModels.Request
	child   *enrollmentModels.RequestChild
	links   []*enrollmentModels.RequestChildOffering
}

func classRosterPersonIDs(students []*userModels.Student) []int64 {
	ids := make([]int64, 0, len(students))
	seen := map[int64]bool{}
	for _, student := range students {
		if student == nil || student.PersonID <= 0 || seen[student.PersonID] {
			continue
		}
		seen[student.PersonID] = true
		ids = append(ids, student.PersonID)
	}
	return ids
}

func classRosterGroupIDs(students []*userModels.Student) []int64 {
	ids := make([]int64, 0, len(students))
	seen := map[int64]bool{}
	for _, student := range students {
		if student == nil || student.GroupID == nil || *student.GroupID <= 0 || seen[*student.GroupID] {
			continue
		}
		seen[*student.GroupID] = true
		ids = append(ids, *student.GroupID)
	}
	return ids
}

func (s *reportService) classRosterGroupNames(ctx context.Context, students []*userModels.Student) (map[int64]*educationModels.Group, error) {
	groupIDs := classRosterGroupIDs(students)
	if len(groupIDs) == 0 {
		return nil, nil
	}
	groups, err := s.educationGroupRepo.FindByIDs(ctx, groupIDs)
	if err != nil {
		return nil, fmt.Errorf("class roster report: load groups: %w", err)
	}
	return groups, nil
}

func classRosterGroupName(student *userModels.Student, groups map[int64]*educationModels.Group) string {
	if student == nil || student.GroupID == nil || groups == nil {
		return ""
	}
	group := groups[*student.GroupID]
	if group == nil {
		return ""
	}
	return group.Name
}

func classRosterApprovedEnrollments(
	children []*enrollmentModels.RequestChild,
	requestByID map[int64]*enrollmentModels.Request,
	studentByID map[int64]*userModels.Student,
) (map[int64]*classRosterApprovedEnrollment, []int64) {
	out := make(map[int64]*classRosterApprovedEnrollment)
	childIDs := make([]int64, 0, len(children))
	for _, child := range children {
		if child == nil ||
			child.Status != enrollmentModels.ChildStatusApproved ||
			child.CreatedStudentID == nil ||
			*child.CreatedStudentID <= 0 {
			continue
		}
		studentID := *child.CreatedStudentID
		if studentByID[studentID] == nil {
			continue
		}
		req := requestByID[child.RequestID]
		if req == nil {
			continue
		}
		current := out[studentID]
		if current == nil {
			out[studentID] = &classRosterApprovedEnrollment{request: req, child: child}
		} else if classRosterChildIsNewer(req, child, current.request, current.child) {
			current.request = req
			current.child = child
		}
	}
	for _, enrollment := range out {
		if enrollment != nil && enrollment.child != nil && enrollment.child.ID > 0 {
			childIDs = append(childIDs, enrollment.child.ID)
		}
	}
	sort.Slice(childIDs, func(i, j int) bool { return childIDs[i] < childIDs[j] })
	return out, childIDs
}

func classRosterChildIsNewer(candidateReq *enrollmentModels.Request, candidateChild *enrollmentModels.RequestChild, currentReq *enrollmentModels.Request, currentChild *enrollmentModels.RequestChild) bool {
	if currentReq == nil {
		return true
	}
	if candidateReq == nil {
		return false
	}
	if !candidateReq.SubmittedAt.Equal(currentReq.SubmittedAt) {
		return candidateReq.SubmittedAt.After(currentReq.SubmittedAt)
	}
	if currentChild == nil {
		return true
	}
	if candidateChild == nil {
		return false
	}
	return candidateChild.ID > currentChild.ID
}

func classRosterAttachOfferingLinks(enrollments map[int64]*classRosterApprovedEnrollment, links []*enrollmentModels.RequestChildOffering) {
	studentIDByChildID := make(map[int64]int64)
	for studentID, enrollment := range enrollments {
		if enrollment == nil || enrollment.child == nil || enrollment.child.ID <= 0 {
			continue
		}
		studentIDByChildID[enrollment.child.ID] = studentID
	}
	for _, link := range links {
		if link == nil {
			continue
		}
		studentID, ok := studentIDByChildID[link.RequestChildID]
		if !ok {
			continue
		}
		enrollments[studentID].links = append(enrollments[studentID].links, link)
	}
}

func classRosterRow(
	student *userModels.Student,
	person *userModels.Person,
	groupName string,
	enrollment *classRosterApprovedEnrollment,
	offeringByID map[int64]*enrollmentModels.CareOffering,
	schemas map[int64]*enrollmentModels.FormSchema,
) (ClassRosterRow, error) {
	row := ClassRosterRow{
		StudentID:         student.ID,
		SchoolClass:       student.SchoolClass,
		GroupName:         groupName,
		EnrollmentSummary: "Keine Anmeldung",
		CareDays:          []string{},
		ArrivalByDay:      map[string]string{},
		PickupByDay:       map[string]string{},
	}
	if person != nil {
		row.FirstName = person.FirstName
		row.LastName = person.LastName
	}
	row.Departure = classRosterDepartureFromStudent(student)
	if enrollment == nil || enrollment.request == nil || enrollment.child == nil {
		return row, nil
	}

	pickupByDay, err := careUsagePickupByDay(enrollment.request, enrollment.child, schemas)
	if err != nil {
		return row, fmt.Errorf("class roster report: child %d pickup schedule: %w", enrollment.child.ID, err)
	}
	arrivalByDay, err := careUsageScheduleByTarget(enrollment.request, enrollment.child, schemas, enrollmentModels.TargetScheduleArrival)
	if err != nil {
		return row, fmt.Errorf("class roster report: child %d arrival schedule: %w", enrollment.child.ID, err)
	}
	departure, err := classRosterDeparture(enrollment.request, enrollment.child, schemas, student)
	if err != nil {
		return row, fmt.Errorf("class roster report: child %d departure: %w", enrollment.child.ID, err)
	}
	mergedLinks := classRosterMergeOfferingLinks(enrollment.links)
	includedOfferingIDs := classRosterIncludedOfferingIDs(mergedLinks)
	careRow := careUsageRow(enrollment.request, enrollment.child, mergedLinks, offeringByID, includedOfferingIDs, pickupByDay)

	row.Registered = true
	row.EnrollmentSummary = classRosterEnrollmentSummary(careRow.Offerings)
	row.Offerings = careRow.Offerings
	row.CareDays = careRow.EffectiveDays
	row.PickupByDay = pickupByDay
	row.ArrivalByDay = arrivalByDay
	row.Departure = departure
	return row, nil
}

func classRosterMergeOfferingLinks(links []*enrollmentModels.RequestChildOffering) []*enrollmentModels.RequestChildOffering {
	if len(links) == 0 {
		return nil
	}
	byID := make(map[int64]*enrollmentModels.RequestChildOffering, len(links))
	for _, link := range links {
		if link == nil || link.CareOfferingID <= 0 {
			continue
		}
		merged := byID[link.CareOfferingID]
		if merged == nil {
			copy := *link
			copy.SelectedDays = sortedDayCodes(copy.SelectedDays)
			copy.ManualSelectedDays = sortedDayCodes(copy.ManualSelectedDays)
			copy.AutomaticSelectedDays = sortedDayCodes(copy.AutomaticSelectedDays)
			byID[link.CareOfferingID] = &copy
			continue
		}
		merged.SelectedDays = mergeDayCodes(merged.SelectedDays, link.SelectedDays)
		merged.ManualSelectedDays = mergeDayCodes(merged.ManualSelectedDays, link.ManualSelectedDays)
		merged.AutomaticSelectedDays = mergeDayCodes(merged.AutomaticSelectedDays, link.AutomaticSelectedDays)
	}
	out := make([]*enrollmentModels.RequestChildOffering, 0, len(byID))
	for _, link := range byID {
		out = append(out, link)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CareOfferingID < out[j].CareOfferingID })
	return out
}

func mergeDayCodes(a, b []string) []string {
	combined := make([]string, 0, len(a)+len(b))
	combined = append(combined, a...)
	combined = append(combined, b...)
	return sortedDayCodes(combined)
}

func classRosterIncludedOfferingIDs(links []*enrollmentModels.RequestChildOffering) map[int64]bool {
	out := make(map[int64]bool, len(links))
	for _, link := range links {
		if link != nil && link.CareOfferingID > 0 {
			out[link.CareOfferingID] = true
		}
	}
	return out
}

func classRosterEnrollmentSummary(offerings []CareUsageRowOffering) string {
	names := make([]string, 0, len(offerings))
	seen := map[string]bool{}
	for _, offering := range offerings {
		name := strings.TrimSpace(offering.Name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) == 0 {
		return "Angemeldet"
	}
	return "Angemeldet: " + strings.Join(names, ", ")
}

func classRosterDeparture(req *enrollmentModels.Request, child *enrollmentModels.RequestChild, schemas map[int64]*enrollmentModels.FormSchema, student *userModels.Student) (string, error) {
	allowed, fallback, note, ok, err := classRosterDepartureFromPhase(req, child, schemas)
	if err != nil {
		return "", err
	}
	if ok {
		return classRosterFormatDeparture(allowed, fallback, note), nil
	}
	return classRosterDepartureFromStudent(student), nil
}

func classRosterDepartureFromPhase(req *enrollmentModels.Request, child *enrollmentModels.RequestChild, schemas map[int64]*enrollmentModels.FormSchema) (userModels.AllowedDepartureModes, userModels.DepartureDays, *string, bool, error) {
	if req == nil || req.SchemaID == nil || child == nil {
		return nil, nil, nil, false, nil
	}
	schema := schemas[*req.SchemaID]
	if schema == nil {
		return nil, nil, nil, false, nil
	}
	var explicitAllowed *userModels.AllowedDepartureModes
	var explicitDeparture *userModels.DepartureDays
	legacyBus := userModels.BusDays{}
	legacyPickup := userModels.PickupDays{}
	hasLegacy := false

	for _, field := range classRosterDepartureFields(schema) {
		raw := classRosterFieldValue(req, child, field)
		if raw == nil {
			continue
		}
		switch field.Target {
		case enrollmentModels.TargetStudentAllowedDepartureModes:
			modes, err := decodeAllowedDepartureModes(raw)
			if err != nil {
				return nil, nil, nil, false, fmt.Errorf("%s: %w", field.Key, err)
			}
			if explicitAllowed != nil {
				modes = (*explicitAllowed).Merge(modes)
			}
			explicitAllowed = &modes
		case enrollmentModels.TargetStudentDeparture:
			days, err := decodeDepartureDays(raw)
			if err != nil {
				return nil, nil, nil, false, fmt.Errorf("%s: %w", field.Key, err)
			}
			if explicitDeparture != nil {
				days = (*explicitDeparture).Merge(days)
			}
			explicitDeparture = &days
		case enrollmentModels.TargetStudentBusDays, enrollmentModels.TargetStudentBus:
			days, err := decodeBusDays(raw)
			if err != nil {
				return nil, nil, nil, false, fmt.Errorf("%s: %w", field.Key, err)
			}
			legacyBus = days
			hasLegacy = true
		case enrollmentModels.TargetStudentPickupStatus:
			days, err := decodePickupDays(raw)
			if err != nil {
				return nil, nil, nil, false, fmt.Errorf("%s: %w", field.Key, err)
			}
			legacyPickup = days
			hasLegacy = true
		}
	}

	note := classRosterCompanionNote(child)
	if explicitAllowed != nil {
		allowed := explicitAllowed.Normalize()
		return allowed, allowed.DepartureDays(), note, true, nil
	}
	if explicitDeparture != nil {
		departure := explicitDeparture.Normalize()
		return userModels.AllowedDepartureModesFromDeparture(departure), departure, note, true, nil
	}
	if hasLegacy {
		allowed := userModels.AllowedDepartureModesFromLegacy(legacyBus, legacyPickup)
		return allowed, allowed.DepartureDays(), note, true, nil
	}
	return nil, nil, note, false, nil
}

func classRosterDepartureFields(schema *enrollmentModels.FormSchema) []enrollmentModels.FormField {
	fields := make([]enrollmentModels.FormField, 0)
	if schema == nil {
		return fields
	}
	for _, field := range schema.Fields {
		if !field.AppliesToCh {
			continue
		}
		switch field.Target {
		case enrollmentModels.TargetStudentAllowedDepartureModes,
			enrollmentModels.TargetStudentDeparture,
			enrollmentModels.TargetStudentBusDays,
			enrollmentModels.TargetStudentBus,
			enrollmentModels.TargetStudentPickupStatus:
			fields = append(fields, field)
		}
	}
	sort.SliceStable(fields, func(i, j int) bool {
		return fields[i].SortOrder < fields[j].SortOrder
	})
	return fields
}

func classRosterFieldValue(req *enrollmentModels.Request, child *enrollmentModels.RequestChild, field enrollmentModels.FormField) any {
	if field.AppliesToCh {
		if child == nil || child.CustomData == nil {
			return nil
		}
		return child.CustomData[field.Key]
	}
	if req == nil || req.CustomData == nil {
		return nil
	}
	return req.CustomData[field.Key]
}

func classRosterCompanionNote(child *enrollmentModels.RequestChild) *string {
	if child == nil || child.CustomData == nil {
		return nil
	}
	note := strings.TrimSpace(stringValue(child.CustomData[enrollmentModels.TargetStudentDepartureCompanionNote]))
	if note == "" {
		return nil
	}
	if len([]rune(note)) > userModels.MaxDepartureCompanionNoteLen {
		note = truncateRunes(note, userModels.MaxDepartureCompanionNoteLen)
	}
	return &note
}

func classRosterDepartureFromStudent(student *userModels.Student) string {
	if student == nil {
		return "Geht alleine"
	}
	return classRosterFormatDeparture(student.AllowedDepartureModes, student.DepartureDays, student.DepartureCompanionNote)
}

func classRosterFormatDeparture(allowed userModels.AllowedDepartureModes, fallback userModels.DepartureDays, companionNote *string) string {
	modeLabels := map[userModels.DepartureMode]string{
		userModels.DepartureAlone:       "zu Fuß",
		userModels.DepartureBus:         "Bus",
		userModels.DeparturePickup:      "Abholung",
		userModels.DepartureAccompanied: "Mit anderem Kind",
	}
	shortDay := map[string]string{
		userModels.PickupDayMonday:    "Mo",
		userModels.PickupDayTuesday:   "Di",
		userModels.PickupDayWednesday: "Mi",
		userModels.PickupDayThursday:  "Do",
		userModels.PickupDayFriday:    "Fr",
	}
	allowed = allowed.Normalize()
	if !allowed.HasAny() {
		allowed = userModels.AllowedDepartureModesFromDeparture(fallback)
	}
	parts := make([]string, 0, len(userModels.PickupDayOrder))
	for _, day := range userModels.PickupDayOrder {
		modes := allowed[day]
		if len(modes) == 0 {
			continue
		}
		labels := make([]string, 0, len(modes))
		for _, mode := range modes {
			labels = append(labels, modeLabels[mode])
		}
		parts = append(parts, shortDay[day]+": "+strings.Join(labels, ", "))
	}
	summary := "Geht alleine"
	if len(parts) > 0 {
		summary = strings.Join(parts, ", ")
	}
	if companionNote == nil || strings.TrimSpace(*companionNote) == "" || !allowed.HasMode(userModels.DepartureAccompanied) {
		return summary
	}
	return summary + " (mit: " + strings.TrimSpace(*companionNote) + ")"
}

func careUsagePickupByDay(req *enrollmentModels.Request, child *enrollmentModels.RequestChild, schemas map[int64]*enrollmentModels.FormSchema) (map[string]string, error) {
	return careUsageScheduleByTarget(req, child, schemas, enrollmentModels.TargetSchedulePickup)
}

func careUsageScheduleByTarget(req *enrollmentModels.Request, child *enrollmentModels.RequestChild, schemas map[int64]*enrollmentModels.FormSchema, target string) (map[string]string, error) {
	out := map[string]string{}
	if req == nil || req.SchemaID == nil || child == nil {
		return out, nil
	}
	schema := schemas[*req.SchemaID]
	if schema == nil {
		return out, nil
	}
	fields := careUsageScheduleFields(schema, target)
	for _, field := range fields {
		raw := classRosterFieldValue(req, child, field)
		if raw == nil {
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

func careUsageScheduleFields(schema *enrollmentModels.FormSchema, target string) []enrollmentModels.FormField {
	if schema == nil {
		return nil
	}
	fields := make([]enrollmentModels.FormField, 0)
	for _, field := range schema.Fields {
		if field.Target != target ||
			field.Type != enrollmentModels.FormFieldWeekdaySchedule {
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
		if filters.PickupTime != "" && row.PickupByDay[filters.Weekday] != filters.PickupTime {
			return false
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

func (s *reportService) recordClassRosterExportAudit(ctx context.Context, report *ClassRosterReport, actorAccountID int64, actorRole, format string) error {
	if s.dataAccessLogRepo == nil {
		return fmt.Errorf("class roster report export audit: data access log repo not configured")
	}
	if report == nil {
		return fmt.Errorf("class roster report export audit: report required")
	}
	if actorAccountID <= 0 {
		return fmt.Errorf("class roster report export audit: actor account id required")
	}
	if strings.TrimSpace(actorRole) == "" {
		actorRole = "unknown"
	}
	phase, err := s.phaseRepo.FindByID(ctx, report.Phase.ID)
	if err != nil {
		return fmt.Errorf("class roster report export audit: phase %d: %w", report.Phase.ID, err)
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
	entry.SetMetadata("report", "class_roster")
	entry.SetMetadata("format", format)
	entry.SetMetadata("school_class", report.Filters.SchoolClass)
	entry.SetMetadata("status_filter", report.Filters.Status)
	entry.SetMetadata("student_count", report.Totals.Students)
	entry.SetMetadata("registered_count", report.Totals.Registered)
	if err := s.dataAccessLogRepo.Create(ctx, entry); err != nil {
		return fmt.Errorf("class roster report export audit write: %w", err)
	}
	return nil
}
