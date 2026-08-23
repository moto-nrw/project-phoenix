package enrollment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/collation"
	"github.com/moto-nrw/project-phoenix/internal/sliceutil"
	"github.com/moto-nrw/project-phoenix/internal/strutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
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
	TargetSchoolClass *string                `json:"target_school_class,omitempty"`
	Status            string                 `json:"status"`
	Offerings         []CareUsageRowOffering `json:"offerings"`
	EffectiveDays     []string               `json:"effective_days"`
	// CareDays is the display-only set of weekdays used by the compact
	// export. Unlike EffectiveDays, it includes every weekday when care is
	// not constrained by an active offering catalog.
	CareDays          []string          `json:"-"`
	DayCount          int               `json:"day_count"`
	PickupByDay       map[string]string `json:"pickup_by_day"`
	GuardianFirstName string            `json:"guardian_first_name"`
	GuardianLastName  string            `json:"guardian_last_name"`
	GuardianEmail     string            `json:"guardian_email"`
	GuardianPhone     *string           `json:"guardian_phone,omitempty"`
	SubmittedAt       time.Time         `json:"submitted_at"`
	// SchedulePickupByDay is the maintained Kind-Gehzeit (manual rows and
	// rolled-out Angebots-Gehzeiten, #2290) of the student linked to this
	// request child, when one exists. Display-only enrichment for the
	// compact export (#2215): filters and totals keep using the
	// enrollment-form snapshot in PickupByDay.
	SchedulePickupByDay map[string]string `json:"schedule_pickup_by_day"`
	// Guardians are all guardian contacts on the request (primary plus
	// additional request guardians), mirroring the class-roster contact
	// column. The flat Guardian* fields above stay the submitting guardian.
	Guardians []ClassRosterGuardian `json:"guardians"`
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
	AllClasses  bool   `json:"all_classes"`
	// OfferingDate pins the offering-link selection to a specific calendar
	// day (#1772 class day view: paging to Monday must show Monday's
	// selection, not today's). Nil keeps the export default
	// reportOfferingDate(phase) — today clamped to the phase window.
	OfferingDate *timezone.Date `json:"offering_date,omitempty"`
	// SkipGuardianData omits the guardian-facing enrichments (emergency
	// contacts, request guardians, companion links). The class day view
	// keeps only Registered/OfferingsByDay/Arrival-/PickupByDay and serves
	// no guardian contact data at all (#1772) — and it rebuilds the roster
	// per covering phase for every class on the teachers' landing page, so
	// these three queries would be pure fan-out waste there. Internal-only,
	// never bound from a request.
	SkipGuardianData bool `json:"-"`
}

// careDate is the day this roster describes. It decides which children still
// belong to the class for care purposes (#2487): the class day view pages
// through the week and passes the day it renders, every other caller means
// today.
func (f ClassRosterFilters) careDate() timezone.Date {
	if f.OfferingDate != nil {
		return *f.OfferingDate
	}
	return timezone.TodayDate()
}

// filterCareRunningOn drops the children whose care at the OGS had already
// ended on that day (#2487). FindBySchoolClass only filters graduates, so
// without this a departed child would keep showing up on the Lehrkraft class
// day sheet and on every Klassenliste export.
func filterCareRunningOn(students []*userModels.Student, day timezone.Date) []*userModels.Student {
	kept := make([]*userModels.Student, 0, len(students))
	for _, student := range students {
		if student == nil || student.CareEndedOn(day) {
			continue
		}
		kept = append(kept, student)
	}
	return kept
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
	AllClasses  bool   `json:"all_classes"`
	Status      string `json:"status"`
}

type ClassRosterTotals struct {
	Students   int `json:"students"`
	Registered int `json:"registered"`
	// ListEntries counts the class-list-only entries (#2382) among Students.
	ListEntries int `json:"list_entries"`
}

// ClassListEntryNoCareLabel marks a class-list-only entry (#2382) on every
// class-list surface: the child has no OGS record at all — deliberately
// distinct from "Keine Anmeldung" (a known OGS child without an enrollment in
// this phase).
const ClassListEntryNoCareLabel = "Keine Betreuung"

type ClassRosterRow struct {
	StudentID         int64  `json:"student_id"`
	FirstName         string `json:"first_name"`
	LastName          string `json:"last_name"`
	SchoolClass       string `json:"school_class"`
	GroupName         string `json:"group_name,omitempty"`
	Registered        bool   `json:"registered"`
	EnrollmentSummary string `json:"enrollment_summary"`
	// ListEntry marks a class-list-only entry (#2382): a child of the class
	// cohort with NO OGS record. StudentID is 0 for these rows; ListEntryID
	// carries the users.class_list_entries id instead, serialized as a JSON
	// string because JavaScript clients round numbers beyond 2^53. The row
	// exists only so the Klassenverband is complete — it can never carry
	// offerings, times or guardians.
	ListEntry      bool                   `json:"list_entry,omitempty"`
	ListEntryID    int64                  `json:"list_entry_id,string,omitempty"`
	Offerings      []CareUsageRowOffering `json:"offerings"`
	OfferingsByDay map[string][]string    `json:"offerings_by_day"`
	CareDays       []string               `json:"care_days"`
	ArrivalByDay   map[string]string      `json:"arrival_by_day"`
	PickupByDay    map[string]string      `json:"pickup_by_day"`
	// SchedulePickupByDay is the maintained Kind-Gehzeit from
	// schedule.student_pickup_schedules (manual rows and rolled-out
	// Angebots-Gehzeiten). It outranks the enrollment-form answer in the
	// weekday cells (#2290).
	SchedulePickupByDay map[string]string `json:"schedule_pickup_by_day"`
	// DepartureByDay carries the compact Geh-/Abholregelung per weekday code
	// ("mon".."fri"), e.g. "wird abgeholt" — the weekday cells print it next
	// to the pickup time (#2254). Every weekday has an entry; a day without an
	// explicit rule carries the business default "geht alleine".
	DepartureByDay map[string]string     `json:"departure_by_day"`
	Guardians      []ClassRosterGuardian `json:"guardians"`
}

type ClassRosterGuardian struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
	Phone string `json:"phone,omitempty"`
}

type ReportService interface {
	CareUsage(ctx context.Context, filters CareUsageFilters) (*CareUsageReport, error)
	ExportCareUsage(ctx context.Context, filters CareUsageFilters, actorAccountID int64, actorRole, format string, compact bool) (*CareUsageReport, error)
	ExportClassRoster(ctx context.Context, filters ClassRosterFilters, actorAccountID int64, actorRole, format string) (*ClassRosterReport, error)
	// ClassDay is the read-only per-class day view for the Lehrkraft handoff
	// (#1772). Every call writes a GDPR access-log row for the actor;
	// actorRole carries the caller's actual roles (comma-joined claims).
	ClassDay(ctx context.Context, schoolClass string, date timezone.Date, actorAccountID int64, actorRole string) (*ClassDayReport, error)
}

type ReportServiceConfig struct {
	RequestRepo              enrollmentModels.RequestRepository
	RequestChildRepo         enrollmentModels.RequestChildRepository
	RequestGuardianRepo      enrollmentModels.RequestGuardianRepository
	RequestChildOfferingRepo enrollmentModels.RequestChildOfferingRepository
	CareOfferingRepo         enrollmentModels.CareOfferingRepository
	FormSchemaRepo           enrollmentModels.FormSchemaRepository
	PhaseRepo                enrollmentModels.PhaseRepository
	DataAccessLogRepo        auditModels.DataAccessLogRepository
	StudentRepo              userModels.StudentRepository
	StudentGuardianRepo      userModels.StudentGuardianRepository
	// StudentCompanionRepo supplies the "läuft mit" links the class roster
	// prints next to an accompanied departure. Optional: an unconfigured repo
	// only costs the names in that one column, never the report.
	StudentCompanionRepo userModels.StudentCompanionRepository
	PersonRepo           userModels.PersonRepository
	EducationGroupRepo   educationModels.GroupRepository
	// StudentStatusDayRepo supplies the scheduled day statuses (sick /
	// excused / class trip) the class day view (#1772) marks students with.
	// REQUIRED for ClassDay: it fails fast ("status/schedule dependencies
	// not configured") rather than serving a sheet where a sick child shows
	// as staying. The enrollment reports and the class roster never consume
	// it, so a config built only for those may leave it nil.
	StudentStatusDayRepo activeModels.StudentStatusDayRepository
	// PickupScheduleSvc / ArrivalScheduleSvc supply the effective per-date
	// times (weekly plan + day exceptions) for the class day view. They are
	// the CURRENT truth — the enrollment form answer is only the snapshot the
	// plan was materialized from. REQUIRED for ClassDay (same fail-fast as
	// StudentStatusDayRepo); no other report path consumes them.
	PickupScheduleSvc  scheduleService.PickupScheduleService
	ArrivalScheduleSvc scheduleService.ArrivalScheduleService
	// ClassListEntryRepo supplies the class-list-only entries (#2382) the
	// class roster and the class day view append to the Klassenverband.
	// Optional: nil (older tests, report paths that never show class lists)
	// simply serves rosters without list entries.
	ClassListEntryRepo userModels.ClassListEntryRepository
	// CareDaySvc owns the "kommt heute / kommt nicht" decision (timeless
	// exception on EITHER leg cancels the day). The class day view consumes
	// it instead of re-deriving the precedence from raw schedule entries —
	// re-implementations are explicitly forbidden (care_day_resolver.go).
	// REQUIRED for ClassDay (same fail-fast); unused by the other reports.
	CareDaySvc scheduleService.CareDayService
	// Settings supplies enrollment.care_offerings_enabled so the class
	// roster matches the form: a leftover active catalog must not constrain
	// pickup times when offerings are turned off. Optional in tests; nil
	// follows the registry default (enabled).
	Settings RequestSettingsResolver
}

type reportService struct {
	ReportServiceConfig
}

func NewReportService(cfg ReportServiceConfig) ReportService {
	return &reportService{ReportServiceConfig: cfg}
}

// BookingViewDate is the reference date for SHOWING a child's booked care
// offerings to a human — today, falling back to the end of the care period
// once that period is over so a finished period still shows its final state.
//
// Deliberately different from reportOfferingDate below, and deliberately
// shared between the parent portal and the staff views (#2185): both must
// answer "what is booked, and what starts later" from the SAME day, or a
// guardian on the phone and the staff member looking at the child see
// different rows. Before the phase starts, that means every booking is
// correctly reported as not yet effective, on both sides.
//
// reportOfferingDate clamps to the phase START as well, because a report
// about a future phase should describe the state that phase will have — a
// report is about the phase, this is about today.
func BookingViewDate(today, serviceEnd timezone.Date) timezone.Date {
	if !serviceEnd.IsZero() && serviceEnd.Before(today) {
		return serviceEnd
	}
	return today
}

// reportOfferingDate selects the current point within a phase. Reports for a
// future phase show the selection that will apply at its start; reports for a
// completed phase show the final selection instead of mixing all intervals.
func reportOfferingDate(phase *enrollmentModels.Phase) timezone.Date {
	today := timezone.TodayDate()
	if phase == nil || (phase.ServiceStartDate.IsZero() && phase.ServiceEndDate.IsZero()) {
		return today
	}
	if !phase.ServiceStartDate.IsZero() && today.Before(phase.ServiceStartDate) {
		return phase.ServiceStartDate
	}
	if !phase.ServiceEndDate.IsZero() && today.After(phase.ServiceEndDate) {
		return phase.ServiceEndDate
	}
	return today
}

func (s *reportService) CareUsage(ctx context.Context, filters CareUsageFilters) (*CareUsageReport, error) {
	return s.careUsage(ctx, filters, false)
}

func (s *reportService) careUsage(ctx context.Context, filters CareUsageFilters, enrichCompact bool) (*CareUsageReport, error) {
	filters = normalizeCareUsageFilters(filters)
	if err := validateCareUsageFilters(filters); err != nil {
		return nil, err
	}
	if filters.PhaseID <= 0 {
		return nil, fmt.Errorf("care usage report: phase_id required")
	}

	phase, err := s.PhaseRepo.FindByID(ctx, filters.PhaseID)
	if err != nil {
		return nil, fmt.Errorf("care usage report: phase %d: %w", filters.PhaseID, ErrReportPhaseNotFound)
	}
	requests, err := s.RequestRepo.ListAdmin(ctx, enrollmentModels.RequestListFilters{PhaseID: filters.PhaseID})
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
	children, err := s.RequestChildRepo.ListByRequestIDs(ctx, reqIDs)
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
	offeringDate := reportOfferingDate(phase)
	links, err := s.RequestChildOfferingRepo.ListByRequestChildIDsAtDate(ctx, childIDs, offeringDate)
	if err != nil {
		return nil, fmt.Errorf("care usage report: list child offerings: %w", err)
	}
	offerings, err := s.CareOfferingRepo.ListByPhase(ctx, filters.PhaseID)
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
		return collation.CompareGermanNames(
			report.Rows[i].ChildLastName, report.Rows[i].ChildFirstName,
			report.Rows[j].ChildLastName, report.Rows[j].ChildFirstName,
		) < 0
	})
	report.ByOffering = careUsageOfferingStats(offeringStats)
	report.FilterOptions.GradeLevels = careUsageGradeOptions(gradeSeen)
	report.FilterOptions.PickupTimes = sortedPickupTimes(pickupTimeSeen)
	if enrichCompact {
		if err := s.enrichCompactCareUsage(ctx, report, children, requestByID, offeringByID, offeringDate); err != nil {
			return nil, err
		}
	}
	return report, nil
}

func (s *reportService) enrichCompactCareUsage(ctx context.Context, report *CareUsageReport, children []*enrollmentModels.RequestChild, requestByID map[int64]*enrollmentModels.Request, offeringByID map[int64]*enrollmentModels.CareOffering, offeringDate timezone.Date) error {
	requestIDs := make([]int64, 0, len(report.Rows))
	childByID := make(map[int64]*enrollmentModels.RequestChild, len(children))
	for _, child := range children {
		childByID[child.ID] = child
	}
	studentIDs := make([]int64, 0, len(report.Rows))
	for _, row := range report.Rows {
		requestIDs = append(requestIDs, row.RequestID)
		if id := careUsageStudentID(childByID[row.ChildID]); id != 0 {
			studentIDs = append(studentIDs, id)
		}
	}

	guardiansByRequest := map[int64][]*enrollmentModels.RequestGuardian{}
	if s.RequestGuardianRepo != nil && len(requestIDs) > 0 {
		guardians, err := s.RequestGuardianRepo.ListByRequestIDs(ctx, requestIDs)
		if err != nil {
			return fmt.Errorf("care usage report: list request guardians: %w", err)
		}
		guardiansByRequest = classRosterRequestGuardiansByRequestID(guardians)
	}
	schedulePickup, err := s.schedulePickupByStudentIDs(ctx, studentIDs, offeringDate)
	if err != nil {
		return fmt.Errorf("care usage report: %w", err)
	}
	careOfferingsEnabled := true
	if s.Settings != nil {
		careOfferingsEnabled, err = s.Settings.ResolveBool(ctx, configModel.KeyEnrollmentCareOfferingsEnabled)
		if err != nil {
			return fmt.Errorf("care usage report: resolve %s: %w", configModel.KeyEnrollmentCareOfferingsEnabled, err)
		}
	}

	for i := range report.Rows {
		row := &report.Rows[i]
		row.CareDays = classRosterCareDays(row.EffectiveDays, offeringByID, careOfferingsEnabled)
		row.Guardians = classRosterEnrollmentGuardians(requestByID[row.RequestID], guardiansByRequest[row.RequestID])
		if byDay := schedulePickup[careUsageStudentID(childByID[row.ChildID])]; byDay != nil {
			row.SchedulePickupByDay = byDay
		}
	}
	return nil
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
		if s.FormSchemaRepo == nil {
			return nil, fmt.Errorf("care usage report: form schema repo not configured")
		}
		schema, err := s.FormSchemaRepo.FindByID(ctx, *req.SchemaID)
		if err != nil {
			return nil, fmt.Errorf("care usage report: load schema %d: %w", *req.SchemaID, err)
		}
		schemas[*req.SchemaID] = schema
	}
	return schemas, nil
}

func (s *reportService) ExportCareUsage(ctx context.Context, filters CareUsageFilters, actorAccountID int64, actorRole, format string, compact bool) (*CareUsageReport, error) {
	report, err := s.careUsage(ctx, filters, compact)
	if err != nil {
		return nil, err
	}
	if err := s.recordCareUsageExportAudit(ctx, report, actorAccountID, actorRole, format, compact); err != nil {
		return nil, err
	}
	return report, nil
}

func (s *reportService) ClassRoster(ctx context.Context, filters ClassRosterFilters) (*ClassRosterReport, error) {
	filters = normalizeClassRosterFilters(filters)
	if err := validateClassRosterFilters(filters); err != nil {
		return nil, err
	}
	if s.StudentRepo == nil {
		return nil, fmt.Errorf("class roster report: student repo not configured")
	}
	students, err := s.classRosterStudents(ctx, filters)
	if err != nil {
		return nil, err
	}
	report, err := s.classRosterForStudents(ctx, filters, students)
	if err != nil {
		return nil, err
	}
	if err := s.appendClassListEntries(ctx, filters, report); err != nil {
		return nil, err
	}
	return report, nil
}

// appendClassListEntries folds the class-list-only entries (#2382) into the
// roster: children of the Klassenverband without any OGS record, marked
// "Keine Betreuung". They sort into their class alphabetically like every
// student row; the AllClasses heading logic keys off the row's class, so a
// class that exists only through entries gets its own section automatically.
func (s *reportService) appendClassListEntries(ctx context.Context, filters ClassRosterFilters, report *ClassRosterReport) error {
	rows, err := s.classListEntryRows(ctx, filters.SchoolClass, filters.AllClasses)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	if len(report.Rows)+len(rows) > maxReportRows {
		return fmt.Errorf("class roster report: %d rows: %w", len(report.Rows)+len(rows), ErrReportExportTooLarge)
	}
	report.Rows = append(report.Rows, rows...)
	sortClassRosterRows(report.Rows)
	report.Totals.Students += len(rows)
	report.Totals.ListEntries += len(rows)
	return nil
}

// classListEntryRows loads the class-list-only entries — one class or all —
// and renders them as roster rows. A nil repo (feature not wired) yields no
// rows.
func (s *reportService) classListEntryRows(ctx context.Context, schoolClass string, allClasses bool) ([]ClassRosterRow, error) {
	if s.ClassListEntryRepo == nil {
		return nil, nil
	}
	var entries []*userModels.ClassListEntry
	var err error
	if allClasses {
		entries, err = s.ClassListEntryRepo.List(ctx, nil)
	} else {
		entries, err = s.ClassListEntryRepo.FindBySchoolClass(ctx, schoolClass)
	}
	if err != nil {
		return nil, fmt.Errorf("class roster report: list class list entries: %w", err)
	}
	rows := make([]ClassRosterRow, 0, len(entries))
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		rows = append(rows, ClassRosterRow{
			ListEntry:         true,
			ListEntryID:       entry.ID,
			FirstName:         entry.FirstName,
			LastName:          entry.LastName,
			SchoolClass:       entry.SchoolClass,
			EnrollmentSummary: ClassListEntryNoCareLabel,
			Offerings:         []CareUsageRowOffering{},
			OfferingsByDay:    map[string][]string{},
			CareDays:          []string{},
			ArrivalByDay:      map[string]string{},
			PickupByDay:       map[string]string{},
			DepartureByDay:    map[string]string{},
			Guardians:         []ClassRosterGuardian{},
		})
	}
	return rows, nil
}

// classRosterForStudents builds the roster for an already-loaded student
// set. Split out for the class day view (#1772): that view merges the
// roster of EVERY covering phase for every class on the teachers' landing
// page, so it loads the class once and reuses it across phases instead of
// re-querying identical students per phase. Callers pass normalized,
// validated filters — the public ClassRoster wrapper owns that.
func (s *reportService) classRosterForStudents(ctx context.Context, filters ClassRosterFilters, students []*userModels.Student) (*ClassRosterReport, error) {
	if s.PersonRepo == nil {
		return nil, fmt.Errorf("class roster report: person repo not configured")
	}
	if s.EducationGroupRepo == nil {
		return nil, fmt.Errorf("class roster report: education group repo not configured")
	}
	if !filters.SkipGuardianData {
		if s.StudentGuardianRepo == nil {
			return nil, fmt.Errorf("class roster report: student guardian repo not configured")
		}
		if s.RequestGuardianRepo == nil {
			return nil, fmt.Errorf("class roster report: request guardian repo not configured")
		}
	}

	phase, err := s.PhaseRepo.FindByID(ctx, filters.PhaseID)
	if err != nil {
		return nil, fmt.Errorf("class roster report: phase %d: %w", filters.PhaseID, ErrReportPhaseNotFound)
	}
	if len(students) > maxReportRows {
		return nil, fmt.Errorf("class roster report: %d students: %w", len(students), ErrReportExportTooLarge)
	}
	studentByID := make(map[int64]*userModels.Student, len(students))
	for _, student := range students {
		if student != nil {
			studentByID[student.ID] = student
		}
	}
	studentIDs := classRosterStudentIDs(students)
	studentGuardianContacts := map[int64][]ClassRosterGuardian{}
	companions := map[int64][]userModels.CompanionLink{}
	if !filters.SkipGuardianData {
		studentGuardianContacts, err = s.classRosterStudentGuardianContacts(ctx, studentIDs)
		if err != nil {
			return nil, err
		}
		companions, err = s.classRosterCompanions(ctx, studentIDs)
		if err != nil {
			return nil, err
		}
	}
	persons, err := s.PersonRepo.FindByIDs(ctx, classRosterPersonIDs(students))
	if err != nil {
		return nil, fmt.Errorf("class roster report: load persons: %w", err)
	}
	groups, err := s.classRosterGroupNames(ctx, students)
	if err != nil {
		return nil, err
	}

	var requests []*enrollmentModels.Request
	var children []*enrollmentModels.RequestChild
	requestGuardiansByID := map[int64][]*enrollmentModels.RequestGuardian{}
	if len(studentIDs) > 0 {
		requests, err = s.RequestRepo.ListAdmin(ctx, enrollmentModels.RequestListFilters{
			PhaseID:           filters.PhaseID,
			CreatedStudentIDs: studentIDs,
		})
		if err != nil {
			return nil, fmt.Errorf("class roster report: list requests: %w", err)
		}
		if len(requests) > maxExportRequests {
			return nil, fmt.Errorf("class roster report: %d requests: %w", len(requests), ErrReportExportTooLarge)
		}
		requestIDs := classRosterRequestIDs(requests)
		children, err = s.RequestChildRepo.ListByRequestIDs(ctx, requestIDs)
		if err != nil {
			return nil, fmt.Errorf("class roster report: list children: %w", err)
		}
		if !filters.SkipGuardianData {
			requestGuardians, err := s.RequestGuardianRepo.ListByRequestIDs(ctx, requestIDs)
			if err != nil {
				return nil, fmt.Errorf("class roster report: list request guardians: %w", err)
			}
			requestGuardiansByID = classRosterRequestGuardiansByRequestID(requestGuardians)
		}
		children = classRosterChildrenForStudents(children, studentByID)
		if len(children) > maxReportRows {
			return nil, fmt.Errorf("class roster report: %d children: %w", len(children), ErrReportExportTooLarge)
		}
	}
	requestByID := classRosterRequestsByID(requests)

	offerings, err := s.CareOfferingRepo.ListByPhase(ctx, filters.PhaseID)
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
	careOfferingsEnabled, err := s.classRosterCareOfferingsEnabled(ctx)
	if err != nil {
		return nil, err
	}

	enrollmentsByStudent, approvedChildIDs := classRosterApprovedEnrollments(children, requestByID, studentByID)
	offeringDate := reportOfferingDate(phase)
	if filters.OfferingDate != nil {
		offeringDate = *filters.OfferingDate
	}
	links, err := s.RequestChildOfferingRepo.ListByRequestChildIDsAtDate(ctx, approvedChildIDs, offeringDate)
	if err != nil {
		return nil, fmt.Errorf("class roster report: list child offerings: %w", err)
	}
	classRosterAttachOfferingLinks(enrollmentsByStudent, links)
	classRosterAttachRequestGuardians(enrollmentsByStudent, requestGuardiansByID)

	schedulePickup, err := s.classRosterSchedulePickupByStudent(ctx, students, offeringDate)
	if err != nil {
		return nil, err
	}
	rows := make([]ClassRosterRow, 0, len(students))
	for _, student := range students {
		if student == nil {
			continue
		}
		person := persons[student.PersonID]
		row, err := classRosterRow(student, person, classRosterGroupName(student, groups), enrollmentsByStudent[student.ID], offeringByID, schemas, studentGuardianContacts[student.ID], companions[student.ID], careOfferingsEnabled)
		if err != nil {
			return nil, err
		}
		if byDay := schedulePickup[student.ID]; byDay != nil {
			row.SchedulePickupByDay = byDay
		}
		rows = append(rows, row)
	}
	sortClassRosterRows(rows)

	report := &ClassRosterReport{
		Phase: CareUsagePhase{ID: phase.ID, Name: phase.Name},
		Filters: ClassRosterAppliedFilters{
			PhaseID:     filters.PhaseID,
			SchoolClass: filters.SchoolClass,
			AllClasses:  filters.AllClasses,
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
	if filters.AllClasses && filters.SchoolClass != "" {
		return fmt.Errorf("%w: school_class and all_classes are mutually exclusive", ErrReportInvalidFilter)
	}
	if !filters.AllClasses && filters.SchoolClass == "" {
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
	out := sliceutil.UniquePositive(ids)
	slices.Sort(out)
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
	effectiveDays := sortedDayCodes(slices.Collect(maps.Keys(daySet)))
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

func careUsageBookedPickupDays(row CareUsageRow, filters CareUsageFilters) []string {
	return careUsageDays(row, filters, func(day string) bool {
		return strings.TrimSpace(row.PickupByDay[day]) != ""
	})
}

func careUsageBookedDays(row CareUsageRow, filters CareUsageFilters) []string {
	return careUsageDays(row, filters, func(string) bool { return true })
}

// careUsageDays collects the booked day codes from the row's (filtered)
// offerings, falling back to the effective days when none match; include
// narrows the set (e.g. to days that carry a pickup time).
func careUsageDays(row CareUsageRow, filters CareUsageFilters, include func(day string) bool) []string {
	daySet := map[string]bool{}
	includedOfferingIDs := makeIDSet(filters.CareOfferingIDs)
	for _, offering := range row.Offerings {
		if filters.CareOfferingIDsSet && !includedOfferingIDs[offering.ID] {
			continue
		}
		for _, day := range offering.Days {
			day = strings.ToLower(strings.TrimSpace(day))
			if day != "" && include(day) {
				daySet[day] = true
			}
		}
	}
	if len(daySet) == 0 {
		for _, day := range row.EffectiveDays {
			day = strings.ToLower(strings.TrimSpace(day))
			if day != "" && include(day) {
				daySet[day] = true
			}
		}
	}
	return sortedDayCodes(slices.Collect(maps.Keys(daySet)))
}

type classRosterApprovedEnrollment struct {
	request   *enrollmentModels.Request
	child     *enrollmentModels.RequestChild
	links     []*enrollmentModels.RequestChildOffering
	guardians []*enrollmentModels.RequestGuardian
}

func classRosterStudentIDs(students []*userModels.Student) []int64 {
	ids := make([]int64, 0, len(students))
	seen := map[int64]bool{}
	for _, student := range students {
		if student == nil || student.ID <= 0 || seen[student.ID] {
			continue
		}
		seen[student.ID] = true
		ids = append(ids, student.ID)
	}
	return ids
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

func classRosterRequestIDs(requests []*enrollmentModels.Request) []int64 {
	ids := make([]int64, 0, len(requests))
	seen := map[int64]bool{}
	for _, req := range requests {
		if req == nil || req.ID <= 0 || seen[req.ID] {
			continue
		}
		seen[req.ID] = true
		ids = append(ids, req.ID)
	}
	return ids
}

func classRosterRequestsByID(requests []*enrollmentModels.Request) map[int64]*enrollmentModels.Request {
	out := make(map[int64]*enrollmentModels.Request, len(requests))
	for _, req := range requests {
		if req != nil {
			out[req.ID] = req
		}
	}
	return out
}

func classRosterRequestGuardiansByRequestID(guardians []*enrollmentModels.RequestGuardian) map[int64][]*enrollmentModels.RequestGuardian {
	out := make(map[int64][]*enrollmentModels.RequestGuardian)
	for _, guardian := range guardians {
		if guardian == nil || guardian.RequestID <= 0 {
			continue
		}
		out[guardian.RequestID] = append(out[guardian.RequestID], guardian)
	}
	return out
}

func classRosterChildrenForStudents(children []*enrollmentModels.RequestChild, studentByID map[int64]*userModels.Student) []*enrollmentModels.RequestChild {
	out := make([]*enrollmentModels.RequestChild, 0, len(children))
	for _, child := range children {
		if child == nil || child.CreatedStudentID == nil {
			continue
		}
		if studentByID[*child.CreatedStudentID] != nil {
			out = append(out, child)
		}
	}
	return out
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
	groups, err := s.EducationGroupRepo.FindByIDs(ctx, groupIDs)
	if err != nil {
		return nil, fmt.Errorf("class roster report: load groups: %w", err)
	}
	return groups, nil
}

// classRosterStudents loads the roster's students: one class, or — for the
// all-classes export — every non-empty class (ListSchoolClasses only returns
// classes that have students, so empty classes never produce empty lists).
func (s *reportService) classRosterStudents(ctx context.Context, filters ClassRosterFilters) ([]*userModels.Student, error) {
	if !filters.AllClasses {
		students, err := s.StudentRepo.FindBySchoolClass(ctx, filters.SchoolClass)
		if err != nil {
			return nil, fmt.Errorf("class roster report: list students: %w", err)
		}
		return filterCareRunningOn(students, filters.careDate()), nil
	}
	classes, err := s.StudentRepo.ListSchoolClasses(ctx)
	if err != nil {
		return nil, fmt.Errorf("class roster report: list school classes: %w", err)
	}
	students := make([]*userModels.Student, 0)
	// ListSchoolClasses is case-sensitive DISTINCT while FindBySchoolClass
	// matches LOWER(TRIM(...)); dedupe with the latter rule so a tenant
	// with both "1a" and "1A" doesn't load the same students twice.
	seen := make(map[string]bool, len(classes))
	for _, class := range classes {
		key := strings.ToLower(strings.TrimSpace(class))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		classStudents, err := s.StudentRepo.FindBySchoolClass(ctx, class)
		if err != nil {
			return nil, fmt.Errorf("class roster report: list students of class %q: %w", class, err)
		}
		students = append(students, filterCareRunningOn(classStudents, filters.careDate())...)
		if len(students) > maxReportRows {
			return nil, fmt.Errorf("class roster report: %d students: %w", len(students), ErrReportExportTooLarge)
		}
	}
	return students, nil
}

func sortClassRosterRows(rows []ClassRosterRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		if r := collation.CompareSchoolClasses(rows[i].SchoolClass, rows[j].SchoolClass); r != 0 {
			return r < 0
		}
		if r := collation.CompareGermanNames(rows[i].LastName, rows[i].FirstName, rows[j].LastName, rows[j].FirstName); r != 0 {
			return r < 0
		}
		if rows[i].StudentID != rows[j].StudentID {
			return rows[i].StudentID < rows[j].StudentID
		}
		return rows[i].ListEntryID < rows[j].ListEntryID
	})
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

func classRosterAttachRequestGuardians(enrollments map[int64]*classRosterApprovedEnrollment, guardiansByRequestID map[int64][]*enrollmentModels.RequestGuardian) {
	for _, enrollment := range enrollments {
		if enrollment == nil || enrollment.request == nil {
			continue
		}
		enrollment.guardians = guardiansByRequestID[enrollment.request.ID]
	}
}

func classRosterRow(
	student *userModels.Student,
	person *userModels.Person,
	groupName string,
	enrollment *classRosterApprovedEnrollment,
	offeringByID map[int64]*enrollmentModels.CareOffering,
	schemas map[int64]*enrollmentModels.FormSchema,
	studentGuardians []ClassRosterGuardian,
	companions []userModels.CompanionLink,
	careOfferingsEnabled bool,
) (ClassRosterRow, error) {
	studentContactGuardians := classRosterStudentGuardians(student, studentGuardians)
	row := ClassRosterRow{
		StudentID:           student.ID,
		SchoolClass:         student.SchoolClass,
		GroupName:           groupName,
		EnrollmentSummary:   "Keine Anmeldung",
		CareDays:            []string{},
		OfferingsByDay:      map[string][]string{},
		ArrivalByDay:        map[string]string{},
		PickupByDay:         map[string]string{},
		SchedulePickupByDay: map[string]string{},
		Guardians:           studentContactGuardians,
	}
	if person != nil {
		row.FirstName = person.FirstName
		row.LastName = person.LastName
	}
	row.DepartureByDay = classRosterDepartureByDayFromStudent(student, companions)
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
	departureByDay, err := classRosterDepartureByDay(enrollment.request, enrollment.child, schemas, student, companions)
	if err != nil {
		return row, fmt.Errorf("class roster report: child %d departure: %w", enrollment.child.ID, err)
	}
	mergedLinks := classRosterMergeOfferingLinks(enrollment.links)
	includedOfferingIDs := classRosterIncludedOfferingIDs(mergedLinks)
	careRow := careUsageRow(enrollment.request, enrollment.child, mergedLinks, offeringByID, includedOfferingIDs, pickupByDay)

	row.Registered = true
	row.EnrollmentSummary = classRosterEnrollmentSummary(careRow.Offerings)
	row.Offerings = careRow.Offerings
	row.OfferingsByDay = classRosterOfferingsByDay(careRow.Offerings)
	row.CareDays = classRosterCareDays(careRow.EffectiveDays, offeringByID, careOfferingsEnabled)
	row.PickupByDay = pickupByDay
	row.ArrivalByDay = arrivalByDay
	row.DepartureByDay = departureByDay
	row.Guardians = classRosterEnrollmentGuardians(enrollment.request, enrollment.guardians)
	if len(row.Guardians) == 0 {
		row.Guardians = studentContactGuardians
	}
	return row, nil
}

// classRosterCareDays is the roster-side mirror of relevantCareDaysForChild:
// booked offering days when the phase has an active catalog (or leftover
// selections), every weekday when it does not. The form only loads offerings
// when enrollment.care_offerings_enabled is on, and only active rows; leftover
// active catalog entries after the setting is turned off must not hide pickup
// times the form treated as unrestricted. EffectiveDays is derived only from
// booked offerings and stays empty in the unconstrained case.
func classRosterCareDays(effectiveDays []string, offeringByID map[int64]*enrollmentModels.CareOffering, careOfferingsEnabled bool) []string {
	if (careOfferingsEnabled && classRosterHasActiveOfferings(offeringByID)) || len(effectiveDays) > 0 {
		return effectiveDays
	}
	return sortedDayCodes([]string{"mon", "tue", "wed", "thu", "fri"})
}

func (s *reportService) classRosterCareOfferingsEnabled(ctx context.Context) (bool, error) {
	if s.Settings == nil {
		return true, nil
	}
	enabled, err := s.Settings.ResolveBool(ctx, configModel.KeyEnrollmentCareOfferingsEnabled)
	if err != nil {
		return false, fmt.Errorf("class roster report: resolve %s: %w", configModel.KeyEnrollmentCareOfferingsEnabled, err)
	}
	return enabled, nil
}

func classRosterHasActiveOfferings(offeringByID map[int64]*enrollmentModels.CareOffering) bool {
	for _, offering := range offeringByID {
		if offering != nil && offering.IsActive {
			return true
		}
	}
	return false
}

// classRosterCompanions loads the "läuft mit" links of every listed child, so
// an accompanied departure names the children it means.
func (s *reportService) classRosterCompanions(ctx context.Context, studentIDs []int64) (map[int64][]userModels.CompanionLink, error) {
	if s.StudentCompanionRepo == nil || len(studentIDs) == 0 {
		return map[int64][]userModels.CompanionLink{}, nil
	}
	links, err := s.StudentCompanionRepo.ListLinksForStudents(ctx, studentIDs)
	if err != nil {
		return nil, fmt.Errorf("class roster report: load departure companions: %w", err)
	}
	return links, nil
}

func (s *reportService) classRosterStudentGuardianContacts(ctx context.Context, studentIDs []int64) (map[int64][]ClassRosterGuardian, error) {
	out := make(map[int64][]ClassRosterGuardian, len(studentIDs))
	if len(studentIDs) == 0 {
		return out, nil
	}
	rows, err := s.StudentGuardianRepo.ListEmergencyContactRows(ctx, studentIDs)
	if err != nil {
		return nil, fmt.Errorf("class roster report: load student guardians: %w", err)
	}
	return classRosterStudentGuardianContactsFromRows(rows), nil
}

func classRosterStudentGuardianContactsFromRows(rows []userModels.GuardianEmergencyContactRow) map[int64][]ClassRosterGuardian {
	type contactAccumulator struct {
		contacts map[string]ClassRosterGuardian
		order    []string
	}
	byStudent := map[int64]*contactAccumulator{}
	for _, row := range rows {
		if row.StudentID <= 0 {
			continue
		}
		name := strings.TrimSpace(row.FirstName.String + " " + row.LastName.String)
		email := strings.TrimSpace(row.Email.String)
		phone := strings.TrimSpace(row.PhoneNumber.String)
		if name == "" && email == "" && phone == "" {
			continue
		}
		key := classRosterStudentGuardianContactKey(row)
		acc := byStudent[row.StudentID]
		if acc == nil {
			acc = &contactAccumulator{contacts: map[string]ClassRosterGuardian{}}
			byStudent[row.StudentID] = acc
		}
		contact, exists := acc.contacts[key]
		if !exists {
			acc.order = append(acc.order, key)
		}
		if contact.Name == "" {
			contact.Name = name
		}
		if contact.Email == "" {
			contact.Email = email
		}
		contact.Phone = strutil.JoinUnique(contact.Phone, phone)
		acc.contacts[key] = contact
	}

	out := make(map[int64][]ClassRosterGuardian, len(byStudent))
	for studentID, acc := range byStudent {
		contacts := make([]ClassRosterGuardian, 0, len(acc.order))
		for _, key := range acc.order {
			contacts = append(contacts, acc.contacts[key])
		}
		out[studentID] = normalizeClassRosterGuardians(contacts)
	}
	return out
}

func classRosterStudentGuardianContactKey(row userModels.GuardianEmergencyContactRow) string {
	if row.GuardianProfileID > 0 {
		return strconv.FormatInt(row.GuardianProfileID, 10)
	}
	return strings.ToLower(strings.TrimSpace(row.FirstName.String + " " + row.LastName.String + "|" + row.Email.String))
}

func classRosterStudentGuardians(student *userModels.Student, linkedContacts []ClassRosterGuardian) []ClassRosterGuardian {
	contacts := normalizeClassRosterGuardians(linkedContacts)
	if len(contacts) > 0 {
		return contacts
	}
	if student != nil {
		contacts = append(contacts, ClassRosterGuardian{
			Name:  stringPtrValue(student.GuardianName),
			Email: stringPtrValue(student.GuardianEmail),
			Phone: strutil.JoinUnique(stringPtrValue(student.GuardianPhone), stringPtrValue(student.GuardianContact)),
		})
	}
	return normalizeClassRosterGuardians(contacts)
}

func classRosterEnrollmentGuardians(req *enrollmentModels.Request, additional []*enrollmentModels.RequestGuardian) []ClassRosterGuardian {
	contacts := []ClassRosterGuardian{}
	if req != nil {
		contacts = append(contacts, ClassRosterGuardian{
			Name:  strings.TrimSpace(req.GuardianFirstName + " " + req.GuardianLastName),
			Email: strings.TrimSpace(req.GuardianEmail),
			Phone: stringPtrValue(req.GuardianPhone),
		})
	}
	for _, guardian := range additional {
		if guardian == nil {
			continue
		}
		contacts = append(contacts, ClassRosterGuardian{
			Name:  strings.TrimSpace(guardian.FirstName + " " + guardian.LastName),
			Email: stringPtrValue(guardian.Email),
			Phone: stringPtrValue(guardian.Phone),
		})
	}
	return normalizeClassRosterGuardians(contacts)
}

func normalizeClassRosterGuardians(contacts []ClassRosterGuardian) []ClassRosterGuardian {
	if len(contacts) == 0 {
		return []ClassRosterGuardian{}
	}
	out := make([]ClassRosterGuardian, 0, len(contacts))
	seen := map[string]bool{}
	for _, contact := range contacts {
		contact.Name = strings.TrimSpace(contact.Name)
		contact.Email = strings.TrimSpace(contact.Email)
		contact.Phone = strings.TrimSpace(contact.Phone)
		if contact.Name == "" && contact.Email == "" && contact.Phone == "" {
			continue
		}
		key := strings.ToLower(contact.Name + "|" + contact.Email + "|" + contact.Phone)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, contact)
	}
	return out
}

func stringPtrValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func classRosterOfferingsByDay(offerings []CareUsageRowOffering) map[string][]string {
	out := map[string][]string{}
	seenByDay := map[string]map[string]bool{}
	for _, offering := range offerings {
		name := strings.TrimSpace(offering.Name)
		if name == "" {
			continue
		}
		for _, day := range offering.Days {
			day = strings.ToLower(strings.TrimSpace(day))
			if day == "" {
				continue
			}
			seen := seenByDay[day]
			if seen == nil {
				seen = map[string]bool{}
				seenByDay[day] = seen
			}
			if seen[name] {
				continue
			}
			seen[name] = true
			out[day] = append(out[day], name)
		}
	}
	for day := range out {
		sort.Strings(out[day])
	}
	return out
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

func classRosterDepartureByDay(req *enrollmentModels.Request, child *enrollmentModels.RequestChild, schemas map[int64]*enrollmentModels.FormSchema, student *userModels.Student, companions []userModels.CompanionLink) (map[string]string, error) {
	allowed, fallback, note, ok, err := classRosterDepartureFromPhase(req, child, schemas)
	if err != nil {
		return nil, err
	}
	if ok {
		// The phase form answered the plan, but the "läuft mit" links belong to
		// the CHILD and are the current, structured answer to "mit wem" either
		// way — the roster prints both sources.
		return classRosterFormatDepartureByDay(allowed, fallback, note, companions), nil
	}
	return classRosterDepartureByDayFromStudent(student, companions), nil
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
		note = strutil.TruncateRunes(note, userModels.MaxDepartureCompanionNoteLen, "")
	}
	return &note
}

func classRosterDepartureByDayFromStudent(student *userModels.Student, companions []userModels.CompanionLink) map[string]string {
	if student == nil {
		return classRosterFormatDepartureByDay(nil, nil, nil, nil)
	}
	return classRosterFormatDepartureByDay(student.AllowedDepartureModes, student.DepartureDays, student.DepartureCompanionNote, companions)
}

// classRosterDayModeLabels phrase the departure modes so they read naturally
// after a pickup time in a weekday cell ("14:30 Uhr, wird abgeholt").
var classRosterDayModeLabels = map[userModels.DepartureMode]string{
	userModels.DepartureAlone:       "geht alleine",
	userModels.DepartureBus:         "fährt Bus",
	userModels.DeparturePickup:      "wird abgeholt",
	userModels.DepartureAccompanied: "mit anderem Kind",
}

// classRosterFormatDepartureByDay renders the plan plus the "mit wem" detail,
// keyed by weekday code — the weekday cells print each day's own rule instead
// of one summarized week column (#2254). Every weekday gets an entry; a day
// without an explicit rule carries the business default "geht alleine".
//
// Both "mit wem" sources travel: a child whose Laufgemeinschaft answers
// "mit wem" needs no free-text note (the note requirement is satisfied per
// weekday by a link), so cells built from the note alone would print
// "mit anderem Kind" with no name on the sheet staff carry to the door.
//
// The links are printed only on the weekdays the rendered plan actually allows
// "Anderes Kind". They are the CURRENT links of the live child, while the plan
// may come from the approved enrollment phase — a plan that says Monday
// accompanied and Tuesday bus, next to a link the child gained on Tuesday
// since, would contradict itself in the Tuesday cell. Filtering is the honest
// reading: a day the printed plan does not allow has no "mit wem" to print.
// The free-text note answers the accompanied days no link covers — a day a
// link already names does not repeat the note (#2254).
func classRosterFormatDepartureByDay(allowed userModels.AllowedDepartureModes, fallback userModels.DepartureDays, companionNote *string, companions []userModels.CompanionLink) map[string]string {
	allowed = allowed.Normalize()
	if !allowed.HasAny() {
		allowed = userModels.AllowedDepartureModesFromDeparture(fallback)
	}
	note := ""
	if companionNote != nil {
		note = strings.TrimSpace(*companionNote)
	}
	out := make(map[string]string, len(userModels.PickupDayOrder))
	for _, day := range userModels.PickupDayOrder {
		modes := allowed[day]
		if len(modes) == 0 {
			out[day] = classRosterDayModeLabels[userModels.DepartureAlone]
			continue
		}
		labels := make([]string, 0, len(modes))
		accompanied := false
		for _, mode := range modes {
			if mode == userModels.DepartureAccompanied {
				accompanied = true
			}
			labels = append(labels, classRosterDayModeLabels[mode])
		}
		cell := strings.Join(labels, " / ")
		if accompanied {
			onDay := userModels.FilterCompanionLinksToDays(companions, map[string]bool{day: true})
			names := make([]string, 0, len(onDay))
			for _, link := range onDay {
				names = append(names, userModels.CompanionDisplayName(link))
			}
			detail := strings.Join(names, ", ")
			if detail == "" {
				detail = note
			}
			if detail != "" {
				cell += " (mit: " + detail + ")"
			}
		}
		out[day] = cell
	}
	return out
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
		bookedDays := careUsageBookedDays(row, filters)
		if !slices.Contains(bookedDays, filters.Weekday) {
			return false
		}
		if filters.PickupTime != "" && row.PickupByDay[filters.Weekday] != filters.PickupTime {
			return false
		}
	}
	if filters.PickupTime != "" {
		found := false
		for _, day := range careUsageBookedPickupDays(row, filters) {
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

func (s *reportService) recordCareUsageExportAudit(ctx context.Context, report *CareUsageReport, actorAccountID int64, actorRole, format string, compact bool) error {
	if s.DataAccessLogRepo == nil {
		return fmt.Errorf("care usage report export audit: data access log repo not configured")
	}
	if report == nil {
		return fmt.Errorf("care usage report export audit: report required")
	}
	phase, err := s.PhaseRepo.FindByID(ctx, report.Phase.ID)
	if err != nil {
		return fmt.Errorf("care usage report export audit: phase %d: %w", report.Phase.ID, err)
	}
	entry, err := exportAuditEntry("care usage report export audit", actorAccountID, actorRole,
		auditModels.ResourceTypeEnrollmentPhaseExport,
		phase.ServiceStartDate.BerlinMidnight(), phase.ServiceEndDate.EndOfDay(), time.Now())
	if err != nil {
		return err
	}
	entry.SetMetadata("phase_id", report.Phase.ID)
	entry.SetMetadata("report", "care_usage")
	entry.SetMetadata("format", format)
	layout := "detailed"
	if compact {
		layout = "compact"
	}
	entry.SetMetadata("layout", layout)
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
	return writeExportAudit(ctx, s.DataAccessLogRepo, entry, "care usage report export audit")
}

func (s *reportService) recordClassRosterExportAudit(ctx context.Context, report *ClassRosterReport, actorAccountID int64, actorRole, format string) error {
	if s.DataAccessLogRepo == nil {
		return fmt.Errorf("class roster report export audit: data access log repo not configured")
	}
	if report == nil {
		return fmt.Errorf("class roster report export audit: report required")
	}
	phase, err := s.PhaseRepo.FindByID(ctx, report.Phase.ID)
	if err != nil {
		return fmt.Errorf("class roster report export audit: phase %d: %w", report.Phase.ID, err)
	}
	entry, err := exportAuditEntry("class roster report export audit", actorAccountID, actorRole,
		auditModels.ResourceTypeEnrollmentPhaseExport,
		phase.ServiceStartDate.BerlinMidnight(), phase.ServiceEndDate.EndOfDay(), time.Now())
	if err != nil {
		return err
	}
	entry.SetMetadata("phase_id", report.Phase.ID)
	entry.SetMetadata("report", "class_roster")
	entry.SetMetadata("format", format)
	schoolClassMeta := report.Filters.SchoolClass
	if report.Filters.AllClasses {
		schoolClassMeta = "alle"
	}
	entry.SetMetadata("school_class", schoolClassMeta)
	entry.SetMetadata("status_filter", report.Filters.Status)
	entry.SetMetadata("student_count", report.Totals.Students)
	entry.SetMetadata("registered_count", report.Totals.Registered)
	return writeExportAudit(ctx, s.DataAccessLogRepo, entry, "class roster report export audit")
}

// classRosterISOWeekdayDay maps pickup-schedule weekdays (ISO, Mon=1) onto
// the report day codes used across the roster maps.
var classRosterISOWeekdayDay = map[int]string{1: "mon", 2: "tue", 3: "wed", 4: "thu", 5: "fri"}

// classRosterSchedulePickupByStudent loads the effective weekly Kind-Gehzeit
// plan on the same date used for the report's offering selection.
// Nil-safe: wirings without the schedule service render without the column.
func (s *reportService) classRosterSchedulePickupByStudent(ctx context.Context, students []*userModels.Student, date timezone.Date) (map[int64]map[string]string, error) {
	studentIDs := make([]int64, 0, len(students))
	for _, student := range students {
		if student != nil {
			studentIDs = append(studentIDs, student.ID)
		}
	}
	out, err := s.schedulePickupByStudentIDs(ctx, studentIDs, date)
	if err != nil {
		return nil, fmt.Errorf("class roster report: %w", err)
	}
	return out, nil
}

// careUsageStudentID resolves the student of an approved request child
// (rollout-created or matched existing student). Unapproved children return
// 0 because their form snapshot must not be replaced by live student data.
func careUsageStudentID(child *enrollmentModels.RequestChild) int64 {
	if child == nil || child.Status != enrollmentModels.ChildStatusApproved {
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

// schedulePickupByStudentIDs maps student ID -> day code -> effective pickup
// time (HH:MM) on date, shared by the class-roster and care-usage reports.
func (s *reportService) schedulePickupByStudentIDs(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]map[string]string, error) {
	out := map[int64]map[string]string{}
	if s.PickupScheduleSvc == nil || len(studentIDs) == 0 {
		return out, nil
	}
	rows, err := s.PickupScheduleSvc.GetWeeklySchedulesByStudentIDsForDate(ctx, studentIDs, date)
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
