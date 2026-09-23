package compose

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// TemplateAdministrationDependencies wires timetable.TemplateAdministration:
// the retained template rows, the owner's materialization and recurrence
// gate, and the ports the composition root binds — the instance lifecycle's
// deviation preservation, Enrollment's care-offering checks and roster resync
// (late-bound: the decision service is built after this one), School
// Structure's education groups and class rules, and the realtime staffing
// announcement. Staffing, Logger and Today are optional.
type TemplateAdministrationDependencies struct {
	Groups               activitiesModel.GroupRepository
	Categories           activitiesModel.CategoryRepository
	Schedules            activitiesModel.ScheduleRepository
	Enrollments          activitiesModel.StudentEnrollmentRepository
	Supervisors          activitiesModel.SupervisorPlannedRepository
	Instances            scheduleModel.ActivityInstanceRepository
	InstanceStaff        scheduleModel.InstanceStaffRepository
	Participants         scheduleModel.InstanceStudentRepository
	Timeframes           scheduleModel.TimeframeRepository
	PlanningTracks       PlanningTrackAssignments
	EducationGroups      EducationGroupDirectory
	Materialization      timetable.MaterializationCapability
	Deviations           SeriesDeviations
	CareOfferings        CareOfferingChecks
	ResyncOfferingRoster func(context.Context, timetable.OfferingRosterResyncInput) error
	RecurrenceLock       timetable.RecurrenceWriteLock
	SchoolClasses        SchoolClassRules
	Staffing             StaffingAnnouncer
	Logger               *slog.Logger
	DB                   *bun.DB
	Today                func() timezone.Date
}

type templateAdministration struct {
	templates *TemplateService
	split     *TemplateSplitService
}

var _ timetable.TemplateAdministration = (*templateAdministration)(nil)

// NewTemplateAdministration composes the Timetable owner's template writes.
func NewTemplateAdministration(deps TemplateAdministrationDependencies) (timetable.TemplateAdministration, error) {
	if deps.Groups == nil || deps.Categories == nil || deps.Schedules == nil || deps.Enrollments == nil ||
		deps.Supervisors == nil || deps.Instances == nil || deps.InstanceStaff == nil || deps.Participants == nil ||
		deps.Timeframes == nil || deps.EducationGroups == nil || deps.RecurrenceLock == nil || deps.DB == nil {
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
		DB:                   deps.DB,
		Today:                deps.Today,
	})
	if err != nil {
		return nil, err
	}
	return &templateAdministration{
		templates: NewTemplateService(TemplateServiceDependencies{
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
			DB:                     deps.DB,
			Today:                  deps.Today,
		}),
		split: split,
	}, nil
}

func (a *templateAdministration) CreateTemplate(ctx context.Context, cmd timetable.CreateTemplateCommand) (*timetable.CreateTemplateResult, error) {
	return a.templates.CreateTemplate(ctx, createTemplateInput(cmd))
}

func (a *templateAdministration) UpdateTemplate(ctx context.Context, cmd timetable.UpdateTemplateCommand) error {
	return a.templates.UpdateTemplate(ctx, templateUpdateInput(cmd))
}

func (a *templateAdministration) ArchiveTemplate(ctx context.Context, templateID int64) (int64, error) {
	return a.templates.ArchiveTemplate(ctx, templateID)
}

func (a *templateAdministration) SplitTemplate(ctx context.Context, cmd timetable.SplitTemplateCommand) (*timetable.SplitTemplateResult, error) {
	return a.split.Split(ctx, templateSplitInput(cmd))
}

func (a *templateAdministration) EndTemplateFromDate(ctx context.Context, cmd timetable.EndTemplateCommand) (*timetable.EndTemplateResult, error) {
	return a.split.EndFromDate(ctx, cmd)
}

func (a *templateAdministration) ResolveLivingTemplateSegment(ctx context.Context, templateID int64) (int64, bool, error) {
	return a.templates.ResolveLivingTemplateSegment(ctx, templateID)
}

func (a *templateAdministration) ValidateTemplateEducationGroup(ctx context.Context, groupID *int64) error {
	return a.templates.ValidateTemplateEducationGroup(ctx, groupID)
}

func (a *templateAdministration) FindOrCreateTimeframe(ctx context.Context, start, end time.Time, descHint string) (int64, error) {
	return a.templates.FindOrCreateTimeframe(ctx, start, end, descHint)
}

func (a *templateAdministration) TemplateAssignmentsOn(ctx context.Context, templateID int64, date timezone.Date, calendarPeriodID int64) (timetable.TemplateAssignments, error) {
	return a.templates.templateAssignmentsOn(ctx, templateID, date, calendarPeriodID)
}

func (a *templateAdministration) AlignPlannedInstanceStaff(ctx context.Context, templateID int64, staffIDs []int64, from timezone.Date) error {
	return a.templates.reconcilePredecessorInstanceStaff(ctx, templateID, staffIDs, from, nil)
}

// templateAssignmentsOn derives the same concrete people a fresh
// materialized occurrence would receive for one date. It intentionally reads
// the persisted template roster after CreateTemplate has completed, so
// offering-service start dates and selected weekdays are already authoritative.
func (s *TemplateService) templateAssignmentsOn(
	ctx context.Context,
	templateID int64,
	date timezone.Date,
	periodID int64,
) (timetable.TemplateAssignments, error) {
	if s.deps.StudentEnrollmentRepo == nil || s.deps.ActivitySupervisorRepo == nil || s.deps.ActivityGroupRepo == nil {
		return timetable.TemplateAssignments{}, &ScheduleError{
			Op:  "derive template assignments: validate dependencies",
			Err: errors.New("required roster repository is nil"),
		}
	}
	studentIDs, err := s.templateStudentsOn(ctx, templateID, date, periodID)
	if err != nil {
		return timetable.TemplateAssignments{}, err
	}
	supervisors, err := s.deps.ActivitySupervisorRepo.FindByGroupID(ctx, templateID)
	if err != nil {
		return timetable.TemplateAssignments{}, &ScheduleError{Op: "derive template assignments: load supervisors", Err: err}
	}
	staffIDs := make([]int64, 0, len(supervisors))
	seenStaff := make(map[int64]struct{}, len(supervisors))
	for _, supervisor := range supervisors {
		if !isSupervisorValidOn(supervisor, date, periodID) {
			continue
		}
		staffIDs = appendUnseen(staffIDs, seenStaff, supervisor.StaffID)
	}
	return timetable.TemplateAssignments{StudentIDs: studentIDs, StaffIDs: staffIDs}, nil
}

// templateStudentsOn returns the enrolled, non-graduated children valid on
// the date followed by the template's dynamic target students.
func (s *TemplateService) templateStudentsOn(ctx context.Context, templateID int64, date timezone.Date, periodID int64) ([]int64, error) {
	enrollments, err := s.deps.StudentEnrollmentRepo.FindByGroupID(ctx, templateID)
	if err != nil {
		return nil, &ScheduleError{Op: "derive template assignments: load enrollments", Err: err}
	}
	studentIDs := make([]int64, 0, len(enrollments))
	seenStudents := make(map[int64]struct{}, len(enrollments))
	for _, enrollment := range enrollments {
		if !isEnrollmentValidOn(enrollment, date, periodID) || enrollmentStudentIsAlumnus(enrollment) {
			continue
		}
		studentIDs = appendUnseen(studentIDs, seenStudents, enrollment.StudentID)
	}
	targetRepo, ok := s.deps.ActivityGroupRepo.(activitiesModel.GroupTargetRepository)
	if !ok {
		return studentIDs, nil
	}
	targetStudentIDs, err := targetRepo.FindTargetStudentIDs(ctx, templateID)
	if err != nil {
		return nil, &ScheduleError{Op: "derive template assignments: load target students", Err: err}
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
func groupTargetModels(targets []timetable.GroupTargetInput) []*activitiesModel.GroupTarget {
	if targets == nil {
		return nil
	}
	rows := make([]*activitiesModel.GroupTarget, 0, len(targets))
	for _, target := range targets {
		rows = append(rows, &activitiesModel.GroupTarget{
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

func templateFieldsUpdate(fields timetable.TemplateFields) activitiesModel.TemplateFieldsUpdate {
	return activitiesModel.TemplateFieldsUpdate{
		Name:                    fields.Name,
		Type:                    fields.Type,
		CategoryID:              fields.CategoryID,
		PlanningTrackID:         fields.PlanningTrackID,
		PlanningTrackIDProvided: fields.PlanningTrackIDProvided,
		RoomID:                  fields.RoomID,
		EducationGroupID:        fields.EducationGroupID,
		MaxParticipants:         fields.MaxParticipants,
		MaxParticipantsProvided: fields.MaxParticipantsProvided,
		RequiredStaff:           fields.RequiredStaff,
		CalendarPeriodID:        fields.CalendarPeriodID,
		TargetGroupType:         fields.TargetGroupType,
		TargetGradeLevel:        fields.TargetGradeLevel,
		TargetSchoolClass:       fields.TargetSchoolClass,
		ListKind:                fields.ListKind,
		Notes:                   fields.Notes,
		SourceCareOfferingIDs:   fields.SourceCareOfferingIDs,
		SourceGradeLevels:       fields.SourceGradeLevels,
		SourceSchoolClasses:     fields.SourceSchoolClasses,
	}
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
