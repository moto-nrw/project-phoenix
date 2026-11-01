package application

import (
	"context"
	"errors"
	"fmt"
	"slices"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

const connectOfferingSourceOp = "connect offering source"

// templateRosterReview is the Timetable owner's roster review (#5103) over
// the template writes.
type templateRosterReview struct {
	templates *TemplateService
}

var _ timetable.TemplateRosterReview = (*templateRosterReview)(nil)

// FutureRosters reads the schedules, enrollment rows, targets and target
// members of all templates at once.
func (r *templateRosterReview) FutureRosters(ctx context.Context, templateIDs []int64, from calendar.Date) (map[int64]timetable.TemplateFutureRoster, error) {
	result := make(map[int64]timetable.TemplateFutureRoster, len(templateIDs))
	if len(templateIDs) == 0 {
		return result, nil
	}
	s := r.templates
	if from.IsZero() {
		return nil, &ports.ScheduleError{Op: "future rosters", Err: fmt.Errorf("%w: from is required", timetable.ErrRosterReviewInvalid)}
	}
	schedules, err := groupsSchedules(ctx, s.deps.ActivityScheduleRepo, templateIDs)
	if err != nil {
		return nil, &ports.ScheduleError{Op: "future rosters: load schedules", Err: err}
	}
	enrollments, err := r.futureRosterEnrollments(ctx, templateIDs)
	if err != nil {
		return nil, err
	}
	targets, err := legacyGroupTargets(ctx, s.deps.ActivityGroupRepo, templateIDs)
	if err != nil {
		return nil, &ports.ScheduleError{Op: "future rosters: load targets", Err: err}
	}
	members, err := s.deps.ActivityGroupRepo.ListTargetStudentIDs(ctx, templateIDs)
	if err != nil {
		return nil, &ports.ScheduleError{Op: "future rosters: load target students", Err: err}
	}
	schedulesByTemplate := make(map[int64][]*activityContract.Schedule, len(templateIDs))
	for _, schedule := range schedules {
		schedulesByTemplate[schedule.ActivityGroupID] = append(schedulesByTemplate[schedule.ActivityGroupID], schedule)
	}
	for templateID, templateSchedules := range schedulesByTemplate {
		roster := futureRoster(templateID, templateSchedules, from, enrollments[templateID], members[templateID])
		roster.TargetSchoolClasses, roster.TargetGradeLevels = uniformTargets(targets[templateID])
		result[templateID] = roster
	}
	return result, nil
}

// PlannedRosters reads the enrollment rows and the target members of all
// templates at once (#5343).
func (r *templateRosterReview) PlannedRosters(ctx context.Context, templateIDs []int64) (map[int64]timetable.TemplatePlannedRoster, error) {
	result := make(map[int64]timetable.TemplatePlannedRoster, len(templateIDs))
	if len(templateIDs) == 0 {
		return result, nil
	}
	enrollments, err := r.futureRosterEnrollments(ctx, templateIDs)
	if err != nil {
		return nil, err
	}
	members, err := r.templates.deps.ActivityGroupRepo.ListTargetStudentIDs(ctx, templateIDs)
	if err != nil {
		return nil, &ports.ScheduleError{Op: "planned rosters: load target students", Err: err}
	}
	for _, templateID := range templateIDs {
		roster := timetable.TemplatePlannedRoster{TargetStudentIDs: members[templateID]}
		for _, row := range enrollments[templateID] {
			roster.Entries = append(roster.Entries, plannedRosterEntry(row))
		}
		result[templateID] = roster
	}
	return result, nil
}

func plannedRosterEntry(row *activityContract.StudentEnrollment) timetable.TemplateRosterEntry {
	entry := timetable.TemplateRosterEntry{
		StudentID: row.StudentID, CalendarPeriodID: row.CalendarPeriodID, Weekday: row.Weekday,
		SelectedWeekdays: slices.Clone(row.SelectedWeekdays), FromBooking: row.EnrollmentRequestChildID != nil,
	}
	if row.KeptWithoutBookingThrough != nil {
		entry.KeptWithoutBookingThrough = new(calendar.Date(*row.KeptWithoutBookingThrough))
	}
	if !row.ValidFrom.IsZero() {
		entry.ValidFrom = new(calendar.Date(row.ValidFrom))
	}
	if row.ValidUntil != nil {
		entry.ValidUntil = new(calendar.Date(*row.ValidUntil))
	}
	return entry
}

// futureRosterEnrollments lists the enrollment rows of the templates without
// the children who left the school.
func (r *templateRosterReview) futureRosterEnrollments(ctx context.Context, templateIDs []int64) (map[int64][]*activityContract.StudentEnrollment, error) {
	s := r.templates
	rows, err := listLegacyStudentEnrollments(ctx, s.deps.StudentEnrollmentRepo, timetable.StudentEnrollmentFilter{ActivityGroupIDs: templateIDs}, "future rosters")
	if err != nil {
		return nil, &ports.ScheduleError{Op: "future rosters: load enrollments", Err: err}
	}
	alumni := map[int64]bool{}
	if s.deps.Directories.StudentAlumni != nil && len(rows) > 0 {
		studentIDs := make([]int64, 0, len(rows))
		for _, row := range rows {
			studentIDs = append(studentIDs, row.StudentID)
		}
		if alumni, err = s.deps.Directories.StudentAlumni(ctx, studentIDs); err != nil {
			return nil, &ports.ScheduleError{Op: "future rosters: load alumni", Err: err}
		}
	}
	byTemplate := make(map[int64][]*activityContract.StudentEnrollment, len(templateIDs))
	for _, row := range rows {
		if row != nil && !alumni[row.StudentID] {
			byTemplate[row.ActivityGroupID] = append(byTemplate[row.ActivityGroupID], row)
		}
	}
	return byTemplate, nil
}

// futureRoster plans every enrollment row still valid on or after the
// boundary on the weekdays it applies to, and every target member on every
// schedule weekday.
func futureRoster(
	templateID int64,
	schedules []*activityContract.Schedule,
	from calendar.Date,
	enrollments []*activityContract.StudentEnrollment,
	targetMembers []int64,
) timetable.TemplateFutureRoster {
	if validFrom, _, err := commonScheduleValidityEnvelope(schedules); err == nil && validFrom != nil && validFrom.After(from) {
		from = *validFrom
	}
	var periodID *int64
	if len(schedules) > 0 {
		periodID = schedules[0].CalendarPeriodID
	}
	weekdays := make(map[int][]int64, len(schedules))
	for _, schedule := range schedules {
		students := slices.Clone(targetMembers)
		for _, row := range enrollments {
			if enrollmentPlannedOnOrAfter(row, from, periodID, schedule.Weekday) {
				students = append(students, row.StudentID)
			}
		}
		slices.Sort(students)
		weekdays[schedule.Weekday] = slices.Compact(students)
	}
	return timetable.TemplateFutureRoster{TemplateID: templateID, From: from, Weekdays: weekdays}
}

// enrollmentPlannedOnOrAfter reports whether the row still plans its child
// on the weekday on or after from, in the template's period.
func enrollmentPlannedOnOrAfter(row *activityContract.StudentEnrollment, from calendar.Date, periodID *int64, weekday int) bool {
	if row.ValidUntil != nil && !calendar.Date(*row.ValidUntil).After(from) {
		return false
	}
	if row.CalendarPeriodID != nil && periodID != nil && *row.CalendarPeriodID != *periodID {
		return false
	}
	if !rosterWeekdayApplies(row.Weekday, weekday) {
		return false
	}
	return len(row.SelectedWeekdays) == 0 || slices.Contains(row.SelectedWeekdays, weekday)
}

// uniformTargets names the dynamic targets when all are Klassen or all are
// Jahrgänge; a Gruppe or a mix restricts nothing.
func uniformTargets(targets []*activityContract.GroupTarget) ([]string, []int) {
	var classes []string
	var grades []int
	for _, target := range targets {
		switch {
		case target.TargetGroupType == activityContract.TargetGroupTypeKlasse && target.TargetSchoolClass != nil && len(grades) == 0:
			classes = append(classes, *target.TargetSchoolClass)
		case target.TargetGroupType == activityContract.TargetGroupTypeJahrgang && target.TargetGradeLevel != nil && len(classes) == 0:
			grades = append(grades, int(*target.TargetGradeLevel))
		default:
			return nil, nil
		}
	}
	slices.Sort(grades)
	return classes, slices.Compact(grades)
}

// SeriesSegments lists the series segments with days on or after from,
// ordered by their start.
func (r *templateRosterReview) SeriesSegments(ctx context.Context, templateID int64, from calendar.Date) ([]timetable.TemplateSeriesSegment, error) {
	segments, err := r.futureSegments(ctx, templateID, from)
	if err != nil {
		return nil, err
	}
	result := make([]timetable.TemplateSeriesSegment, 0, len(segments))
	for _, segment := range segments {
		result = append(result, timetable.TemplateSeriesSegment{
			TemplateID: segment.Group.ID, CalendarPeriodID: segment.Group.CalendarPeriodID,
			ValidFrom: segment.ValidFrom, ValidUntil: segment.ValidUntil,
		})
	}
	return result, nil
}

func (r *templateRosterReview) futureSegments(ctx context.Context, templateID int64, from calendar.Date) ([]templateSeriesSegment, error) {
	s := r.templates
	segments, err := loadTemplateSeriesSegments(ctx, s.deps.ActivityGroupRepo, s.deps.ActivityScheduleRepo, s.getLogger(), templateID)
	if err != nil {
		return nil, err
	}
	future := make([]templateSeriesSegment, 0, len(segments))
	for _, segment := range segments {
		if segment.Group != nil && (segment.ValidUntil == nil || segment.ValidUntil.After(from)) {
			future = append(future, segment)
		}
	}
	slices.SortStableFunc(future, func(left, right templateSeriesSegment) int {
		switch {
		case left.ValidFrom == nil && right.ValidFrom == nil:
			return 0
		case left.ValidFrom == nil:
			return -1
		case right.ValidFrom == nil:
			return 1
		}
		return left.ValidFrom.Compare(*right.ValidFrom)
	})
	return future, nil
}

// NextSeriesOccurrence reads the first planned, non-spontaneous occurrence of
// any series segment on or after from.
func (r *templateRosterReview) NextSeriesOccurrence(ctx context.Context, templateID int64, from calendar.Date) (*calendar.Date, error) {
	segments, err := r.futureSegments(ctx, templateID, from)
	if err != nil || len(segments) == 0 {
		return nil, err
	}
	ids := make([]int64, 0, len(segments))
	for _, segment := range segments {
		ids = append(ids, segment.Group.ID)
	}
	spontaneous := false
	fromText := from.String()
	instances, err := r.templates.deps.ActivityInstanceRepo.Plans.ListActivityInstances(ctx, timetable.ActivityInstanceFilter{
		ActivityGroupIDs: ids, FromDate: &fromText, Status: "planned", IsSpontaneous: &spontaneous,
		OrderByDateAndTime: true, Limit: 1,
	})
	if err != nil {
		return nil, &ports.ScheduleError{Op: "next series occurrence", Err: err}
	}
	if len(instances) == 0 {
		return nil, nil
	}
	date, err := calendar.ParseDate(instances[0].Date)
	if err != nil {
		return nil, &ports.ScheduleError{Op: "next series occurrence: parse date", Err: err}
	}
	return &date, nil
}

// SetManualRosterConfirmed writes the flag on the series root of an active
// Regeltermin with children; a duty has no roster to keep.
func (r *templateRosterReview) SetManualRosterConfirmed(ctx context.Context, templateID int64, confirmed bool) error {
	group, err := r.templates.reviewedTemplate(ctx, templateID)
	if err != nil {
		return err
	}
	rows, err := r.templates.deps.ActivityGroupRepo.UpdateTemplateRosterReview(ctx, group.ID, timetable.TemplateRosterReviewUpdate{ManualRosterConfirmed: &confirmed})
	if err != nil {
		return &ports.ScheduleError{Op: "set manual roster", Err: err}
	}
	if rows == 0 {
		return &ports.ScheduleError{Op: "set manual roster", Err: timetable.ErrTemplateSegmentNotEditable}
	}
	return nil
}

// reviewedTemplate loads an active template the roster review may change.
func (s *TemplateService) reviewedTemplate(ctx context.Context, templateID int64) (*activityContract.Group, error) {
	group, err := findLegacyGroup(ctx, s.deps.ActivityGroupRepo, templateID)
	if err != nil || group == nil || !group.IsTemplate || group.ArchivedAt != nil {
		if err != nil && !domain.IsRecordNotFound(err) {
			return nil, &ports.ScheduleError{Op: "roster review: load template", Err: err}
		}
		return nil, &ports.ScheduleError{Op: "roster review: load template", Err: timetable.ErrTemplateSegmentNotEditable}
	}
	if group.Type == activityContract.GroupTypeDuty {
		return nil, &ports.ScheduleError{Op: "roster review: load template", Err: fmt.Errorf("%w: a duty has no children", timetable.ErrRosterReviewInvalid)}
	}
	return group, nil
}

// ConnectOfferingSource switches every series segment with days on or after
// the effective date to the offerings as source, in place: target and source
// columns, the hand roster from the boundary on, the sourced roster and the
// already materialized planned occurrences. It runs in the caller's tenant
// transaction (the route's unit of work) and holds the recurrence gate like a
// template update; the Care Plan resync joins the same unit.
func (r *templateRosterReview) ConnectOfferingSource(ctx context.Context, cmd timetable.ConnectOfferingSourceCommand) (timetable.ConnectOfferingSourceResult, error) {
	s := r.templates
	if err := validateConnectOfferingSource(cmd, calendar.TodayDate()); err != nil {
		return timetable.ConnectOfferingSourceResult{}, err
	}
	if _, ok := tenant.TransactionFromContext(ctx); !ok || tenant.FromContext(ctx) <= 0 {
		return timetable.ConnectOfferingSourceResult{}, &ports.ScheduleError{Op: connectOfferingSourceOp, Err: errors.New("requires the tenant transaction of the request")}
	}
	if err := s.lockRecurrence(ctx, connectOfferingSourceOp); err != nil {
		return timetable.ConnectOfferingSourceResult{}, err
	}
	segments, err := r.futureSegments(ctx, cmd.TemplateID, cmd.EffectiveDate)
	if err != nil {
		return timetable.ConnectOfferingSourceResult{}, err
	}
	if len(segments) == 0 {
		return timetable.ConnectOfferingSourceResult{}, &ports.ScheduleError{Op: connectOfferingSourceOp, Err: timetable.ErrTemplateSegmentNotEditable}
	}
	result := timetable.ConnectOfferingSourceResult{EffectiveDate: cmd.EffectiveDate}
	for _, segment := range segments {
		if err := r.connectSegment(ctx, cmd, segment); err != nil {
			return timetable.ConnectOfferingSourceResult{}, err
		}
		result.TemplateIDs = append(result.TemplateIDs, segment.Group.ID)
	}
	keptByHand := false
	if _, err := s.deps.ActivityGroupRepo.UpdateTemplateRosterReview(ctx, cmd.TemplateID, timetable.TemplateRosterReviewUpdate{ManualRosterConfirmed: &keptByHand}); err != nil {
		return timetable.ConnectOfferingSourceResult{}, &ports.ScheduleError{Op: connectOfferingSourceOp + ": clear manual roster", Err: err}
	}
	return result, nil
}

func validateConnectOfferingSource(cmd timetable.ConnectOfferingSourceCommand, today calendar.Date) error {
	switch {
	case cmd.TemplateID <= 0:
		return &ports.ScheduleError{Op: connectOfferingSourceOp, Err: fmt.Errorf("%w: template id is required", timetable.ErrRosterReviewInvalid)}
	case len(cmd.OfferingIDs) == 0:
		return &ports.ScheduleError{Op: connectOfferingSourceOp, Err: fmt.Errorf("%w: at least one offering is required", timetable.ErrRosterReviewInvalid)}
	case len(cmd.GradeLevels) > 0 && len(cmd.SchoolClasses) > 0:
		return &ports.ScheduleError{Op: connectOfferingSourceOp, Err: fmt.Errorf("%w: grade and class filters exclude each other", timetable.ErrRosterReviewInvalid)}
	case cmd.EffectiveDate.IsZero() || cmd.EffectiveDate.Before(today):
		return &ports.ScheduleError{Op: connectOfferingSourceOp, Err: fmt.Errorf("%w: effective date must not lie in the past", timetable.ErrRosterReviewInvalid)}
	}
	return nil
}

// connectSegment converts one segment from the boundary max(effective date,
// segment start) on.
func (r *templateRosterReview) connectSegment(ctx context.Context, cmd timetable.ConnectOfferingSourceCommand, segment templateSeriesSegment) error {
	s := r.templates
	group := segment.Group
	if group.Type == activityContract.GroupTypeDuty {
		return &ports.ScheduleError{Op: connectOfferingSourceOp, Err: fmt.Errorf("%w: a duty has no children", timetable.ErrRosterReviewInvalid)}
	}
	boundary := cmd.EffectiveDate
	if segment.ValidFrom != nil && segment.ValidFrom.After(boundary) {
		boundary = *segment.ValidFrom
	}
	previousTargets, err := listTargetStudentIDs(ctx, s.deps.ActivityGroupRepo, group.ID)
	if err != nil {
		return &ports.ScheduleError{Op: connectOfferingSourceOp + ": load target students", Err: err}
	}
	if err := s.validateOfferingSourceReference(ctx, cmd.OfferingIDs, group.SourceCareOfferingIDs, group.CalendarPeriodID, connectOfferingSourceOp+": validate offering source"); err != nil {
		return err
	}
	if err := r.writeConnectedSource(ctx, cmd, group.ID); err != nil {
		return err
	}
	rows, err := groupStudentEnrollments(ctx, s.deps.StudentEnrollmentRepo, s.deps.Directories, group.ID)
	if err != nil {
		return &ports.ScheduleError{Op: connectOfferingSourceOp + ": load enrollments", Err: err}
	}
	_, retired, err := s.retireUnprotectedTemplateEnrollments(ctx, rows, group.CalendarPeriodID, boundary, segment.ValidUntil, group.CalendarPeriodID)
	if err != nil {
		return err
	}
	if err := s.resyncConnectedSegment(ctx, cmd, group, boundary); err != nil {
		return err
	}
	return r.realignConnectedOccurrences(ctx, group, boundary, unionStudentIDs(retired, previousTargets))
}

func (r *templateRosterReview) writeConnectedSource(ctx context.Context, cmd timetable.ConnectOfferingSourceCommand, groupID int64) error {
	s := r.templates
	changed, err := s.deps.ActivityGroupRepo.UpdateTemplateRosterReview(ctx, groupID, timetable.TemplateRosterReviewUpdate{
		Source: &timetable.OfferingSourceInput{CareOfferingIDs: cmd.OfferingIDs, GradeLevels: cmd.GradeLevels, SchoolClasses: cmd.SchoolClasses},
	})
	if err != nil {
		return &ports.ScheduleError{Op: connectOfferingSourceOp + ": write source", Err: err}
	}
	if changed != 1 {
		return &ports.ScheduleError{Op: connectOfferingSourceOp + ": write source", Err: timetable.ErrTemplateSegmentNotEditable}
	}
	if err := replaceLegacyGroupTargets(ctx, s.deps.ActivityGroupRepo, groupID, nil); err != nil {
		return &ports.ScheduleError{Op: connectOfferingSourceOp + ": clear targets", Err: err}
	}
	return nil
}

func (s *TemplateService) resyncConnectedSegment(ctx context.Context, cmd timetable.ConnectOfferingSourceCommand, group *activityContract.Group, boundary calendar.Date) error {
	if s.deps.ResyncOfferingRoster == nil {
		return &ports.ScheduleError{Op: connectOfferingSourceOp, Err: errors.New("offering roster resync is not configured")}
	}
	if err := s.deps.ResyncOfferingRoster(ctx, timetable.OfferingRosterResyncInput{
		TemplateID:       group.ID,
		OfferingIDs:      cmd.OfferingIDs,
		GradeLevels:      cmd.GradeLevels,
		SchoolClasses:    cmd.SchoolClasses,
		CalendarPeriodID: group.CalendarPeriodID,
		EffectiveFrom:    boundary,
	}); err != nil {
		return &ports.ScheduleError{Op: connectOfferingSourceOp + ": resync offering roster", Err: err}
	}
	if s.deps.CareOfferings.ValidateSeries == nil {
		return nil
	}
	if err := s.deps.CareOfferings.ValidateSeries(ctx, group.ID); err != nil {
		return careOfferingValidationError(ctx, s.deps.CareOfferings,
			connectOfferingSourceOp+": validate linked care offerings",
			"connected series is incompatible with an existing care offering", err)
	}
	return nil
}

// realignConnectedOccurrences removes the children without coverage from the
// planned occurrences from the boundary on and adds the covered ones: the
// retired hand children, the former target members and the new sourced
// children.
func (r *templateRosterReview) realignConnectedOccurrences(ctx context.Context, group *activityContract.Group, boundary calendar.Date, formerStudentIDs []int64) error {
	s := r.templates
	rows, err := groupStudentEnrollments(ctx, s.deps.StudentEnrollmentRepo, s.deps.Directories, group.ID)
	if err != nil {
		return &ports.ScheduleError{Op: connectOfferingSourceOp + ": reload enrollments", Err: err}
	}
	current := make([]int64, 0, len(rows))
	for _, row := range rows {
		current = append(current, row.StudentID)
	}
	studentIDs := unionStudentIDs(formerStudentIDs, current)
	if _, _, err := s.newRosterReconciler().reconcileSourcedTemplateRosters(ctx, group.ID, studentIDs, boundary, nil); err != nil {
		return &ports.ScheduleError{Op: connectOfferingSourceOp + ": realign occurrences", Err: err}
	}
	return nil
}
