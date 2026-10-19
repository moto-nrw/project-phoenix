package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func (d *timetableCommands) FindScheduledInstance(ctx context.Context, id int64) (value0 timetable.ScheduledInstance, err error) {
	err = d.timetableData.deps.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = d.timetableData.FindScheduledInstance(txCtx, id)
		return err
	})
	return
}

func (d *timetableCommands) ListScheduledInstances(ctx context.Context, from, to calendar.Date) (value0 []timetable.ScheduledInstance, err error) {
	err = d.timetableData.deps.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = d.timetableData.ListScheduledInstances(txCtx, from, to)
		return err
	})
	return
}

// ListScheduledInstanceRows loads the rows of the listed blocks with one
// batched read per kind plus one cutoff read per distinct date, instead of
// three reads per block (#2940). Participant rows keep the created_at order
// of the single-block read.
func (d *timetableCommands) ListScheduledInstanceRows(ctx context.Context, instances []timetable.ScheduledInstance) (value0 *timetable.ScheduledInstanceRows, err error) {
	err = d.timetableData.deps.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = d.timetableData.ListScheduledInstanceRows(txCtx, instances)
		return err
	})
	return
}

func (d *timetableCommands) ListBlockStaff(ctx context.Context, instanceID int64) (value0 []timetable.InstanceStaff, err error) {
	err = d.timetableData.deps.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = d.timetableData.ListBlockStaff(txCtx, instanceID)
		return err
	})
	return
}

func (d *timetableCommands) ListBlockParticipants(ctx context.Context, instanceID int64) (value0 []timetable.ScheduledParticipant, err error) {
	err = d.timetableData.deps.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = d.timetableData.ListBlockParticipants(txCtx, instanceID)
		return err
	})
	return
}

// ListCareDayCandidates returns the rows whose care-day verdict can still
// change something: still 'expected', plus rows a broad day status (sick,
// excused, class trip) flipped to 'absent' and still owns. The status day
// is written before anything knows whether the child was booked, so an
// unbooked child can already sit there as absent (#1747).
func (d *timetableCommands) ListCareDayCandidates(ctx context.Context, instanceIDs []int64) (value0 []timetable.ScheduledParticipant, err error) {
	err = d.timetableData.deps.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = d.timetableData.ListCareDayCandidates(txCtx, instanceIDs)
		return err
	})
	return
}

func (d *timetableCommands) FindBlockParticipant(ctx context.Context, instanceID, studentID int64) (value0 *timetable.ScheduledParticipant, err error) {
	err = d.timetableData.deps.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = d.timetableData.FindBlockParticipant(txCtx, instanceID, studentID)
		return err
	})
	return
}

func (d *timetableCommands) BlockDisplayMetadata(ctx context.Context, instanceIDs []int64) (value0 map[int64]timetable.BlockDisplayMetadata, err error) {
	err = d.timetableData.deps.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = d.timetableData.BlockDisplayMetadata(txCtx, instanceIDs)
		return err
	})
	return
}

func (d *timetableCommands) PatchSlotAttendance(ctx context.Context, participantID int64, patch timetable.AttendancePatch) (err error) {
	err = d.timetableData.deps.RunInTx(ctx, func(txCtx context.Context) error {
		err = d.timetableData.PatchSlotAttendance(txCtx, participantID, patch)
		return err
	})
	return
}

// ListUnderstaffedInstances loads the window's planned and running blocks
// with one read, their staff rows with a second, and returns the
// understaffed ones with their counts. Completed and cancelled blocks are
// history and never reported.
func (d *timetableCommands) ListUnderstaffedInstances(ctx context.Context, from, to calendar.Date) (value0 []timetable.UnderstaffedInstance, err error) {
	err = d.timetableData.deps.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = d.timetableData.ListUnderstaffedInstances(txCtx, from, to)
		return err
	})
	return
}

func (d *timetableCommands) SpontaneousRoomExists(ctx context.Context, roomID int64) (value0 bool, err error) {
	err = d.timetableData.deps.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = d.timetableData.SpontaneousRoomExists(txCtx, roomID)
		return err
	})
	return
}

func (d *timetableCommands) SpontaneousRoomOccupied(ctx context.Context, roomID int64) (value0 bool, err error) {
	err = d.timetableData.deps.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = d.timetableData.SpontaneousRoomOccupied(txCtx, roomID)
		return err
	})
	return
}

// ResolveSpontaneousActivity returns the requested activity or the activity
// named after the title; a missing one is created in the "Spontan" category
// under a name lock, so two starts with the same title share it.
func (d *timetableCommands) ResolveSpontaneousActivity(ctx context.Context, title string, requestedID *int64, createdBy int64) (value0 *int64, err error) {
	err = d.timetableData.deps.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = d.timetableData.ResolveSpontaneousActivity(txCtx, title, requestedID, createdBy)
		return err
	})
	return
}

// StudentWeek reads a child's timetable over the window with one read per
// kind, whatever the window's length: the planned blocks with the child's
// slot, every block of the tenant (for the walk-ins), the child's visits in
// running or ended sessions, the regular arrival and pickup through Care
// Plan's baselines and the dated exceptions.
func (d *timetableCommands) StudentWeek(ctx context.Context, studentID int64, from, to calendar.Date) (value0 *timetable.StudentWeek, err error) {
	err = d.timetableData.deps.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = d.timetableData.StudentWeek(txCtx, studentID, from, to)
		return err
	})
	return
}

func (d *timetableCommands) ListTemplateEntries(ctx context.Context, templateID *int64, childrenPerStaffRatio int) (value0 []timetable.TemplateListEntry, err error) {
	err = d.timetableData.deps.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = d.timetableData.ListTemplateEntries(txCtx, templateID, childrenPerStaffRatio)
		return err
	})
	return
}

// ListTemplateEntriesForTemplatePeriod is the detail read model of one
// editable template period, with period-scoped roster and capacity.
func (d *timetableCommands) ListTemplateEntriesForTemplatePeriod(ctx context.Context, templateID, periodID int64, childrenPerStaffRatio int) (value0 []timetable.TemplateListEntry, err error) {
	err = d.timetableData.deps.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = d.timetableData.ListTemplateEntriesForTemplatePeriod(txCtx, templateID, periodID, childrenPerStaffRatio)
		return err
	})
	return
}

func (d *timetableCommands) ListTemplateEntriesForPeriod(ctx context.Context, periodID *int64, childrenPerStaffRatio int) (value0 []timetable.TemplateListEntry, err error) {
	err = d.timetableData.deps.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = d.timetableData.ListTemplateEntriesForPeriod(txCtx, periodID, childrenPerStaffRatio)
		return err
	})
	return
}

// ListTemplateWeekdayRoster is a thin read: grouping the rows into weekday
// assignments is a presentation concern of the caller.
func (d *timetableCommands) ListTemplateWeekdayRoster(ctx context.Context, templateID, calendarPeriodID *int64) (value0 []timetable.TemplateWeekdayRosterRow, err error) {
	err = d.timetableData.deps.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = d.timetableData.ListTemplateWeekdayRoster(txCtx, templateID, calendarPeriodID)
		return err
	})
	return
}
