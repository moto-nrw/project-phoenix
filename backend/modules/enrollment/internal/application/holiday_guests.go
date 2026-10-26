package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Holiday guests of the hosting school (#4666, ADR 0120). The caller runs
// every operation in the tenant transaction of the hosting school. The store
// reads only the confirmed guests of weeks the school hosts until their last
// day; this service decides by the calendar day of its clock which guest the
// school still cares for and when the read-only days are over.

// HolidayGuestStore reads and writes the holiday guests in the tenant
// transaction of the hosting school.
type HolidayGuestStore interface {
	// LockGuests serializes the reconciliations of the school until the
	// transaction ends, so the hourly run and a group choice cannot copy the
	// same guest twice.
	LockGuests(ctx context.Context, schoolID int64) error
	SetHostGroup(ctx context.Context, schoolID int64, group enrollment.HolidayHostGroup) error
	ListHostGroups(ctx context.Context, schoolID int64) ([]enrollment.HolidayHostGroup, error)
	// ReadGuestWeek reads a hosted week of a published period that ends on
	// today or later; found is false otherwise.
	ReadGuestWeek(ctx context.Context, schoolID, weekID int64, today calendar.Date) (enrollment.HolidayGuestWeek, bool, error)
	// ListGuestSources reads the confirmed children of the week who are no
	// students of the school.
	ListGuestSources(ctx context.Context, schoolID int64, week enrollment.HolidayGuestWeek) ([]enrollment.HolidayGuestSource, error)
	// ListOpenGuests lists the guests whose access has not ended early, of
	// one week or, with weekID 0, of every week ending on from or later.
	ListOpenGuests(ctx context.Context, schoolID, weekID int64, from calendar.Date) ([]enrollment.HolidayGuest, error)
	// ListWeekGuests lists every guest of one week, also those whose access
	// ended early.
	ListWeekGuests(ctx context.Context, schoolID, weekID int64) ([]enrollment.HolidayGuest, error)
	// ReopenGuest clears the early end of a guest whose booking came back.
	ReopenGuest(ctx context.Context, schoolID, guestID int64) error
	InsertGuest(ctx context.Context, schoolID int64, guest enrollment.HolidayGuest) error
	UpdateGuest(ctx context.Context, schoolID int64, guest enrollment.HolidayGuest) error
	EndGuest(ctx context.Context, schoolID, guestID int64, endedOn calendar.Date) error
	PurgeContacts(ctx context.Context, schoolID int64, lastReadableEnd calendar.Date) error
	ReadGuest(ctx context.Context, schoolID, studentID int64, lastReadableEnd calendar.Date) (enrollment.HolidayGuest, bool, error)
}

// HolidayGuests serves enrollment.HolidayGuests.
type HolidayGuests struct {
	store  HolidayGuestStore
	people enrollment.HolidayGuestPeople
	care   enrollment.HolidayGuestCareDays
	log    enrollment.HolidayGuestAccessLog
	today  func() calendar.Date
	logger *slog.Logger
}

var _ enrollment.HolidayGuests = (*HolidayGuests)(nil)

// NewHolidayGuests builds the holiday guests; a nil today uses the Berlin
// calendar day.
func NewHolidayGuests(store HolidayGuestStore, people enrollment.HolidayGuestPeople, care enrollment.HolidayGuestCareDays, log enrollment.HolidayGuestAccessLog, today func() calendar.Date) *HolidayGuests {
	if store == nil || people == nil || care == nil || log == nil {
		panic("enrollment: the holiday guest store, People Directory, Care Plan and the access log are required")
	}
	if today == nil {
		today = calendar.TodayDate
	}
	return &HolidayGuests{store: store, people: people, care: care, log: log, today: today, logger: slog.Default().With("service", "holiday-guests")}
}

// AssignHolidayGuestGroup stores the group of the week and copies the
// week's guests into it in the same transaction.
func (h *HolidayGuests) AssignHolidayGuestGroup(ctx context.Context, host enrollment.HolidayHost, weekID, groupID int64) (enrollment.HolidayGuestTransfer, error) {
	if host.SchoolID <= 0 || host.AccountID <= 0 {
		return enrollment.HolidayGuestTransfer{}, errors.New("enrollment: the hosting school and the account are required")
	}
	if weekID <= 0 {
		return enrollment.HolidayGuestTransfer{}, enrollment.ErrHostedHolidayWeekNotFound
	}
	if groupID <= 0 {
		return enrollment.HolidayGuestTransfer{}, errors.New("enrollment: the group is required")
	}
	if err := h.store.LockGuests(ctx, host.SchoolID); err != nil {
		return enrollment.HolidayGuestTransfer{}, err
	}
	today := h.today()
	week, found, err := h.store.ReadGuestWeek(ctx, host.SchoolID, weekID, today)
	if err != nil {
		return enrollment.HolidayGuestTransfer{}, err
	}
	if !found {
		return enrollment.HolidayGuestTransfer{}, enrollment.ErrHostedHolidayWeekNotFound
	}
	group := enrollment.HolidayHostGroup{OrganizationID: week.OrganizationID, WeekID: week.ID, GroupID: groupID, AssignedByAccountID: host.AccountID}
	if err := h.store.SetHostGroup(ctx, host.SchoolID, group); err != nil {
		return enrollment.HolidayGuestTransfer{}, err
	}
	// A group choice replans every guest of the week: changes that came while
	// the week had no group are written now.
	guests, err := h.reconcileWeek(ctx, host.SchoolID, group, today, true)
	if err != nil {
		return enrollment.HolidayGuestTransfer{}, err
	}
	if err := h.log.RecordHolidayGuestAccess(ctx, enrollment.HolidayGuestAccess{
		Kind: enrollment.HolidayGuestAccessTransfer, ActorAccountID: host.AccountID, ActorRole: host.Role,
		WeekID: week.ID, StartsOn: week.StartsOn, EndsOn: week.EndsOn, Guests: guests,
	}); err != nil {
		return enrollment.HolidayGuestTransfer{}, fmt.Errorf("record holiday guest transfer: %w", err)
	}
	return enrollment.HolidayGuestTransfer{WeekID: week.ID, GroupID: groupID, Guests: guests}, nil
}

// ReconcileHolidayGuests reconciles every week the school picked a group
// for, ends the guests of weeks whose access ended otherwise, and deletes
// the contacts past the read-only days. A transfer that copied or ended a
// guest is recorded under the account that picked the group.
func (h *HolidayGuests) ReconcileHolidayGuests(ctx context.Context, schoolID int64) error {
	if schoolID <= 0 {
		return errors.New("enrollment: the hosting school is required")
	}
	if err := h.store.LockGuests(ctx, schoolID); err != nil {
		return err
	}
	today := h.today()
	groups, err := h.store.ListHostGroups(ctx, schoolID)
	if err != nil {
		return err
	}
	reconciled := make(map[int64]bool, len(groups))
	for _, group := range groups {
		reconciled[group.WeekID] = true
		if err := h.reconcileAndRecord(ctx, schoolID, group, today); err != nil {
			return err
		}
	}
	// A week without a group (its group was deleted) keeps its guests in
	// care without a group until its access ends; ending them is still due.
	open, err := h.store.ListOpenGuests(ctx, schoolID, 0, today)
	if err != nil {
		return err
	}
	for _, guest := range open {
		if reconciled[guest.WeekID] {
			continue
		}
		reconciled[guest.WeekID] = true
		if _, err := h.reconcileWeek(ctx, schoolID, enrollment.HolidayHostGroup{OrganizationID: guest.OrganizationID, WeekID: guest.WeekID}, today, false); err != nil {
			return err
		}
	}
	return h.store.PurgeContacts(ctx, schoolID, today.AddDays(-enrollment.HolidayGuestReadOnlyDays))
}

func (h *HolidayGuests) reconcileAndRecord(ctx context.Context, schoolID int64, group enrollment.HolidayHostGroup, today calendar.Date) error {
	week, found, err := h.store.ReadGuestWeek(ctx, schoolID, group.WeekID, today)
	if err != nil {
		return err
	}
	changes, err := h.reconcile(ctx, schoolID, group, week, found, today, false)
	if err != nil {
		return err
	}
	if changes.copied == 0 || !found {
		return nil
	}
	if err := h.log.RecordHolidayGuestAccess(ctx, enrollment.HolidayGuestAccess{
		Kind: enrollment.HolidayGuestAccessTransfer, ActorAccountID: group.AssignedByAccountID, ActorRole: "system",
		WeekID: week.ID, StartsOn: week.StartsOn, EndsOn: week.EndsOn, Guests: changes.guests, Automatic: true,
	}); err != nil {
		return fmt.Errorf("record holiday guest transfer: %w", err)
	}
	return nil
}

// reconcileWeek reconciles one week and returns how many guests it holds.
func (h *HolidayGuests) reconcileWeek(ctx context.Context, schoolID int64, group enrollment.HolidayHostGroup, today calendar.Date, replanAll bool) (int, error) {
	week, found, err := h.store.ReadGuestWeek(ctx, schoolID, group.WeekID, today)
	if err != nil {
		return 0, err
	}
	changes, err := h.reconcile(ctx, schoolID, group, week, found, today, replanAll)
	return changes.guests, err
}

type guestChanges struct {
	guests int
	copied int
}

// reconcile brings the guests of one week in line with its confirmed
// bookings. A week the school no longer hosts, or whose period is no
// longer published, ends every guest still in care; a guest whose booking
// went ends as well. Without a group no new guest is copied.
func (h *HolidayGuests) reconcile(ctx context.Context, schoolID int64, group enrollment.HolidayHostGroup, week enrollment.HolidayGuestWeek, found bool, today calendar.Date, replanAll bool) (guestChanges, error) {
	// An open week reads its ended guests too: a booking that comes back
	// reopens its guest instead of copying the child a second time.
	var existing []enrollment.HolidayGuest
	var sources []enrollment.HolidayGuestSource
	var err error
	if found {
		if existing, err = h.store.ListWeekGuests(ctx, schoolID, group.WeekID); err != nil {
			return guestChanges{}, err
		}
		if sources, err = h.store.ListGuestSources(ctx, schoolID, week); err != nil {
			return guestChanges{}, err
		}
	} else if existing, err = h.store.ListOpenGuests(ctx, schoolID, group.WeekID, today); err != nil {
		return guestChanges{}, err
	}
	byChild := make(map[int64]enrollment.HolidayGuest, len(existing))
	for _, guest := range existing {
		byChild[guest.ChildID] = guest
	}
	changes, err := h.applySources(ctx, schoolID, sources, byChild, week, group, replanAll, today)
	if err != nil {
		return guestChanges{}, err
	}
	// What is left has no confirmed booking in the week any more.
	for _, gone := range byChild {
		if gone.EndedOn != nil {
			continue
		}
		if err := h.endGuest(ctx, schoolID, gone, today); err != nil {
			return guestChanges{}, err
		}
	}
	return changes, nil
}

// applySources refreshes the guests that still have a confirmed booking and
// copies the new ones, removing each handled guest from byChild.
func (h *HolidayGuests) applySources(ctx context.Context, schoolID int64, sources []enrollment.HolidayGuestSource, byChild map[int64]enrollment.HolidayGuest, week enrollment.HolidayGuestWeek, group enrollment.HolidayHostGroup, replanAll bool, today calendar.Date) (guestChanges, error) {
	var groupID *int64
	if group.GroupID > 0 {
		groupID = &group.GroupID
	}
	changes := guestChanges{}
	for _, source := range sources {
		guest, ok := byChild[source.ChildID]
		delete(byChild, source.ChildID)
		switch {
		case ok && guest.EndedOn == nil:
			if err := h.refreshGuest(ctx, schoolID, guest, source, week, groupID, group.AssignedByAccountID, replanAll, today); err != nil {
				return guestChanges{}, err
			}
		case groupID == nil:
			continue
		case ok:
			if err := h.reopenGuest(ctx, schoolID, guest, source, week, groupID, group.AssignedByAccountID, today); err != nil {
				return guestChanges{}, err
			}
			changes.copied++
		default:
			if err := h.copyGuest(ctx, schoolID, source, week, groupID, group.AssignedByAccountID, today); err != nil {
				return guestChanges{}, err
			}
			changes.copied++
		}
		changes.guests++
	}
	return changes, nil
}

func (h *HolidayGuests) copyGuest(ctx context.Context, schoolID int64, source enrollment.HolidayGuestSource, week enrollment.HolidayGuestWeek, groupID *int64, actor int64, today calendar.Date) error {
	studentID, err := h.people.CreateHolidayGuestChild(ctx, guestChild(source, week, groupID, today))
	if err != nil {
		return fmt.Errorf("create the child of a holiday guest: %w", err)
	}
	if err := h.store.InsertGuest(ctx, schoolID, guestRecord(enrollment.HolidayGuest{
		OrganizationID: week.OrganizationID, WeekID: week.ID, ChildID: source.ChildID, StudentID: studentID,
	}, source, week)); err != nil {
		return err
	}
	return h.planDays(ctx, studentID, actor, source, week)
}

// planDays writes the booked care of each day of the week into Care Plan.
func (h *HolidayGuests) planDays(ctx context.Context, studentID, actor int64, source enrollment.HolidayGuestSource, week enrollment.HolidayGuestWeek) error {
	days := make([]calendar.Date, 0, 7)
	for day := week.StartsOn; !day.After(week.EndsOn); day = day.AddDays(1) {
		days = append(days, day)
	}
	err := h.care.PlanHolidayGuestDays(ctx, studentID, actor, days, source.TimeModelStarts, source.TimeModelEnds)
	if errors.Is(err, enrollment.ErrHolidayGuestPlanAuthorMissing) {
		// The account that picked the group has no staff record (any more):
		// the guest still joins the group, only its days stay unplanned
		// until the school picks the group again. The reconciliation of the
		// other guests and the deletion of expired contacts go on.
		h.logger.Warn("holiday guest days left unplanned: the account that picked the group has no staff record",
			"student_id", studentID,
			"account_id", actor,
		)
		return nil
	}
	if err != nil {
		return fmt.Errorf("plan the care days of a holiday guest: %w", err)
	}
	return nil
}

// refreshGuest follows a change of the registration (names, time model,
// contacts) or of the group and writes the child only when it differs.
func (h *HolidayGuests) refreshGuest(ctx context.Context, schoolID int64, guest enrollment.HolidayGuest, source enrollment.HolidayGuestSource, week enrollment.HolidayGuestWeek, groupID *int64, actor int64, replanDays bool, today calendar.Date) error {
	current, err := h.people.ReadHolidayGuestChild(ctx, guest.StudentID)
	if err != nil {
		return fmt.Errorf("read the child of a holiday guest: %w", err)
	}
	if groupID == nil {
		groupID = current.GroupID
	}
	wanted := guestChild(source, week, groupID, today)
	if !sameGuestChild(current, wanted) {
		if err := h.people.UpdateHolidayGuestChild(ctx, guest.StudentID, wanted); err != nil {
			return fmt.Errorf("update the child of a holiday guest: %w", err)
		}
	}
	// A rebooked time model or moved week replans the days; a week without a
	// group keeps its plan until someone picks a group again.
	replan := replanDays || guest.StartsOn != week.StartsOn || guest.EndsOn != week.EndsOn ||
		guest.TimeModelStarts != source.TimeModelStarts || guest.TimeModelEnds != source.TimeModelEnds
	if replan && actor > 0 {
		if err := h.care.ClearHolidayGuestDays(ctx, guest.StudentID); err != nil {
			return fmt.Errorf("clear the care days of a holiday guest: %w", err)
		}
		if err := h.planDays(ctx, guest.StudentID, actor, source, week); err != nil {
			return err
		}
	}
	return h.store.UpdateGuest(ctx, schoolID, guestRecord(guest, source, week))
}

// reopenGuest brings back a guest whose booking returned after its access
// ended early: the same child, its care interval, group and days again.
func (h *HolidayGuests) reopenGuest(ctx context.Context, schoolID int64, guest enrollment.HolidayGuest, source enrollment.HolidayGuestSource, week enrollment.HolidayGuestWeek, groupID *int64, actor int64, today calendar.Date) error {
	if err := h.store.ReopenGuest(ctx, schoolID, guest.ID); err != nil {
		return err
	}
	return h.refreshGuest(ctx, schoolID, guest, source, week, groupID, actor, true, today)
}

// endGuest ends the care of a guest whose booking or week went: from
// today on the child is no longer cared for, and its contacts go.
func (h *HolidayGuests) endGuest(ctx context.Context, schoolID int64, guest enrollment.HolidayGuest, today calendar.Date) error {
	current, err := h.people.ReadHolidayGuestChild(ctx, guest.StudentID)
	if err != nil {
		return fmt.Errorf("read the child of a holiday guest: %w", err)
	}
	lastDay := today.AddDays(-1)
	if current.EnrolledUntil == "" || current.EnrolledUntil.After(lastDay) || current.Pending {
		current.Pending = false
		current.EnrolledUntil = lastDay
		if current.EnrolledFrom == "" || current.EnrolledFrom.After(lastDay) {
			current.EnrolledFrom = lastDay
		}
		if err := h.people.UpdateHolidayGuestChild(ctx, guest.StudentID, current); err != nil {
			return fmt.Errorf("end the care of a holiday guest: %w", err)
		}
	}
	if err := h.care.ClearHolidayGuestDays(ctx, guest.StudentID); err != nil {
		return fmt.Errorf("clear the care days of a holiday guest: %w", err)
	}
	return h.store.EndGuest(ctx, schoolID, guest.ID, today)
}

// guestChild is the child a guest should be on today: before its week
// pending and in no group, from its first day active in the group.
func guestChild(source enrollment.HolidayGuestSource, week enrollment.HolidayGuestWeek, groupID *int64, today calendar.Date) enrollment.HolidayGuestChild {
	child := enrollment.HolidayGuestChild{
		FirstName: source.FirstName, LastName: source.LastName, BirthDate: source.BirthDate, GroupID: groupID,
		EnrolledFrom: week.StartsOn, EnrolledUntil: week.EndsOn,
	}
	if today.Before(week.StartsOn) {
		child.GroupID, child.Pending = nil, true
	}
	return child
}

func sameGuestChild(a, b enrollment.HolidayGuestChild) bool {
	sameGroup := (a.GroupID == nil) == (b.GroupID == nil) && (a.GroupID == nil || *a.GroupID == *b.GroupID)
	return sameGroup && a.Pending == b.Pending && a.FirstName == b.FirstName && a.LastName == b.LastName && a.BirthDate == b.BirthDate &&
		a.EnrolledFrom == b.EnrolledFrom && a.EnrolledUntil == b.EnrolledUntil
}

func guestRecord(guest enrollment.HolidayGuest, source enrollment.HolidayGuestSource, week enrollment.HolidayGuestWeek) enrollment.HolidayGuest {
	guest.StartsOn, guest.EndsOn = week.StartsOn, week.EndsOn
	guest.HomeSchoolID, guest.GuestSchoolName = source.HomeSchoolID, source.GuestSchoolName
	guest.TimeModelName, guest.TimeModelStarts, guest.TimeModelEnds = source.TimeModelName, source.TimeModelStarts, source.TimeModelEnds
	guest.Emergency, guest.Pickup = source.Emergency, source.Pickup
	return guest
}

// HolidayGuestSheet returns the sheet of a guest until the read-only days
// after its week are over and records the read.
func (h *HolidayGuests) HolidayGuestSheet(ctx context.Context, host enrollment.HolidayHost, studentID int64) (enrollment.HolidayGuestSheet, error) {
	if host.SchoolID <= 0 || host.AccountID <= 0 {
		return enrollment.HolidayGuestSheet{}, errors.New("enrollment: the hosting school and the account are required")
	}
	if studentID <= 0 {
		return enrollment.HolidayGuestSheet{}, enrollment.ErrHolidayGuestNotFound
	}
	today := h.today()
	guest, found, err := h.store.ReadGuest(ctx, host.SchoolID, studentID, today.AddDays(-enrollment.HolidayGuestReadOnlyDays))
	if err != nil {
		return enrollment.HolidayGuestSheet{}, err
	}
	if !found {
		return enrollment.HolidayGuestSheet{}, enrollment.ErrHolidayGuestNotFound
	}
	if err := h.log.RecordHolidayGuestAccess(ctx, enrollment.HolidayGuestAccess{
		Kind: enrollment.HolidayGuestAccessSheet, ActorAccountID: host.AccountID, ActorRole: host.Role,
		WeekID: guest.WeekID, StudentID: guest.StudentID, StartsOn: guest.StartsOn, EndsOn: guest.EndsOn,
	}); err != nil {
		return enrollment.HolidayGuestSheet{}, fmt.Errorf("record holiday guest sheet read: %w", err)
	}
	return holidayGuestSheet(guest, today), nil
}

func holidayGuestSheet(guest enrollment.HolidayGuest, today calendar.Date) enrollment.HolidayGuestSheet {
	sheet := enrollment.HolidayGuestSheet{
		StudentID: guest.StudentID, WeekID: guest.WeekID, StartsOn: guest.StartsOn, EndsOn: guest.EndsOn,
		TimeModelName: guest.TimeModelName, TimeModelStarts: guest.TimeModelStarts, TimeModelEnds: guest.TimeModelEnds,
		HomeSchoolID: guest.HomeSchoolID, GuestSchoolName: guest.GuestSchoolName,
		Emergency: guest.Emergency, Pickup: guest.Pickup, ReadOnly: guest.EndsOn.Before(today),
	}
	if sheet.Emergency == nil {
		sheet.Emergency = []enrollment.HolidayGuestContact{}
	}
	if sheet.Pickup == nil {
		sheet.Pickup = []enrollment.HolidayGuestContact{}
	}
	return sheet
}
