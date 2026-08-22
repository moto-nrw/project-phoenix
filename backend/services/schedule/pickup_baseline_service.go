package schedule

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/sliceutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModel "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
)

// PickupBaselineProjection is the recurring pickup plan applicable on every
// date in a requested range. WeeklyByStudentDate keeps all weekdays because a
// care-day read must distinguish "no pickup on Monday" from "no plan on file".
type PickupWeek map[int]*scheduleModel.StudentPickupSchedule
type PickupPlanByDate map[timezone.Date]PickupWeek
type PickupPlansByStudent map[int64]PickupPlanByDate

type PickupBaselineProjection struct {
	WeeklyByStudentDate   PickupPlansByStudent
	OfferingByStudentDate PickupPlansByStudent
}

// ForDate returns the effective recurring row for the date's weekday. Day
// exceptions are deliberately outside this contract.
func (p *PickupBaselineProjection) ForDate(studentID int64, date timezone.Date) *scheduleModel.StudentPickupSchedule {
	if p == nil {
		return nil
	}
	return p.WeeklyByStudentDate[studentID][date][effectiveISOWeekday(date)]
}

// OfferingForDate returns the booking-derived row hidden underneath a manual
// override, if one exists for the date's weekday.
func (p *PickupBaselineProjection) OfferingForDate(studentID int64, date timezone.Date) *scheduleModel.StudentPickupSchedule {
	if p == nil {
		return nil
	}
	return p.OfferingByStudentDate[studentID][date][effectiveISOWeekday(date)]
}

// HasPlan reports whether any recurring pickup weekday exists on the date.
func (p *PickupBaselineProjection) HasPlan(studentID int64, date timezone.Date) bool {
	return p != nil && len(p.WeeklyByStudentDate[studentID][date]) > 0
}

// WeeklyForDate returns the full recurring week applicable on date.
func (p *PickupBaselineProjection) WeeklyForDate(studentID int64, date timezone.Date) PickupWeek {
	if p == nil {
		return nil
	}
	return p.WeeklyByStudentDate[studentID][date]
}

// OfferingWeeklyForDate returns only the booking-derived part of the plan.
// Write paths use it to avoid persisting an unchanged projected value.
func (p *PickupBaselineProjection) OfferingWeeklyForDate(studentID int64, date timezone.Date) PickupWeek {
	if p == nil {
		return nil
	}
	return p.OfferingByStudentDate[studentID][date]
}

// PickupBaselineReader is the single read boundary for regular pickup times.
// Stored staff rows override booking-derived offering rows; legacy materialized
// care_offering rows are ignored.
type PickupBaselineReader interface {
	Project(ctx context.Context, studentIDs []int64, from, to timezone.Date) (*PickupBaselineProjection, error)
	OfferingPickupForDate(ctx context.Context, studentID int64, date timezone.Date) (*scheduleModel.StudentPickupSchedule, error)
}

type pickupBaselineService struct {
	weekly    scheduleModel.StudentPickupScheduleRepository
	links     enrollmentModel.RequestChildOfferingRepository
	offerings enrollmentModel.CareOfferingRepository
}

// NewPickupBaselineService builds the date-aware regular-pickup projection.
func NewPickupBaselineService(
	weekly scheduleModel.StudentPickupScheduleRepository,
	links enrollmentModel.RequestChildOfferingRepository,
	offerings enrollmentModel.CareOfferingRepository,
) PickupBaselineReader {
	if weekly == nil || links == nil || offerings == nil {
		panic("schedule.NewPickupBaselineService: required dependency is nil")
	}
	return &pickupBaselineService{weekly: weekly, links: links, offerings: offerings}
}

func (s *pickupBaselineService) Project(
	ctx context.Context,
	studentIDs []int64,
	from, to timezone.Date,
) (*PickupBaselineProjection, error) {
	projection := &PickupBaselineProjection{
		WeeklyByStudentDate:   make(PickupPlansByStudent, len(studentIDs)),
		OfferingByStudentDate: make(PickupPlansByStudent, len(studentIDs)),
	}
	studentIDs = uniquePositiveStudentIDs(studentIDs)
	if len(studentIDs) == 0 || to.Before(from) {
		return projection, nil
	}

	manual, err := s.loadManualRows(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	offering, err := s.loadOfferingRows(ctx, studentIDs, from, to)
	if err != nil {
		return nil, err
	}

	for _, studentID := range studentIDs {
		byDate := make(PickupPlanByDate)
		for date := from; !date.After(to); date = date.AddDays(1) {
			byWeekday := make(PickupWeek, len(manual[studentID])+len(offering[studentID][date]))
			for weekday, row := range offering[studentID][date] {
				byWeekday[weekday] = row
			}
			for weekday, row := range manual[studentID] {
				byWeekday[weekday] = row
			}
			byDate[date] = byWeekday
		}
		projection.WeeklyByStudentDate[studentID] = byDate
		projection.OfferingByStudentDate[studentID] = offering[studentID]
	}
	return projection, nil
}

func (s *pickupBaselineService) OfferingPickupForDate(
	ctx context.Context,
	studentID int64,
	date timezone.Date,
) (*scheduleModel.StudentPickupSchedule, error) {
	projection, err := s.Project(ctx, []int64{studentID}, date, date)
	if err != nil {
		return nil, err
	}
	return projection.OfferingForDate(studentID, date), nil
}

func (s *pickupBaselineService) loadManualRows(
	ctx context.Context,
	studentIDs []int64,
) (map[int64]map[int]*scheduleModel.StudentPickupSchedule, error) {
	rows, err := s.weekly.FindByStudentIDs(ctx, studentIDs)
	if err != nil {
		return nil, fmt.Errorf("project pickup baselines: load stored schedules: %w", err)
	}
	out := make(map[int64]map[int]*scheduleModel.StudentPickupSchedule, len(studentIDs))
	for _, row := range rows {
		if row == nil || row.Source == scheduleModel.PickupScheduleSourceCareOffering {
			continue
		}
		if out[row.StudentID] == nil {
			out[row.StudentID] = make(map[int]*scheduleModel.StudentPickupSchedule)
		}
		out[row.StudentID][row.Weekday] = row
	}
	return out, nil
}

func (s *pickupBaselineService) loadOfferingRows(
	ctx context.Context,
	studentIDs []int64,
	from, to timezone.Date,
) (PickupPlansByStudent, error) {
	links, err := s.links.ListApprovedByStudentIDsInRange(ctx, studentIDs, from, to)
	if err != nil {
		return nil, fmt.Errorf("project pickup baselines: load booking links: %w", err)
	}
	offeringByID, err := s.loadOfferingsByID(ctx, links)
	if err != nil {
		return nil, err
	}
	return projectOfferingLinks(links, offeringByID, from, to)
}

func (s *pickupBaselineService) loadOfferingsByID(
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
		return nil, fmt.Errorf("project pickup baselines: load care offerings: %w", err)
	}
	offeringByID := make(map[int64]*enrollmentModel.CareOffering, len(offerings))
	for _, offering := range offerings {
		if offering != nil {
			offeringByID[offering.ID] = offering
		}
	}
	return offeringByID, nil
}

func projectOfferingLinks(
	links []*enrollmentModel.ApprovedOfferingChild,
	offeringByID map[int64]*enrollmentModel.CareOffering,
	from, to timezone.Date,
) (PickupPlansByStudent, error) {
	out := make(PickupPlansByStudent)
	for _, entry := range links {
		if entry == nil || entry.Link == nil {
			continue
		}
		offering := offeringByID[entry.Link.CareOfferingID]
		if offering == nil || !offering.IsActive || !offering.CountsAsCare || len(offering.PickupTimes) == 0 {
			continue
		}
		for date := from; !date.After(to); date = date.AddDays(1) {
			if !offeringLinkCovers(entry.Link, date) {
				continue
			}
			if err := projectOfferingWeek(out, entry.StudentID, date, entry.Link, offering); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}

func projectOfferingWeek(
	out PickupPlansByStudent,
	studentID int64,
	date timezone.Date,
	link *enrollmentModel.RequestChildOffering,
	offering *enrollmentModel.CareOffering,
) error {
	for _, day := range offeringPickupDays(link, offering) {
		weekday, row, ok, err := projectedOfferingPickup(studentID, day, offering)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		if out[studentID] == nil {
			out[studentID] = make(PickupPlanByDate)
		}
		if out[studentID][date] == nil {
			out[studentID][date] = make(PickupWeek)
		}
		current := out[studentID][date][weekday]
		if current != nil && !timezone.WallClock(current.PickupTime).Before(row.PickupTime) {
			continue
		}
		out[studentID][date][weekday] = row
	}
	return nil
}

func offeringPickupDays(
	link *enrollmentModel.RequestChildOffering,
	offering *enrollmentModel.CareOffering,
) []string {
	if len(link.SelectedDays) > 0 || offering.DaysOfWeekMode != enrollmentModel.DaysOfWeekModeFixed {
		return link.SelectedDays
	}
	return offering.AvailableDays
}

func projectedOfferingPickup(
	studentID int64,
	day string,
	offering *enrollmentModel.CareOffering,
) (int, *scheduleModel.StudentPickupSchedule, bool, error) {
	key := strings.ToLower(strings.TrimSpace(day))
	hhmm := offering.PickupTimes[key]
	weekday, ok := enrollmentModel.CanonicalDayToISOWeekday(key)
	if !ok || weekday > scheduleModel.WeekdayFriday || hhmm == "" {
		return 0, nil, false, nil
	}
	parsed, err := time.Parse("15:04", hhmm)
	if err != nil {
		return 0, nil, false, fmt.Errorf("project pickup baselines: offering %d has invalid pickup time %q for %s: %w", offering.ID, hhmm, key, err)
	}
	offeringID := offering.ID
	return weekday, &scheduleModel.StudentPickupSchedule{
		StudentID: studentID, Weekday: weekday, PickupTime: timezone.WallClock(parsed),
		Source: scheduleModel.PickupScheduleSourceCareOffering, CareOfferingID: &offeringID,
		CareOfferingName: offering.Name,
	}, true, nil
}

func offeringLinkCovers(link *enrollmentModel.RequestChildOffering, date timezone.Date) bool {
	return link != nil &&
		(link.ValidFrom == nil || !date.Before(*link.ValidFrom)) &&
		(link.ValidUntil == nil || date.Before(*link.ValidUntil))
}

func uniquePositiveStudentIDs(ids []int64) []int64 {
	out := sliceutil.UniquePositive(ids)
	slices.Sort(out)
	return out
}
