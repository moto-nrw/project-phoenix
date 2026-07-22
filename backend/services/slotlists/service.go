package slotlists

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	educationModel "github.com/moto-nrw/project-phoenix/models/education"
	facilitiesModel "github.com/moto-nrw/project-phoenix/models/facilities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	userModel "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

// confidentialityNote matches the wording of the other printed exports.
const confidentialityNote = "Vertraulich, nur für berechtigte Personen. Nach Gebrauch sicher vernichten."

const (
	defaultShortDayCutoff = "14:30"
	defaultLongDayCutoff  = "16:00"
)

const timeLayout = "15:04"

// Service builds slot lists for preview (JSON) and export (PDF/XLSX).
type Service interface {
	BuildList(ctx context.Context, params Params) (*Result, error)
	ListOptions(ctx context.Context, date timezone.Date) (*OptionsResult, error)
	RenderList(ctx context.Context, params Params, format listexport.Format) (listexport.File, error)
}

type instanceReader interface {
	FindByTenantAndDate(ctx context.Context, date timezone.Date) ([]*scheduleModel.ActivityInstance, error)
}

type instanceStudentReader interface {
	FindByInstanceID(ctx context.Context, instanceID int64) ([]*scheduleModel.InstanceStudent, error)
}

type visitReader interface {
	FindByActiveGroupID(ctx context.Context, activeGroupID int64) ([]*activeModel.Visit, error)
}

type attendanceReader interface {
	FindForDate(ctx context.Context, date timezone.Date) ([]*activeModel.Attendance, error)
}

type studentReader interface {
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*userModel.Student, error)
	List(ctx context.Context, filters map[string]interface{}) ([]*userModel.Student, error)
}

type personReader interface {
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*userModel.Person, error)
}

type educationGroupReader interface {
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*educationModel.Group, error)
}

type roomReader interface {
	FindByID(ctx context.Context, id any) (*facilitiesModel.Room, error)
}

type pickupTimeReader interface {
	GetBulkEffectivePickupTimesForDate(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]*scheduleSvc.EffectivePickupTime, error)
}

type settingsReader interface {
	HasTenantOverride(ctx context.Context, key string) (bool, error)
	ResolveString(ctx context.Context, key string) (string, error)
}

// Dependencies wires the narrow read interfaces the service needs.
type Dependencies struct {
	InstanceRepo        instanceReader
	InstanceStudentRepo instanceStudentReader
	VisitRepo           visitReader
	AttendanceRepo      attendanceReader
	StudentRepo         studentReader
	PersonRepo          personReader
	EducationGroupRepo  educationGroupReader
	RoomRepo            roomReader
	PickupService       pickupTimeReader
	ListExport          *listexport.RendererService
	Settings            settingsReader
	UserContext         authorize.StudentReadUserContext
	Logger              *slog.Logger
}

type service struct {
	instanceRepo        instanceReader
	instanceStudentRepo instanceStudentReader
	visitRepo           visitReader
	attendanceRepo      attendanceReader
	studentRepo         studentReader
	personRepo          personReader
	educationGroupRepo  educationGroupReader
	roomRepo            roomReader
	pickupService       pickupTimeReader
	listExport          *listexport.RendererService
	settings            settingsReader
	userContext         authorize.StudentReadUserContext
	logger              *slog.Logger
}

// NewService creates a slot list service.
func NewService(deps Dependencies) Service {
	return &service{
		instanceRepo:        deps.InstanceRepo,
		instanceStudentRepo: deps.InstanceStudentRepo,
		visitRepo:           deps.VisitRepo,
		attendanceRepo:      deps.AttendanceRepo,
		studentRepo:         deps.StudentRepo,
		personRepo:          deps.PersonRepo,
		educationGroupRepo:  deps.EducationGroupRepo,
		roomRepo:            deps.RoomRepo,
		pickupService:       deps.PickupService,
		listExport:          deps.ListExport,
		settings:            deps.Settings,
		userContext:         deps.UserContext,
		logger:              deps.Logger,
	}
}

type pickupBucketConfig struct {
	ShortCutoff string
	LongCutoff  string
}

func (s *service) pickupBuckets(ctx context.Context) (pickupBucketConfig, error) {
	cfg := pickupBucketConfig{ShortCutoff: defaultShortDayCutoff, LongCutoff: defaultLongDayCutoff}
	if s.settings == nil {
		return cfg, nil
	}
	short, err := s.settings.ResolveString(ctx, configModel.KeySlotListShortDayCutoff)
	if err != nil {
		return cfg, fmt.Errorf("resolve short-day pickup cutoff: %w", err)
	}
	if short != "" {
		normalized, err := normalizeHHMM(short)
		if err != nil {
			return cfg, fmt.Errorf("invalid short-day pickup cutoff %q", short)
		}
		cfg.ShortCutoff = normalized
	}

	long, err := s.settings.ResolveString(ctx, configModel.KeySlotListLongDayCutoff)
	if err != nil {
		return cfg, fmt.Errorf("resolve long-day pickup cutoff: %w", err)
	}
	if long != "" {
		normalized, err := normalizeHHMM(long)
		if err != nil {
			return cfg, fmt.Errorf("invalid long-day pickup cutoff %q", long)
		}
		cfg.LongCutoff = normalized
	}
	if cfg.LongCutoff <= cfg.ShortCutoff {
		return cfg, fmt.Errorf("long-day pickup cutoff %s must be after short-day pickup cutoff %s", cfg.LongCutoff, cfg.ShortCutoff)
	}
	return cfg, nil
}

type studentReadAccess struct {
	unrestricted bool
	groupIDs     map[int64]struct{}
}

func (a studentReadAccess) canReadGroup(groupID *int64) bool {
	if a.unrestricted {
		return true
	}
	if groupID == nil || a.groupIDs == nil {
		return false
	}
	_, ok := a.groupIDs[*groupID]
	return ok
}

func (s *service) resolveStudentReadAccess(ctx context.Context) studentReadAccess {
	if authorize.HasPermission("admin:*", jwt.PermissionsFromCtx(ctx)) {
		return studentReadAccess{unrestricted: true}
	}
	if s.userContext == nil {
		return studentReadAccess{groupIDs: map[int64]struct{}{}}
	}

	staff, err := s.userContext.GetCurrentStaff(ctx)
	if err != nil || staff == nil {
		return studentReadAccess{groupIDs: map[int64]struct{}{}}
	}

	scope := s.resolveStringSettingOrDefault(
		ctx,
		configModel.KeyStudentDataScope,
		configModel.StudentDataScopeGroupSupervisorsOnly,
	)
	if scope == configModel.StudentDataScopeAllStaff {
		return studentReadAccess{unrestricted: true}
	}

	educationGroups, err := s.userContext.GetMyGroups(ctx)
	if err != nil {
		return studentReadAccess{groupIDs: map[int64]struct{}{}}
	}
	groupIDs := make(map[int64]struct{}, len(educationGroups))
	for _, group := range educationGroups {
		groupIDs[group.ID] = struct{}{}
	}
	return studentReadAccess{groupIDs: groupIDs}
}

func (s *service) resolveStringSettingOrDefault(ctx context.Context, key, fallback string) string {
	if s.settings == nil {
		return fallback
	}
	has, err := s.settings.HasTenantOverride(ctx, key)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("settings override check failed, using fallback",
				slog.String("key", key),
				slog.String("error", err.Error()),
			)
		}
		return fallback
	}
	if !has {
		return fallback
	}
	val, err := s.settings.ResolveString(ctx, key)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("settings resolve failed, using fallback",
				slog.String("key", key),
				slog.String("error", err.Error()),
			)
		}
		return fallback
	}
	if val == "" {
		return fallback
	}
	return val
}

func normalizeHHMM(value string) (string, error) {
	parsed, err := time.Parse(timeLayout, strings.TrimSpace(value))
	if err != nil {
		return "", err
	}
	return parsed.Format(timeLayout), nil
}

func (s *service) listLabel(ctx context.Context, params Params) (string, error) {
	if params.Target == TargetSlots {
		if params.ListKind != ListKindNone {
			return params.ListKind.Label(), nil
		}
		if len(params.InstanceIDs) == 1 {
			return "Ausgewähltes Angebot", nil
		}
		return "Freie Angebotsauswahl", nil
	}
	cfg, err := s.pickupBuckets(ctx)
	if err != nil {
		return "", err
	}
	switch params.PickupCohort {
	case PickupCohortShortDay:
		return "Ganztag bis " + cfg.ShortCutoff, nil
	case PickupCohortLongDay:
		return "Ganztag bis " + cfg.LongCutoff, nil
	default:
		return params.Target.Label(), nil
	}
}

func (s *service) provenance(params Params, listLabel string) string {
	plan := "Geplante Kinder laut Tagesplanung"
	if params.Target == TargetSlots && params.ListKind != ListKindNone {
		plan = "Geplante Kinder laut Tagesplanung (" + listLabel + ")"
	}
	if params.Target == TargetPickupCohort {
		plan = "Geplante Kinder laut Tagesplanung und Abholzeiten (" + listLabel + ")"
	}
	switch params.Source {
	case SourcePlanned:
		return plan
	case SourceActual:
		return "Dokumentierte Anwesenheit am Datum"
	case SourceReconciliation:
		return plan + " mit dokumentierter Anwesenheit abgeglichen"
	default:
		return ""
	}
}

func (s *service) BuildList(ctx context.Context, params Params) (*Result, error) {
	if s.instanceRepo == nil || s.instanceStudentRepo == nil || s.visitRepo == nil ||
		s.attendanceRepo == nil || s.studentRepo == nil || s.personRepo == nil ||
		s.educationGroupRepo == nil || s.roomRepo == nil ||
		s.pickupService == nil || s.listExport == nil {
		return nil, fmt.Errorf("slot list service is not configured")
	}
	if !params.Target.Valid() {
		return nil, fmt.Errorf("unknown list target %q", params.Target)
	}
	if !params.Source.Valid() {
		return nil, fmt.Errorf("unknown source %q", params.Source)
	}
	if params.Target == TargetPickupCohort && !params.PickupCohort.Valid() {
		return nil, fmt.Errorf("unknown pickup cohort %q", params.PickupCohort)
	}
	if !params.ListKind.Valid() {
		return nil, fmt.Errorf("unknown list kind %q", params.ListKind)
	}
	if params.Target != TargetSlots && params.ListKind != ListKindNone {
		return nil, fmt.Errorf("list kind %q is not valid for target %q", params.ListKind, params.Target)
	}
	if !params.GroupBy.ValidFor(params.Target) {
		return nil, fmt.Errorf("grouping %q is not valid for target %q", params.GroupBy, params.Target)
	}
	access := s.resolveStudentReadAccess(ctx)

	listLabel, err := s.listLabel(ctx, params)
	if err != nil {
		return nil, err
	}

	result := &Result{
		Date:         params.Date.String(),
		Target:       params.Target,
		PickupCohort: params.PickupCohort,
		ListKind:     params.ListKind,
		ListLabel:    listLabel,
		Source:       params.Source,
		GroupBy:      params.GroupBy,
		Provenance:   s.provenance(params, listLabel),
		Slots:        []Slot{},
		Rows:         []Row{},
	}

	var entries []mergedEntry
	if params.Target == TargetPickupCohort {
		entries, err = s.collectPickupEntries(ctx, params, result)
	} else {
		entries, err = s.collectSlotEntries(ctx, params, result)
	}
	if err != nil {
		return nil, err
	}

	// Full (unfiltered) rows for the requested source — used both to derive the
	// available group/class options and as the input to the row filters.
	allRows, err := s.enrichEntries(ctx, entries, params.Source, access)
	if err != nil {
		return nil, err
	}
	result.Groups, result.Classes = availableOptions(allRows)

	rows := filterRows(allRows, params)
	applyGrouping(rows, params.GroupBy)
	sort.SliceStable(rows, func(i, j int) bool {
		// Keep grouped rows contiguous: section heading first, then the same
		// slot/name order within each section.
		if rows[i].GroupTitle != rows[j].GroupTitle {
			return rows[i].GroupTitle < rows[j].GroupTitle
		}
		if rows[i].Slot != rows[j].Slot {
			return rows[i].Slot < rows[j].Slot
		}
		return rows[i].Name < rows[j].Name
	})
	result.Rows = rows
	result.Counters = countRows(rows, params.Source)
	return result, nil
}

func (s *service) ListOptions(ctx context.Context, date timezone.Date) (*OptionsResult, error) {
	slotResult, err := s.BuildList(ctx, Params{
		Date:   date,
		Target: TargetSlots,
		Source: SourcePlanned,
	})
	if err != nil {
		return nil, err
	}
	cohorts := make([]PickupCohortOption, 0, 2)
	for _, cohort := range []PickupCohort{PickupCohortShortDay, PickupCohortLongDay} {
		result, err := s.BuildList(ctx, Params{
			Date:         date,
			Target:       TargetPickupCohort,
			PickupCohort: cohort,
			Source:       SourcePlanned,
		})
		if err != nil {
			return nil, err
		}
		rowCount := len(result.Rows)
		cohorts = append(cohorts, PickupCohortOption{
			Cohort:    cohort,
			Label:     result.ListLabel,
			Available: rowCount > 0,
			RowCount:  rowCount,
		})
	}
	listKinds := make([]ListKindOption, 0, len(AllListKinds))
	for _, kind := range AllListKinds {
		result, err := s.BuildList(ctx, Params{
			Date:     date,
			Target:   TargetSlots,
			ListKind: kind,
			Source:   SourcePlanned,
		})
		if err != nil {
			return nil, err
		}
		slotCount := selectableSlotCount(result.Slots)
		listKinds = append(listKinds, ListKindOption{
			Kind:      kind,
			Label:     kind.Label(),
			Available: slotCount > 0,
			SlotCount: slotCount,
			RowCount:  len(result.Rows),
		})
	}
	return &OptionsResult{Date: date.String(), Slots: slotResult.Slots, PickupCohorts: cohorts, ListKinds: listKinds}, nil
}

// applyGrouping stamps each row's GroupTitle from the chosen dimension. With
// GroupByNone every title stays empty (a flat list). Rows whose group value is
// empty fall into a generic "Ohne …" bucket so they never silently vanish.
func applyGrouping(rows []Row, groupBy GroupBy) {
	if groupBy == GroupByNone {
		return
	}
	for i := range rows {
		var value string
		switch groupBy {
		case GroupBySlot:
			value = rows[i].Slot
		case GroupByRoom:
			value = rows[i].RoomName
		case GroupByClass:
			value = rows[i].SchoolClass
		case GroupByPickupTime:
			value = rows[i].PickupTime
		}
		if value == "" {
			value = "Ohne " + groupBy.Label()
		}
		rows[i].GroupTitle = groupBy.Label() + ": " + value
	}
}

// availableOptions derives the distinct education groups and school classes
// present in the unfiltered rows, sorted for stable UI dropdowns.
func availableOptions(rows []Row) ([]GroupOption, []string) {
	groupByID := map[int64]string{}
	classSet := map[string]struct{}{}
	for _, row := range rows {
		if row.GroupID != nil && row.GroupName != "" {
			groupByID[*row.GroupID] = row.GroupName
		}
		if row.SchoolClass != "" {
			classSet[row.SchoolClass] = struct{}{}
		}
	}
	groups := make([]GroupOption, 0, len(groupByID))
	for id, name := range groupByID {
		groups = append(groups, GroupOption{ID: id, Name: name})
	}
	sort.SliceStable(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
	classes := make([]string, 0, len(classSet))
	for class := range classSet {
		classes = append(classes, class)
	}
	sort.Strings(classes)
	return groups, classes
}

// filterRows applies the offering/group/class filters (AND). An empty filter
// slice means "no restriction" on that dimension.
func filterRows(rows []Row, params Params) []Row {
	if params.Target == TargetSlots && params.InstanceIDsSet && len(params.InstanceIDs) == 0 {
		return []Row{}
	}
	instanceSet := int64Set(params.InstanceIDs)
	groupSet := int64Set(params.GroupIDs)
	classSet := make(map[string]struct{}, len(params.Classes))
	for _, c := range params.Classes {
		classSet[c] = struct{}{}
	}

	out := make([]Row, 0, len(rows))
	for _, row := range rows {
		if len(instanceSet) > 0 && !instanceSet[row.InstanceID] {
			continue
		}
		if len(groupSet) > 0 && (row.GroupID == nil || !groupSet[*row.GroupID]) {
			continue
		}
		if len(classSet) > 0 {
			if _, ok := classSet[row.SchoolClass]; !ok {
				continue
			}
		}
		out = append(out, row)
	}
	return out
}

func int64Set(ids []int64) map[int64]bool {
	set := make(map[int64]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}

// mergedEntry is one (slot, student) pair before person/group enrichment.
type mergedEntry struct {
	StudentID        int64
	InstanceID       int64 // 0 for pickup-based cohorts
	SlotLabel        string
	RoomName         string // planned room of the slot; "" for pickup cohorts
	PickupTime       string
	Planned          bool
	Present          bool
	PlannedStatus    string  // schedule.instance_students.status, "" for pickup cohorts
	PlannedSubstatus *string // schedule.instance_students.substatus, nil for pickup cohorts
}

// collectSlotEntries derives the cohort from materialized activity instances
// (Plan) and their bridged active-group visits (Ist). A visit counts as present
// when it overlaps the slot's Berlin-local time window, so historical
// checked-out visits remain valid evidence for past lists. Cancelled slots are
// returned as selectable context but do not produce rows.
func (s *service) collectSlotEntries(ctx context.Context, params Params, result *Result) ([]mergedEntry, error) {
	instances, err := s.instanceRepo.FindByTenantAndDate(ctx, params.Date)
	if err != nil {
		return nil, err
	}

	roomCache := map[int64]string{}
	entries := []mergedEntry{}
	for _, inst := range instances {
		if !instanceMatchesListKind(inst, params.ListKind) {
			continue
		}
		slotLabel := fmt.Sprintf("%s (%s–%s)", inst.Title, inst.StartTime.Format(timeLayout), inst.EndTime.Format(timeLayout))
		roomName, err := s.lookupRoomName(ctx, inst.RoomID, roomCache)
		if err != nil {
			return nil, err
		}
		result.Slots = append(result.Slots, Slot{
			InstanceID: inst.ID,
			Title:      inst.Title,
			TimeRange:  fmt.Sprintf("%s–%s", inst.StartTime.Format(timeLayout), inst.EndTime.Format(timeLayout)),
			Status:     inst.Status,
		})
		if inst.Status == scheduleModel.InstanceStatusCancelled {
			continue
		}

		planned, err := s.instanceStudentRepo.FindByInstanceID(ctx, inst.ID)
		if err != nil {
			return nil, err
		}
		presentSet, err := s.loadPresentStudents(ctx, inst)
		if err != nil {
			return nil, err
		}

		seenPlanned := make(map[int64]struct{}, len(planned))
		for _, row := range planned {
			present := presentSet[row.StudentID] || row.Status == scheduleModel.AttendanceStatusPresent
			switch {
			case row.IsUnplanned:
				// #1913: the row was created by an observed walk-in visit,
				// not by planning. It is durable presence evidence, never a
				// plan entry — otherwise the Abgleich would relabel the
				// walk-in as "geplant & anwesend".
				seenPlanned[row.StudentID] = struct{}{}
				if present {
					entries = append(entries, mergedEntry{
						StudentID:  row.StudentID,
						InstanceID: inst.ID,
						SlotLabel:  slotLabel,
						RoomName:   roomName,
						Present:    true,
					})
				}
			case row.NotScheduled && row.Status == scheduleModel.AttendanceStatusExpected:
				// #1747 non-booking marker: the care plan did not place the
				// child in the OGS on this date. Not a genuine expectation —
				// skip, and leave the student unseen so a live visit can
				// still surface them as unplanned below.
				continue
			case row.NotScheduled:
				// Unbooked day the system decided anyway (checked in, or an
				// absence written by a status day). The child was never
				// planned for this slot; presence shows as unplanned, an
				// absence on an unbooked day shows nowhere.
				seenPlanned[row.StudentID] = struct{}{}
				if present {
					entries = append(entries, mergedEntry{
						StudentID:  row.StudentID,
						InstanceID: inst.ID,
						SlotLabel:  slotLabel,
						RoomName:   roomName,
						Present:    true,
					})
				}
			default:
				seenPlanned[row.StudentID] = struct{}{}
				entries = append(entries, mergedEntry{
					StudentID:        row.StudentID,
					InstanceID:       inst.ID,
					SlotLabel:        slotLabel,
					RoomName:         roomName,
					Planned:          true,
					Present:          present,
					PlannedStatus:    row.Status,
					PlannedSubstatus: row.Substatus,
				})
			}
		}
		for studentID, present := range presentSet {
			if !present {
				continue
			}
			if _, ok := seenPlanned[studentID]; ok {
				continue
			}
			entries = append(entries, mergedEntry{
				StudentID:  studentID,
				InstanceID: inst.ID,
				SlotLabel:  slotLabel,
				RoomName:   roomName,
				Present:    true,
			})
		}
	}
	return entries, nil
}

func instanceMatchesListKind(inst *scheduleModel.ActivityInstance, listKind ListKind) bool {
	if listKind == ListKindNone {
		return true
	}
	if inst == nil || inst.ListKind == nil {
		return false
	}
	return *inst.ListKind == string(listKind)
}

// lookupRoomName resolves a room's display name, cached per build. A missing
// room is non-fatal (room grouping just shows the "Ohne Raum" bucket).
func (s *service) lookupRoomName(ctx context.Context, roomID int64, cache map[int64]string) (string, error) {
	if roomID == 0 {
		return "", nil
	}
	if name, ok := cache[roomID]; ok {
		return name, nil
	}
	room, err := s.roomRepo.FindByID(ctx, roomID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			cache[roomID] = ""
			return "", nil
		}
		return "", fmt.Errorf("lookup room %d: %w", roomID, err)
	}
	if room == nil {
		cache[roomID] = ""
		return "", nil
	}
	cache[roomID] = room.Name
	return room.Name, nil
}

// loadPresentStudents returns the set of students with a visit that overlaps
// the instance's Berlin-local slot window. A slot that has not started has no
// active group and therefore no presence evidence.
func (s *service) loadPresentStudents(ctx context.Context, inst *scheduleModel.ActivityInstance) (map[int64]bool, error) {
	present := map[int64]bool{}
	if inst.ActiveGroupID == nil {
		return present, nil
	}
	visits, err := s.visitRepo.FindByActiveGroupID(ctx, *inst.ActiveGroupID)
	if err != nil {
		return nil, err
	}
	for _, visit := range visits {
		if visitOverlapsInstance(visit, inst) {
			present[visit.StudentID] = true
		}
	}
	return present, nil
}

func visitOverlapsInstance(visit *activeModel.Visit, inst *scheduleModel.ActivityInstance) bool {
	if visit == nil || inst == nil {
		return false
	}
	start, end := instanceTimeRange(inst)
	if !visit.EntryTime.Before(end) {
		return false
	}
	return visit.ExitTime == nil || visit.ExitTime.After(start)
}

func instanceTimeRange(inst *scheduleModel.ActivityInstance) (time.Time, time.Time) {
	start := time.Date(
		inst.Date.Year, inst.Date.Month, inst.Date.Day,
		inst.StartTime.Hour(), inst.StartTime.Minute(), inst.StartTime.Second(), inst.StartTime.Nanosecond(),
		timezone.Berlin,
	)
	end := time.Date(
		inst.Date.Year, inst.Date.Month, inst.Date.Day,
		inst.EndTime.Hour(), inst.EndTime.Minute(), inst.EndTime.Second(), inst.EndTime.Nanosecond(),
		timezone.Berlin,
	)
	return start, end
}

// collectPickupEntries derives the cohort from effective pickup times
// (weekly schedule + exceptions) and presence from attendance records on the
// selected date. Closed attendance rows still count for historical lists.
func (s *service) collectPickupEntries(ctx context.Context, params Params, result *Result) ([]mergedEntry, error) {
	buckets, err := s.pickupBuckets(ctx)
	if err != nil {
		return nil, err
	}
	students, err := s.studentRepo.List(ctx, map[string]interface{}{})
	if err != nil {
		return nil, err
	}
	studentIDs := make([]int64, 0, len(students))
	for _, student := range students {
		studentIDs = append(studentIDs, student.ID)
	}

	pickupTimes := map[int64]*scheduleSvc.EffectivePickupTime{}
	if len(studentIDs) > 0 {
		pickupTimes, err = s.pickupService.GetBulkEffectivePickupTimesForDate(ctx, studentIDs, params.Date)
		if err != nil {
			return nil, err
		}
	}

	attendanceRows, err := s.attendanceRepo.FindForDate(ctx, params.Date)
	if err != nil {
		return nil, err
	}
	presentSet := make(map[int64]struct{}, len(attendanceRows))
	for _, row := range attendanceRows {
		presentSet[row.StudentID] = struct{}{}
	}

	slotLabel := result.ListLabel
	entries := []mergedEntry{}
	cohort := map[int64]struct{}{}
	for _, student := range students {
		pickup := pickupTimes[student.ID]
		if pickup == nil || pickup.PickupTime == nil {
			continue
		}
		hhmm := pickup.PickupTime.Format(timeLayout)
		if !pickupMatchesCohort(params.PickupCohort, hhmm, buckets) {
			continue
		}
		cohort[student.ID] = struct{}{}
		_, present := presentSet[student.ID]
		entries = append(entries, mergedEntry{
			StudentID:  student.ID,
			SlotLabel:  slotLabel,
			PickupTime: hhmm,
			Planned:    true,
			Present:    present,
		})
	}

	// Children who attended on this date but whose effective pickup time puts
	// them outside this cohort are "unplanned" — but only meaningful in the
	// reconciliation merge. A pickup list has no physical room, so in the pure
	// Ist view "every other present child in the school" is noise, not a member
	// of this list. Plan/Ist therefore stay scoped to the cohort.
	if params.Source == SourceReconciliation {
		for id := range presentSet {
			if _, ok := cohort[id]; ok {
				continue
			}
			pickupLabel := ""
			if pickup := pickupTimes[id]; pickup != nil && pickup.PickupTime != nil {
				pickupLabel = pickup.PickupTime.Format(timeLayout)
			}
			entries = append(entries, mergedEntry{
				StudentID:  id,
				SlotLabel:  slotLabel,
				PickupTime: pickupLabel,
				Present:    true,
			})
		}
	}
	return entries, nil
}

func pickupMatchesCohort(cohort PickupCohort, hhmm string, buckets pickupBucketConfig) bool {
	switch cohort {
	case PickupCohortShortDay:
		return hhmm <= buckets.ShortCutoff
	case PickupCohortLongDay:
		return hhmm > buckets.ShortCutoff && hhmm <= buckets.LongCutoff
	default:
		return false
	}
}

// enrichEntries filters by source/read access, resolves names/classes/groups,
// and maps to display rows.
func (s *service) enrichEntries(ctx context.Context, entries []mergedEntry, source Source, access studentReadAccess) ([]Row, error) {
	filtered := make([]mergedEntry, 0, len(entries))
	for _, entry := range entries {
		switch source {
		case SourcePlanned:
			if entry.Planned {
				filtered = append(filtered, entry)
			}
		case SourceActual:
			if entry.Present {
				filtered = append(filtered, entry)
			}
		case SourceReconciliation:
			filtered = append(filtered, entry)
		}
	}
	if len(filtered) == 0 {
		return []Row{}, nil
	}

	studentIDSet := map[int64]struct{}{}
	studentIDs := []int64{}
	for _, entry := range filtered {
		if _, ok := studentIDSet[entry.StudentID]; !ok {
			studentIDSet[entry.StudentID] = struct{}{}
			studentIDs = append(studentIDs, entry.StudentID)
		}
	}
	students, err := s.studentRepo.FindByIDs(ctx, studentIDs)
	if err != nil {
		return nil, err
	}

	readableStudents := make(map[int64]*userModel.Student, len(students))
	personIDs := make([]int64, 0, len(students))
	groupIDSet := map[int64]struct{}{}
	groupIDs := []int64{}
	for _, entry := range filtered {
		student := students[entry.StudentID]
		if student == nil || !access.canReadGroup(student.GroupID) {
			continue
		}
		if _, ok := readableStudents[student.ID]; ok {
			continue
		}
		readableStudents[student.ID] = student
		personIDs = append(personIDs, student.PersonID)
		if student.GroupID != nil {
			if _, ok := groupIDSet[*student.GroupID]; !ok {
				groupIDSet[*student.GroupID] = struct{}{}
				groupIDs = append(groupIDs, *student.GroupID)
			}
		}
	}
	if len(readableStudents) == 0 {
		return []Row{}, nil
	}
	persons, err := s.personRepo.FindByIDs(ctx, personIDs)
	if err != nil {
		return nil, err
	}
	groups := map[int64]*educationModel.Group{}
	if len(groupIDs) > 0 {
		groups, err = s.educationGroupRepo.FindByIDs(ctx, groupIDs)
		if err != nil {
			return nil, err
		}
	}

	rows := make([]Row, 0, len(filtered))
	for _, entry := range filtered {
		student := readableStudents[entry.StudentID]
		if student == nil {
			continue
		}
		person := persons[student.PersonID]
		if person == nil {
			continue
		}
		groupName := ""
		if student.GroupID != nil {
			if group := groups[*student.GroupID]; group != nil {
				groupName = group.Name
			}
		}
		rows = append(rows, Row{
			StudentID:   entry.StudentID,
			Name:        person.GetFullName(),
			SchoolClass: student.SchoolClass,
			GroupName:   groupName,
			GroupID:     student.GroupID,
			InstanceID:  entry.InstanceID,
			Slot:        entry.SlotLabel,
			RoomName:    entry.RoomName,
			PickupTime:  entry.PickupTime,
			Planned:     entry.Planned,
			Present:     entry.Present,
			Unplanned:   entry.Present && !entry.Planned,
			Excused:     entry.Planned && !entry.Present && signedOffAbsence(entry),
			StatusLabel: statusLabel(entry, source),
		})
	}
	return rows, nil
}

func signedOffAbsence(entry mergedEntry) bool {
	return entry.PlannedStatus == scheduleModel.AttendanceStatusAbsent &&
		entry.PlannedSubstatus != nil &&
		*entry.PlannedSubstatus != ""
}

func statusLabel(entry mergedEntry, source Source) string {
	switch source {
	case SourcePlanned:
		return "Geplant"
	case SourceActual:
		return "Anwesend"
	case SourceReconciliation:
		switch {
		case entry.Present && !entry.Planned:
			return "Ungeplant anwesend"
		case entry.Present:
			return "Anwesend"
		case signedOffAbsence(entry):
			return "Abgemeldet (entschuldigt)"
		default:
			return "Fehlt"
		}
	default:
		return ""
	}
}

func countRows(rows []Row, source Source) Counters {
	counters := Counters{}
	for _, row := range rows {
		if row.Planned {
			counters.Planned++
		}
		if row.Present {
			counters.Present++
		}
		switch {
		case row.Planned && !row.Present && row.Excused:
			// Registered sign-off: a justified absence, not an unexplained gap.
			counters.Excused++
		case row.Planned && !row.Present:
			counters.Missing++
		}
		if row.Unplanned {
			counters.Unplanned++
		}
	}
	// Planned/actual previews intentionally show only their own counter;
	// the merge counters are a reconciliation concept.
	switch source {
	case SourcePlanned:
		return Counters{Planned: counters.Planned}
	case SourceActual:
		return Counters{Present: counters.Present}
	default:
		return counters
	}
}

func (s *service) RenderList(ctx context.Context, params Params, format listexport.Format) (listexport.File, error) {
	result, err := s.BuildList(ctx, params)
	if err != nil {
		return listexport.File{}, err
	}

	pickupBased := params.Target == TargetPickupCohort
	columns := []listexport.Column{
		{ID: listexport.ColumnName, Label: "Name"},
		{ID: listexport.ColumnSchoolClass, Label: "Klasse"},
		{ID: listexport.ColumnGroup, Label: "Gruppe"},
		{ID: listexport.ColumnSlot, Label: "Angebot / Zeitraum"},
	}
	if pickupBased {
		columns = append(columns, listexport.Column{ID: listexport.ColumnPlannedPickup, Label: "Geplante Abholung"})
	}
	columns = append(columns, listexport.Column{ID: listexport.ColumnPresenceStatus, Label: "Status"})

	docRows := make([]listexport.Row, 0, len(result.Rows))
	for _, row := range result.Rows {
		values := map[listexport.ColumnID]string{
			listexport.ColumnName:           row.Name,
			listexport.ColumnSchoolClass:    row.SchoolClass,
			listexport.ColumnGroup:          row.GroupName,
			listexport.ColumnSlot:           row.Slot,
			listexport.ColumnPresenceStatus: row.StatusLabel,
		}
		if pickupBased {
			values[listexport.ColumnPlannedPickup] = row.PickupTime
		}
		docRows = append(docRows, listexport.Row{Values: values, GroupTitle: row.GroupTitle})
	}

	doc := listexport.Document{
		Title:       documentTitle(result),
		Subtitle:    subtitle(result),
		GeneratedAt: time.Now(),
		Filters:     exportFilters(params, result),
		Columns:     columns,
		Rows:        docRows,
		Footer:      confidentialityNote,
	}

	filename := exportFilename(params, result)
	return s.listExport.Render(doc, format, filename)
}

func documentTitle(result *Result) string {
	return fmt.Sprintf("Tagesliste %s – %s", result.Source.Label(), result.ListLabel)
}

func exportFilename(params Params, result *Result) string {
	return fmt.Sprintf("Tagesliste %s %s %s", params.Source.Label(), result.ListLabel, params.Date.String())
}

// exportFilters builds the header lines stamped on the printed export.
func exportFilters(params Params, result *Result) []string {
	filters := []string{
		"Datum: " + params.Date.Format("02.01.2006"),
		"Datenbasis: " + params.Source.Label() + " – " + result.Provenance,
		"Enthalten: " + includedSummary(params, result),
	}
	if params.GroupBy != GroupByNone {
		filters = append(filters, "Gruppiert nach: "+params.GroupBy.Label())
	}
	return filters
}

func includedSummary(params Params, result *Result) string {
	if params.Target == TargetPickupCohort {
		return result.ListLabel
	}
	if params.ListKind != ListKindNone {
		count := selectableSlotCount(result.Slots)
		return fmt.Sprintf("%s (%d %s)", result.ListLabel, count, plural(count, "Termin", "Termine"))
	}
	count := selectableSlotCount(result.Slots)
	if params.InstanceIDsSet {
		count = len(params.InstanceIDs)
	}
	return fmt.Sprintf("%d %s", count, plural(count, "Angebot", "Angebote"))
}

func subtitle(result *Result) string {
	switch result.Source {
	case SourcePlanned:
		return fmt.Sprintf("%d geplante Kinder", result.Counters.Planned)
	case SourceActual:
		return fmt.Sprintf("%d anwesende Kinder", result.Counters.Present)
	default:
		return fmt.Sprintf(
			"%d geplant, %d anwesend, %d fehlend, %d abgemeldet, %d ungeplant",
			result.Counters.Planned,
			result.Counters.Present,
			result.Counters.Missing,
			result.Counters.Excused,
			result.Counters.Unplanned,
		)
	}
}

func selectableSlotCount(slots []Slot) int {
	count := 0
	for _, slot := range slots {
		if slot.Status != string(scheduleModel.InstanceStatusCancelled) {
			count++
		}
	}
	return count
}

func plural(count int, singular string, pluralValue string) string {
	if count == 1 {
		return singular
	}
	return pluralValue
}
