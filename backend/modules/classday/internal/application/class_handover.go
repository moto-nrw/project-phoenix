package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Ports of the class handover (#4794). Student Presence stores the handover;
// the school's write scope decides whether moto schule may write it.
type (
	// ClassHandovers reads and writes Student Presence's class handovers.
	ClassHandovers interface {
		ClassHandoversOn(ctx context.Context, date calendar.Date) ([]studentpresence.ClassHandover, error)
		ChangeClassHandover(ctx context.Context, in studentpresence.ClassHandoverChange) (bool, error)
	}
	// ClassHandoverScope applies operations.school_portal_write_scope: true
	// while the school ticks the class handover.
	ClassHandoverScope func(ctx context.Context) (bool, error)
	// ClassHandoverStaffNames maps staff ids to display names in one read;
	// an unknown staff member has no entry.
	ClassHandoverStaffNames func(ctx context.Context, staffIDs []int64) (map[int64]string, error)
	// ClassHandoverAnnouncer tells the OGS views, once the caller's
	// transaction committed, that a class's handover changed. A handover is
	// the class's arrival in the OGS, so the root binds it to the
	// arrival-schedule event Aufsicht, Gruppe and Kindersuche refetch on.
	ClassHandoverAnnouncer func(ctx context.Context)
)

// ClassHandoverDependencies wire the class handover (#4794). Without
// Handovers or HandoverScope the report shows no handover, the write verdict
// stays false and the write answers classday.ErrClassHandoverNotConfigured.
// Without HandoverStaffNames the report names nobody; without
// HandoverAnnouncer no OGS view is told.
type ClassHandoverDependencies struct {
	Handovers          ClassHandovers
	HandoverScope      ClassHandoverScope
	HandoverStaffNames ClassHandoverStaffNames
	HandoverAnnouncer  ClassHandoverAnnouncer
}

// classHandover is the third write seam of the class-day view (#4794): the
// Lehrkraft hands her class over to the OGS after lessons and marks the
// children with an OGS record who are not there. The handover lives in
// Student Presence apart from the morning attendance and the OGS attendance.
type classHandover struct {
	store     ClassHandovers
	scope     ClassHandoverScope
	names     ClassHandoverStaffNames
	announcer ClassHandoverAnnouncer
	rosters   DayRosters
	today     func() calendar.Date
}

func newClassHandover(deps ClassDayDependencies) classHandover {
	today := calendar.TodayDate
	if now := deps.Now; now != nil {
		today = func() calendar.Date { return calendar.DateFromTime(now()) }
	}
	return classHandover{
		store: deps.Handovers, scope: deps.HandoverScope, names: deps.HandoverStaffNames,
		announcer: deps.HandoverAnnouncer, rosters: deps.Rosters, today: today,
	}
}

// mayHandOver applies the write scope. Without the store or the scope the
// verdict is false.
func (h classHandover) mayHandOver(ctx context.Context) (bool, error) {
	if h.store == nil || h.scope == nil {
		return false, nil
	}
	may, err := h.scope(ctx)
	if err != nil {
		return false, fmt.Errorf("class day handover: resolve write scope: %w", err)
	}
	return may, nil
}

// ChangeClassHandover hands the class over or takes today's handover back
// after the write scope, date and school-day checks; a handover also checks
// every absent child against the roster of the day.
func (h classHandover) ChangeClassHandover(ctx context.Context, in classday.ClassHandoverChange) error {
	if h.store == nil || h.scope == nil || h.rosters == nil {
		return classday.ErrClassHandoverNotConfigured
	}
	if in.SchoolClass == "" || (!in.Withdraw && in.HandedOverBy <= 0) {
		return classday.ErrClassHandoverInvalid
	}
	date, err := h.checkDay(ctx, in.Date)
	if err != nil {
		return err
	}
	if in.Withdraw {
		return h.withdraw(ctx, in.SchoolClass, date)
	}
	roster, err := h.rosters.ClassRosterDay(ctx, in.SchoolClass, date)
	if err != nil {
		return err
	}
	if err := checkHandoverChildren(roster, in.AbsentStudentIDs); err != nil {
		return err
	}
	_, err = h.store.ChangeClassHandover(ctx, studentpresence.ClassHandoverChange{
		SchoolClass: in.SchoolClass, Date: date, HandedOverBy: in.HandedOverBy, AbsentStudentIDs: in.AbsentStudentIDs,
	})
	if errors.Is(err, studentpresence.ErrClassHandoverInvalid) {
		return fmt.Errorf("%w: %w", classday.ErrClassHandoverInvalid, err)
	}
	if err != nil {
		return err
	}
	h.announce(ctx)
	return nil
}

// checkDay applies the write scope, today and the school day.
func (h classHandover) checkDay(ctx context.Context, value classday.Date) (calendar.Date, error) {
	may, err := h.mayHandOver(ctx)
	if err != nil {
		return "", err
	}
	if !may {
		return "", classday.ErrClassHandoverDisabled
	}
	date, err := parseDate(value)
	if err != nil {
		return "", fmt.Errorf("%w: %w", classday.ErrClassHandoverInvalid, err)
	}
	if date != h.today() {
		return "", classday.ErrClassHandoverNotToday
	}
	weekday, err := classDayPlanKey(ctx, date)
	if err != nil {
		return "", err
	}
	if weekday == "" {
		return "", classday.ErrClassHandoverNoSchoolDay
	}
	return date, nil
}

// withdraw takes today's handover back.
func (h classHandover) withdraw(ctx context.Context, schoolClass string, date calendar.Date) error {
	existed, err := h.store.ChangeClassHandover(ctx, studentpresence.ClassHandoverChange{SchoolClass: schoolClass, Date: date, Withdraw: true})
	if err != nil {
		return err
	}
	if !existed {
		return classday.ErrClassHandoverNotFound
	}
	h.announce(ctx)
	return nil
}

// announce tells the OGS views that the handover changed.
func (h classHandover) announce(ctx context.Context) {
	if h.announcer != nil {
		h.announcer(ctx)
	}
}

// checkHandoverChildren refuses an absent child without an OGS record on the
// roster of the day.
func checkHandoverChildren(roster *DayRoster, absent []int64) error {
	onRoster := map[int64]bool{}
	if roster != nil {
		for _, row := range roster.Rows {
			if !row.ListEntry && row.StudentID > 0 {
				onRoster[row.StudentID] = true
			}
		}
	}
	for _, id := range absent {
		if id <= 0 {
			return classday.ErrClassHandoverInvalid
		}
		if !onRoster[id] {
			return classday.ErrClassHandoverUnknownChild
		}
	}
	return nil
}

// applyClassHandover fills in the report's handover, the rows marked "nicht
// da" and the write verdict on a school day.
func (h classHandover) applyClassHandover(ctx context.Context, report *classday.DayReport, date calendar.Date) error {
	if !report.SchoolDay {
		return nil
	}
	if h.store != nil {
		stored, err := h.handoverOf(ctx, report.SchoolClass, date)
		if err != nil {
			return err
		}
		if stored != nil {
			handover, err := h.dayHandover(ctx, stored)
			if err != nil {
				return err
			}
			report.Handover = handover
			markHandoverAbsent(report, stored.AbsentStudentIDs)
		}
	}
	if date != h.today() {
		return nil
	}
	may, err := h.mayHandOver(ctx)
	if err != nil {
		return err
	}
	report.CanHandOver = may
	return nil
}

// handoverOf picks the class's handover among the handovers of date; class
// names match trimmed and lowercased.
func (h classHandover) handoverOf(ctx context.Context, schoolClass string, date calendar.Date) (*studentpresence.ClassHandover, error) {
	handovers, err := h.store.ClassHandoversOn(ctx, date)
	if err != nil {
		return nil, fmt.Errorf("class day report: load class handovers: %w", err)
	}
	class := strings.ToLower(strings.TrimSpace(schoolClass))
	for i := range handovers {
		if strings.ToLower(strings.TrimSpace(handovers[i].SchoolClass)) == class {
			return &handovers[i], nil
		}
	}
	return nil, nil
}

// dayHandover names the staff member of a stored handover.
func (h classHandover) dayHandover(ctx context.Context, stored *studentpresence.ClassHandover) (*classday.DayHandover, error) {
	handover := &classday.DayHandover{HandedOverAt: stored.HandedOverAt.In(calendar.Berlin), AbsentCount: len(stored.AbsentStudentIDs)}
	if h.names == nil || stored.HandedOverByStaffID <= 0 {
		return handover, nil
	}
	names, err := h.names(ctx, []int64{stored.HandedOverByStaffID})
	if err != nil {
		return nil, fmt.Errorf("class day report: name the handing-over staff member: %w", err)
	}
	handover.HandedOverBy = names[stored.HandedOverByStaffID]
	return handover, nil
}

func markHandoverAbsent(report *classday.DayReport, absent []int64) {
	marked := make(map[int64]bool, len(absent))
	for _, id := range absent {
		marked[id] = true
	}
	for i := range report.Rows {
		row := &report.Rows[i]
		if !row.ListEntry && marked[row.StudentID] {
			row.HandoverAbsent = true
		}
	}
}
