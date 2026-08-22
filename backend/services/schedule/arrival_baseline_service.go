package schedule

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
	"github.com/moto-nrw/project-phoenix/internal/sliceutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	educationModel "github.com/moto-nrw/project-phoenix/models/education"
	enrollmentModel "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/config"
)

// Arrival-time baseline (#2414, ADR 0005).
//
// The regular arrival time of a child is not stored per child any more. It is
// projected at read time from two independent sources:
//
//   - WHEN: education.class_arrival_times carries the Unterrichtsschluss of
//     the child's school class per weekday. The production analysis behind
//     that split: 97.5% to 100% of the per-child rows equalled the value of
//     their (class, weekday) pair, while the booked care offering carried two
//     to three different arrival times per weekday and therefore says nothing
//     about the time.
//   - WHICH DAYS: with enrollment.bookings_authoritative switched on, the
//     approved request_child_offerings decide, with the same half-open
//     [valid_from, valid_until) window the pickup baseline uses — so an
//     Abmeldung ends the arrival on its effective date without a job. With it
//     off, every weekday the class carries a time for is a care day, and
//     schools without a Halbjahresanmeldung are unaffected.
//
// A stored schedule.student_arrival_schedules row is a manual override and
// wins over the projection. Nothing derived is ever written back: the pickup
// side materialized first and left 408 ignored rows behind at one tenant,
// which is exactly what ADR 0001 and this file avoid repeating.

// ArrivalWeek maps ISO weekday to the row applicable that day.
type ArrivalWeek map[int]*scheduleModel.StudentArrivalSchedule

// ArrivalPlanByDate keeps a full week per date because the care-day
// derivation must tell "no arrival on Monday" from "no plan on file at all".
type ArrivalPlanByDate map[timezone.Date]ArrivalWeek

// ArrivalPlansByStudent is the per-student projection.
type ArrivalPlansByStudent map[int64]ArrivalPlanByDate

// ArrivalBaselineProjection is the recurring arrival plan applicable on every
// date of a requested range.
type ArrivalBaselineProjection struct {
	WeeklyByStudentDate  ArrivalPlansByStudent
	DerivedByStudentDate ArrivalPlansByStudent
}

// ForDate returns the effective recurring row for the date's weekday. Day
// exceptions are deliberately outside this contract.
func (p *ArrivalBaselineProjection) ForDate(studentID int64, date timezone.Date) *scheduleModel.StudentArrivalSchedule {
	if p == nil {
		return nil
	}
	return p.WeeklyByStudentDate[studentID][date][effectiveISOWeekday(date)]
}

// DerivedForDate returns the projected row hidden underneath a manual
// override, if one exists for the date's weekday.
func (p *ArrivalBaselineProjection) DerivedForDate(studentID int64, date timezone.Date) *scheduleModel.StudentArrivalSchedule {
	if p == nil {
		return nil
	}
	return p.DerivedByStudentDate[studentID][date][effectiveISOWeekday(date)]
}

// HasPlan reports whether any recurring arrival weekday exists on the date.
func (p *ArrivalBaselineProjection) HasPlan(studentID int64, date timezone.Date) bool {
	return p != nil && len(p.WeeklyByStudentDate[studentID][date]) > 0
}

// WeeklyForDate returns the full recurring week applicable on date.
func (p *ArrivalBaselineProjection) WeeklyForDate(studentID int64, date timezone.Date) ArrivalWeek {
	if p == nil {
		return nil
	}
	return p.WeeklyByStudentDate[studentID][date]
}

// DerivedWeeklyForDate returns only the projected part of the plan. Write
// paths use it to avoid persisting a value the projection already produces.
func (p *ArrivalBaselineProjection) DerivedWeeklyForDate(studentID int64, date timezone.Date) ArrivalWeek {
	if p == nil {
		return nil
	}
	return p.DerivedByStudentDate[studentID][date]
}

// ArrivalBaselineReader is the single read boundary for regular arrival times.
type ArrivalBaselineReader interface {
	Project(ctx context.Context, studentIDs []int64, from, to timezone.Date) (*ArrivalBaselineProjection, error)
	DerivedArrivalForDate(ctx context.Context, studentID int64, date timezone.Date) (*scheduleModel.StudentArrivalSchedule, error)
}

type arrivalBaselineService struct {
	weekly     scheduleModel.StudentArrivalScheduleRepository
	students   users.StudentRepository
	classTimes educationModel.ClassArrivalTimeRepository
	links      enrollmentModel.RequestChildOfferingRepository
	offerings  enrollmentModel.CareOfferingRepository
	settings   config.SettingsService
}

// NewArrivalBaselineService builds the date-aware regular-arrival projection.
// settings may be nil (CLI, scheduler bootstraps without config): the booking
// mode then reads as off, which is the behaviour every school had before.
func NewArrivalBaselineService(
	weekly scheduleModel.StudentArrivalScheduleRepository,
	students users.StudentRepository,
	classTimes educationModel.ClassArrivalTimeRepository,
	links enrollmentModel.RequestChildOfferingRepository,
	offerings enrollmentModel.CareOfferingRepository,
	settings config.SettingsService,
) ArrivalBaselineReader {
	if weekly == nil || students == nil || classTimes == nil || links == nil || offerings == nil {
		panic("schedule.NewArrivalBaselineService: required dependency is nil")
	}
	return &arrivalBaselineService{
		weekly:     weekly,
		students:   students,
		classTimes: classTimes,
		links:      links,
		offerings:  offerings,
		settings:   settings,
	}
}

func (s *arrivalBaselineService) Project(
	ctx context.Context,
	studentIDs []int64,
	from, to timezone.Date,
) (*ArrivalBaselineProjection, error) {
	projection := &ArrivalBaselineProjection{
		WeeklyByStudentDate:  make(ArrivalPlansByStudent, len(studentIDs)),
		DerivedByStudentDate: make(ArrivalPlansByStudent, len(studentIDs)),
	}
	studentIDs = uniquePositiveStudentIDs(studentIDs)
	if len(studentIDs) == 0 || to.Before(from) {
		return projection, nil
	}

	stored, err := s.loadStoredRows(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	classByStudent, err := s.loadStudentClasses(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	timesByClass, err := s.loadClassTimes(ctx, classByStudent)
	if err != nil {
		return nil, err
	}
	careDays, err := s.loadCareDays(ctx, studentIDs, from, to)
	if err != nil {
		return nil, err
	}

	for _, studentID := range studentIDs {
		classTimes := timesByClass[schoolclass.Normalize(classByStudent[studentID])]
		weekly := make(ArrivalPlanByDate)
		derived := make(ArrivalPlanByDate)

		for date := from; !date.After(to); date = date.AddDays(1) {
			weekday := effectiveISOWeekday(date)
			if weekday > scheduleModel.WeekdayFriday {
				continue
			}
			row := stored[studentID][weekday]
			if !isCareDay(careDays, studentID, date, weekday, row) {
				continue
			}
			classRow := classArrivalRow(studentID, weekday, classTimes)
			if classRow != nil {
				derived[date] = ArrivalWeek{weekday: classRow}
			}
			if effective := effectiveArrivalRow(row, classRow); effective != nil {
				weekly[date] = ArrivalWeek{weekday: effective}
			}
		}

		projection.WeeklyByStudentDate[studentID] = weekly
		projection.DerivedByStudentDate[studentID] = derived
	}
	return projection, nil
}

func (s *arrivalBaselineService) DerivedArrivalForDate(
	ctx context.Context,
	studentID int64,
	date timezone.Date,
) (*scheduleModel.StudentArrivalSchedule, error) {
	projection, err := s.Project(ctx, []int64{studentID}, date, date)
	if err != nil {
		return nil, err
	}
	return projection.DerivedForDate(studentID, date), nil
}

// isCareDay decides whether the child is in care that day. With the booking
// mode on the booking links are the truth and a stored row cannot add a day;
// with it off the stored row IS the day, exactly as before #2414.
func isCareDay(
	careDays careDayIndex,
	studentID int64,
	date timezone.Date,
	weekday int,
	row *scheduleModel.StudentArrivalSchedule,
) bool {
	if careDays != nil {
		return careDays.covers(studentID, date, weekday)
	}
	return row != nil
}

// classArrivalRow builds the row the class timetable would supply, if any.
func classArrivalRow(
	studentID int64,
	weekday int,
	classTimes *educationModel.ClassArrivalTime,
) *scheduleModel.StudentArrivalSchedule {
	arrival, ok := classTimes.TimeForWeekday(weekday)
	if !ok {
		return nil
	}
	return &scheduleModel.StudentArrivalSchedule{
		StudentID:       studentID,
		Weekday:         weekday,
		ExpectedArrival: arrival,
		Source:          scheduleModel.ArrivalScheduleSourceClassSchedule,
		SourceClass:     classTimes.SchoolClass,
	}
}

// effectiveArrivalRow picks the row that is actually in force: a per-child
// deviation wins, a row without its own time takes the class time, and a care
// day whose class carries no time keeps the day without an arrival time.
func effectiveArrivalRow(
	row *scheduleModel.StudentArrivalSchedule,
	classRow *scheduleModel.StudentArrivalSchedule,
) *scheduleModel.StudentArrivalSchedule {
	if row != nil && !row.InheritsClassTime() {
		override := *row
		override.Source = scheduleModel.ArrivalScheduleSourceStaff
		return &override
	}
	if classRow != nil {
		return classRow
	}
	if row == nil {
		return nil
	}
	careDayOnly := *row
	careDayOnly.Source = scheduleModel.ArrivalScheduleSourceClassSchedule
	return &careDayOnly
}

// loadCareDayRows resolves, per student and date, the rows that are actually
// in force. Two independent facts meet here:
//
//	care day  — with the booking mode on, the approved booking links decide,
//	            so a stale row on an unbooked weekday is IGNORED rather than
//	            deleted (the same treatment ADR 0001 gives legacy pickup rows)
//	            and a newly booked weekday needs no manual row at all. With
//	            the mode off, the stored rows are the care days, exactly as
//	            before this change.
//	time      — the row's own value if it carries one, otherwise the class
//	            timetable. A care day whose class has no time yet stays a care
//	            day and simply has no arrival time.
func (s *arrivalBaselineService) loadStoredRows(
	ctx context.Context,
	studentIDs []int64,
) (map[int64]ArrivalWeek, error) {
	rows, err := s.weekly.FindByStudentIDs(ctx, studentIDs)
	if err != nil {
		return nil, fmt.Errorf("project arrival baselines: load stored schedules: %w", err)
	}
	out := make(map[int64]ArrivalWeek, len(studentIDs))
	for _, row := range rows {
		if row == nil {
			continue
		}
		if out[row.StudentID] == nil {
			out[row.StudentID] = make(ArrivalWeek)
		}
		out[row.StudentID][row.Weekday] = row
	}
	return out, nil
}

func (s *arrivalBaselineService) loadStudentClasses(
	ctx context.Context,
	studentIDs []int64,
) (map[int64]string, error) {
	students, err := s.students.FindByIDs(ctx, studentIDs)
	if err != nil {
		return nil, fmt.Errorf("project arrival baselines: load students: %w", err)
	}
	out := make(map[int64]string, len(students))
	for id, student := range students {
		if student != nil && strings.TrimSpace(student.SchoolClass) != "" {
			out[id] = student.SchoolClass
		}
	}
	return out, nil
}

func (s *arrivalBaselineService) loadClassTimes(
	ctx context.Context,
	classByStudent map[int64]string,
) (map[string]*educationModel.ClassArrivalTime, error) {
	classes := make([]string, 0, len(classByStudent))
	for _, class := range classByStudent {
		classes = append(classes, class)
	}
	if len(classes) == 0 {
		return nil, nil
	}
	rows, err := s.classTimes.FindByClasses(ctx, classes)
	if err != nil {
		return nil, fmt.Errorf("project arrival baselines: load class arrival times: %w", err)
	}
	out := make(map[string]*educationModel.ClassArrivalTime, len(rows))
	for _, row := range rows {
		if row != nil && len(row.ArrivalTimes) > 0 {
			out[schoolclass.Normalize(row.SchoolClass)] = row
		}
	}
	return out, nil
}

// careDayIndex answers "is this child in care on that date's weekday". A nil
// index means the booking mode is off and the stored rows are the care days.
type careDayIndex map[int64]map[timezone.Date]map[int]bool

func (c careDayIndex) covers(studentID int64, date timezone.Date, weekday int) bool {
	return c[studentID][date][weekday]
}

func (s *arrivalBaselineService) loadCareDays(
	ctx context.Context,
	studentIDs []int64,
	from, to timezone.Date,
) (careDayIndex, error) {
	authoritative, err := s.bookingsAuthoritative(ctx)
	if err != nil {
		return nil, err
	}
	if !authoritative {
		return nil, nil
	}

	links, err := s.links.ListApprovedByStudentIDsInRange(ctx, studentIDs, from, to)
	if err != nil {
		return nil, fmt.Errorf("project arrival baselines: load booking links: %w", err)
	}
	offeringByID, err := s.loadOfferingsByID(ctx, links)
	if err != nil {
		return nil, err
	}

	index := make(careDayIndex, len(studentIDs))
	for _, entry := range links {
		if entry == nil || entry.Link == nil {
			continue
		}
		offering := offeringByID[entry.Link.CareOfferingID]
		if offering == nil || !offering.IsActive || !offering.CountsAsCare {
			continue
		}
		for date := from; !date.After(to); date = date.AddDays(1) {
			if !offeringLinkCovers(entry.Link, date) {
				continue
			}
			for _, day := range offeringPickupDays(entry.Link, offering) {
				weekday, ok := enrollmentModel.CanonicalDayToISOWeekday(strings.ToLower(strings.TrimSpace(day)))
				if !ok || weekday > scheduleModel.WeekdayFriday {
					continue
				}
				if index[entry.StudentID] == nil {
					index[entry.StudentID] = make(map[timezone.Date]map[int]bool)
				}
				if index[entry.StudentID][date] == nil {
					index[entry.StudentID][date] = make(map[int]bool)
				}
				index[entry.StudentID][date][weekday] = true
			}
		}
	}
	return index, nil
}

func (s *arrivalBaselineService) bookingsAuthoritative(ctx context.Context) (bool, error) {
	if s.settings == nil {
		return false, nil
	}
	authoritative, err := s.settings.ResolveBool(ctx, configModel.KeyEnrollmentBookingsAuthoritative)
	if err != nil {
		return false, fmt.Errorf("project arrival baselines: resolve booking mode: %w", err)
	}
	return authoritative, nil
}

func (s *arrivalBaselineService) loadOfferingsByID(
	ctx context.Context,
	links []*enrollmentModel.ApprovedOfferingChild,
) (map[int64]*enrollmentModel.CareOffering, error) {
	offeringIDs := make([]int64, 0, len(links))
	for _, entry := range links {
		if entry != nil && entry.Link != nil {
			offeringIDs = append(offeringIDs, entry.Link.CareOfferingID)
		}
	}
	offerings, err := s.offerings.ListByIDs(ctx, sliceutil.UniquePositive(offeringIDs))
	if err != nil {
		return nil, fmt.Errorf("project arrival baselines: load care offerings: %w", err)
	}
	out := make(map[int64]*enrollmentModel.CareOffering, len(offerings))
	for _, offering := range offerings {
		if offering != nil {
			out[offering.ID] = offering
		}
	}
	return out, nil
}
