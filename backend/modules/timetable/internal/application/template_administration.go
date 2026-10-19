package application

import (
	"context"
	"errors"
	"time"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type templateAdministrationQueries interface {
	ArchiveTemplate(context.Context, int64) (int64, error)
	ResolveLivingTemplateSegment(context.Context, int64) (int64, bool, error)
	ValidateTemplateEducationGroup(context.Context, *int64) error
	FindOrCreateTimeframe(context.Context, time.Time, time.Time, string) (int64, error)
	TemplateAssignmentsOn(context.Context, int64, calendar.Date, int64) (timetable.TemplateAssignments, error)
}

type templateAdministration struct {
	templateAdministrationQueries
	templates *TemplateService
	split     *TemplateSplitService
	templateEnder
}

var _ timetable.TemplateAdministration = (*templateAdministration)(nil)

// NewTemplateAdministration composes the Timetable owner's template writes.
func NewTemplateAdministration(deps ports.TemplateAdministrationDependencies) (timetable.TemplateAdministration, error) {
	if deps.Groups == nil || deps.Categories == nil || deps.Schedules == nil || deps.Enrollments == nil ||
		deps.Supervisors == nil || deps.Instances == nil || deps.InstanceStaff == nil || deps.Participants == nil ||
		deps.Timeframes == nil || deps.EducationGroups == nil || deps.RecurrenceLock == nil || deps.Transactions == nil {
		return nil, errors.New("timetable template administration: required dependency is nil")
	}
	split, err := NewTemplateSplitService(TemplateSplitDependencies{
		GroupRepo:            deps.Groups,
		CategoryRepo:         deps.Categories,
		PlanningTracks:       deps.PlanningTracks,
		ScheduleRepo:         deps.Schedules,
		EnrollmentRepo:       deps.Enrollments,
		SupervisorRepo:       deps.Supervisors,
		InstanceRepo:         deps.Instances,
		TimeframeRepo:        deps.Timeframes,
		Materialization:      deps.Materialization,
		Deviations:           deps.Deviations,
		CareOfferings:        deps.CareOfferings,
		ResyncOfferingRoster: deps.ResyncOfferingRoster,
		RecurrenceLock:       deps.RecurrenceLock,
		SchoolClasses:        deps.SchoolClasses,
		Staffing:             deps.Staffing,
		Logger:               deps.Logger,
		Transactions:         deps.Transactions,
		Today:                deps.Today,
	})
	if err != nil {
		return nil, err
	}
	templates := newTemplateAdministrationService(deps)
	return &templateAdministration{templateAdministrationQueries: templates, templates: templates, split: split, templateEnder: split}, nil
}

func (a *templateAdministration) CreateTemplate(ctx context.Context, cmd timetable.CreateTemplateCommand) (*timetable.CreateTemplateResult, error) {
	return a.templates.CreateTemplate(ctx, createTemplateInput(cmd))
}

func (a *templateAdministration) UpdateTemplate(ctx context.Context, cmd timetable.UpdateTemplateCommand) error {
	return a.templates.UpdateTemplate(ctx, templateUpdateInput(cmd))
}

func (a *templateAdministration) SplitTemplate(ctx context.Context, cmd timetable.SplitTemplateCommand) (*timetable.SplitTemplateResult, error) {
	return a.split.Split(ctx, templateSplitInput(cmd))
}

func (a *templateAdministration) AlignPlannedInstanceStaff(ctx context.Context, templateID int64, staffIDs []int64, from calendar.Date) error {
	return a.templates.reconcilePredecessorInstanceStaff(ctx, templateID, staffIDs, from, nil)
}

// templateAssignmentsOn derives the same concrete people a fresh
// materialized occurrence would receive for one date. It intentionally reads
// the persisted template roster after CreateTemplate has completed, so
// offering-service start dates and selected weekdays are already authoritative.
func (s *TemplateService) TemplateAssignmentsOn(
	ctx context.Context,
	templateID int64,
	date calendar.Date,
	periodID int64,
) (timetable.TemplateAssignments, error) {
	if s.deps.StudentEnrollmentRepo == nil || s.deps.ActivitySupervisorRepo == nil || s.deps.ActivityGroupRepo == nil {
		return timetable.TemplateAssignments{}, &ports.ScheduleError{
			Op:  "derive template assignments: validate dependencies",
			Err: errors.New("required roster repository is nil"),
		}
	}
	planWeekday, err := calendar.PlanWeekday(ctx, date)
	if err != nil {
		return timetable.TemplateAssignments{}, &ports.ScheduleError{Op: "derive template assignments: resolve weekend plan", Err: err}
	}
	studentIDs, err := s.templateStudentsOn(ctx, templateID, date, periodID, planWeekday)
	if err != nil {
		return timetable.TemplateAssignments{}, err
	}
	supervisors, err := s.deps.ActivitySupervisorRepo.FindByGroupID(ctx, templateID)
	if err != nil {
		return timetable.TemplateAssignments{}, &ports.ScheduleError{Op: "derive template assignments: load supervisors", Err: err}
	}
	staffIDs := make([]int64, 0, len(supervisors))
	seenStaff := make(map[int64]struct{}, len(supervisors))
	for _, supervisor := range supervisors {
		if !isSupervisorValidOn(supervisor, date, periodID, planWeekday) {
			continue
		}
		staffIDs = appendUnseen(staffIDs, seenStaff, supervisor.StaffID)
	}
	return timetable.TemplateAssignments{StudentIDs: studentIDs, StaffIDs: staffIDs}, nil
}

// templateStudentsOn returns the enrolled, non-graduated children valid on
// the date followed by the template's dynamic target students.
func (s *TemplateService) templateStudentsOn(ctx context.Context, templateID int64, date calendar.Date, periodID int64, planWeekday int) ([]int64, error) {
	enrollments, err := s.deps.StudentEnrollmentRepo.FindByGroupID(ctx, templateID)
	if err != nil {
		return nil, &ports.ScheduleError{Op: "derive template assignments: load enrollments", Err: err}
	}
	studentIDs := make([]int64, 0, len(enrollments))
	seenStudents := make(map[int64]struct{}, len(enrollments))
	for _, enrollment := range enrollments {
		if !isEnrollmentValidOn(enrollment, date, periodID, planWeekday) || enrollmentStudentIsAlumnus(enrollment) {
			continue
		}
		studentIDs = appendUnseen(studentIDs, seenStudents, enrollment.StudentID)
	}
	targetRepo, ok := s.deps.ActivityGroupRepo.(ports.ActivityGroupTargets)
	if !ok {
		return studentIDs, nil
	}
	targetStudentIDs, err := targetRepo.FindTargetStudentIDs(ctx, templateID)
	if err != nil {
		return nil, &ports.ScheduleError{Op: "derive template assignments: load target students", Err: err}
	}
	for _, studentID := range targetStudentIDs {
		if studentID > 0 {
			studentIDs = appendUnseen(studentIDs, seenStudents, studentID)
		}
	}
	return studentIDs, nil
}

func appendUnseen(ids []int64, seen map[int64]struct{}, id int64) []int64 {
	if _, exists := seen[id]; exists {
		return ids
	}
	seen[id] = struct{}{}
	return append(ids, id)
}

// groupTargetModels maps the contract's dynamic targets onto the retained
// rows. nil stays nil: the template commands tell an omitted target list from
// an empty one.
func groupTargetModels(targets []timetable.GroupTargetInput) []*activityContract.GroupTarget {
	if targets == nil {
		return nil
	}
	rows := make([]*activityContract.GroupTarget, 0, len(targets))
	for _, target := range targets {
		rows = append(rows, &activityContract.GroupTarget{
			TargetGroupType:   target.TargetGroupType,
			TargetGradeLevel:  target.TargetGradeLevel,
			TargetSchoolClass: target.TargetSchoolClass,
			EducationGroupID:  target.EducationGroupID,
		})
	}
	return rows
}

func createTemplateInput(cmd timetable.CreateTemplateCommand) CreateTemplateInput {
	return CreateTemplateInput{
		Name:                  cmd.Name,
		Type:                  cmd.Type,
		Weekdays:              cmd.Weekdays,
		StartTime:             cmd.StartTime,
		EndTime:               cmd.EndTime,
		RoomID:                cmd.RoomID,
		CategoryID:            cmd.CategoryID,
		PlanningTrackID:       cmd.PlanningTrackID,
		MaxParticipants:       cmd.MaxParticipants,
		RequiredStaff:         cmd.RequiredStaff,
		WeekPattern:           cmd.WeekPattern,
		CalendarPeriodID:      cmd.CalendarPeriodID,
		EducationGroupID:      cmd.EducationGroupID,
		TargetGroupType:       cmd.TargetGroupType,
		TargetGradeLevel:      cmd.TargetGradeLevel,
		TargetSchoolClass:     cmd.TargetSchoolClass,
		Targets:               groupTargetModels(cmd.Targets),
		SourceCareOfferingIDs: cmd.SourceCareOfferingIDs,
		SourceGradeLevels:     cmd.SourceGradeLevels,
		SourceSchoolClasses:   cmd.SourceSchoolClasses,
		ListKind:              cmd.ListKind,
		Notes:                 cmd.Notes,
		IncludeClosingDays:    cmd.IncludeClosingDays,
		SeriesLastDay:         activityDatePtr(cmd.SeriesLastDay),
		StudentIDs:            cmd.StudentIDs,
		StaffIDs:              cmd.StaffIDs,
		PrimaryStaffID:        cmd.PrimaryStaffID,
		WeekdayAssignments:    cmd.WeekdayAssignments,
		CreatedBy:             cmd.CreatedBy,
		RosterValidFrom:       cmd.RosterValidFrom,
		ScheduleValidFrom:     cmd.ScheduleValidFrom,
		GradeLevelMax:         cmd.GradeLevelMax,
	}
}

func templateUpdateInput(cmd timetable.UpdateTemplateCommand) TemplateUpdateInput {
	return TemplateUpdateInput{
		TemplateID:                  cmd.TemplateID,
		Fields:                      templateFieldsUpdate(cmd.Fields),
		Weekdays:                    cmd.Weekdays,
		TimeframeID:                 cmd.TimeframeID,
		WeekPattern:                 cmd.WeekPattern,
		CalendarPeriodID:            cmd.CalendarPeriodID,
		RosterValidFrom:             cmd.RosterValidFrom,
		StudentIDs:                  cmd.StudentIDs,
		StaffIDs:                    cmd.StaffIDs,
		PrimaryStaffID:              cmd.PrimaryStaffID,
		Targets:                     groupTargetModels(cmd.Targets),
		WeekdayAssignments:          cmd.WeekdayAssignments,
		GradeLevelMax:               cmd.GradeLevelMax,
		SeriesRosterFrom:            cmd.SeriesRosterFrom,
		SeriesRosterScopeStudentIDs: cmd.SeriesRosterScopeStudentIDs,
		SeriesRosterScopeStaffIDs:   cmd.SeriesRosterScopeStaffIDs,
		SeriesRosterScopeWeekdays:   cmd.SeriesRosterScopeWeekdays,
		SeriesRosterPrimaryChanged:  cmd.SeriesRosterPrimaryChanged,
		StartDate:                   cmd.StartDate,
	}
}

func templateFieldsUpdate(fields timetable.TemplateFields) timetable.TemplateUpdate {
	return timetable.TemplateUpdate(fields)
}

func templateSplitInput(cmd timetable.SplitTemplateCommand) TemplateSplitInput {
	return TemplateSplitInput{
		TemplateID:                    cmd.TemplateID,
		EffectiveDate:                 cmd.EffectiveDate,
		Name:                          cmd.Name,
		Type:                          cmd.Type,
		Weekdays:                      cmd.Weekdays,
		StartTime:                     cmd.StartTime,
		EndTime:                       cmd.EndTime,
		RoomID:                        cmd.RoomID,
		CategoryID:                    cmd.CategoryID,
		PlanningTrackID:               cmd.PlanningTrackID,
		PlanningTrackIDProvided:       cmd.PlanningTrackIDProvided,
		MaxParticipants:               cmd.MaxParticipants,
		MaxParticipantsProvided:       cmd.MaxParticipantsProvided,
		RequiredStaff:                 cmd.RequiredStaff,
		RequiredStaffProvided:         cmd.RequiredStaffProvided,
		WeekPattern:                   cmd.WeekPattern,
		CalendarPeriodID:              cmd.CalendarPeriodID,
		EducationGroupID:              cmd.EducationGroupID,
		TargetGroupType:               cmd.TargetGroupType,
		TargetGradeLevel:              cmd.TargetGradeLevel,
		TargetSchoolClass:             cmd.TargetSchoolClass,
		Targets:                       groupTargetModels(cmd.Targets),
		SourceCareOfferingIDs:         cmd.SourceCareOfferingIDs,
		SourceCareOfferingIDsProvided: cmd.SourceCareOfferingIDsProvided,
		SourceGradeLevels:             cmd.SourceGradeLevels,
		SourceGradeLevelsProvided:     cmd.SourceGradeLevelsProvided,
		SourceSchoolClasses:           cmd.SourceSchoolClasses,
		SourceSchoolClassesProvided:   cmd.SourceSchoolClassesProvided,
		Notes:                         cmd.Notes,
		NotesProvided:                 cmd.NotesProvided,
		ListKind:                      cmd.ListKind,
		ListKindProvided:              cmd.ListKindProvided,
		IncludeClosingDays:            cmd.IncludeClosingDays,
		StudentIDs:                    cmd.StudentIDs,
		StaffIDs:                      cmd.StaffIDs,
		PrimaryStaffID:                cmd.PrimaryStaffID,
		WeekdayAssignments:            cmd.WeekdayAssignments,
		MaterializeFrom:               cmd.MaterializeFrom,
		MaterializeTo:                 cmd.MaterializeTo,
		GradeLevelMax:                 cmd.GradeLevelMax,
		ActorAccountID:                cmd.ActorAccountID,
	}
}

func newTemplateAdministrationService(deps ports.TemplateAdministrationDependencies) *TemplateService {
	return NewTemplateService(TemplateServiceDependencies{
		InstanceStudentRepo:    deps.Participants,
		ActivityInstanceRepo:   deps.Instances,
		ActivityScheduleRepo:   deps.Schedules,
		InstanceStaffRepo:      deps.InstanceStaff,
		ActivityCategoryRepo:   deps.Categories,
		PlanningTracks:         deps.PlanningTracks,
		ActivityGroupRepo:      deps.Groups,
		ActivitySupervisorRepo: deps.Supervisors,
		StudentEnrollmentRepo:  deps.Enrollments,
		TimeframeRepo:          deps.Timeframes,
		EducationGroups:        deps.EducationGroups,
		CareOfferings:          deps.CareOfferings,
		ResyncOfferingRoster:   deps.ResyncOfferingRoster,
		RecurrenceLock:         deps.RecurrenceLock,
		SchoolClasses:          deps.SchoolClasses,
		Staffing:               deps.Staffing,
		Logger:                 deps.Logger,
		Transactions:           deps.Transactions,
		Today:                  deps.Today,
	})
}

type templateEnder interface {
	EndTemplateFromDate(context.Context, timetable.EndTemplateCommand) (*timetable.EndTemplateResult, error)
}
