package compose

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// Completion of blocks whose live session somebody else ended (#1747, #3424
// slice S5): the nightly session end, the force-start path that kicks a
// running session off a device or an activity, and the kiosk's "Sitzung
// beenden". A path that only flips the instance status would leave every
// expected row behind — an absence nobody recorded and a non-booking nobody
// wrote down, on a day that is over and that no later path revisits.
//
// Session-end completions stay outside the five-minute reopen window; the
// planner and operations Complete path owns recovery snapshots and the
// planned-end gate.

// EndedSessionInstances is the slice of the retained activity instance
// repository the completion reads and writes; its rows carry the Student
// Presence session each block runs in.
type EndedSessionInstances interface {
	List(ctx context.Context, options *ActivityInstanceQueryOptions) ([]*scheduleModels.ActivityInstance, error)
	CompleteActiveByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, completedAt time.Time) (int64, error)
}

// EndedSessionParticipants is the slice of the retained participant
// repository that finalizes the attendance of the ended blocks.
type EndedSessionParticipants interface {
	FindNotScheduledCandidatesByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*scheduleModels.InstanceStudent, error)
	MarkNotScheduled(ctx context.Context, refs []scheduleModels.StudentInstanceRef) error
	MarkExpectedAbsentByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, updatedAt time.Time, exclusions []scheduleModels.StudentInstanceRef) error
}

// EndedSessionCompletionDependencies wire the completion. CareDays is
// optional: without it no child is spared the absent stamp, which is the
// behaviour that predates #1747 — never a silent skip.
type EndedSessionCompletionDependencies struct {
	Instances    EndedSessionInstances
	Participants EndedSessionParticipants
	CareDays     CareDays
}

type endedSessionCompletion struct {
	deps EndedSessionCompletionDependencies
}

// NewEndedSessionCompletion composes the Timetable owner's completion of
// blocks behind ended live sessions.
func NewEndedSessionCompletion(deps EndedSessionCompletionDependencies) (timetable.EndedSessionCompletion, error) {
	if deps.Instances == nil || deps.Participants == nil {
		return nil, errors.New("ended session completion: instances and participants are required")
	}
	return &endedSessionCompletion{deps: deps}, nil
}

// CompleteActiveByActiveGroupIDs finalizes attendance, then completes the
// blocks. The bulk absent update only matches blocks that still run, so it
// has to precede the completion.
func (c *endedSessionCompletion) CompleteActiveByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, completedAt time.Time) (int64, error) {
	if len(activeGroupIDs) == 0 {
		return 0, nil
	}
	notScheduled, err := c.notScheduledForEndedSessions(ctx, activeGroupIDs)
	if err != nil {
		return 0, err
	}
	// Stamp the non-booking marker before sparing those rows the absence:
	// the spared rows must carry the reason themselves, or later writers of
	// `status` cannot tell them from ordinary expected rows (#1747 review).
	if err := c.deps.Participants.MarkNotScheduled(ctx, notScheduled); err != nil {
		return 0, fmt.Errorf("complete bridged instances: mark not scheduled: %w", err)
	}
	if err := c.deps.Participants.MarkExpectedAbsentByActiveGroupIDs(ctx, activeGroupIDs, completedAt, notScheduled); err != nil {
		return 0, fmt.Errorf("complete bridged instances: mark absent: %w", err)
	}
	return c.deps.Instances.CompleteActiveByActiveGroupIDs(ctx, activeGroupIDs, completedAt)
}

// notScheduledForEndedSessions collects the (instance, student) pairs whose
// child the care plan does not book at all on that instance's date. An
// explicitly cancelled day is deliberately not in here: it is a reported
// absence and still gets written. The exclusions are per instance on
// purpose: one run can close blocks of several dates, and a child not booked
// on one date may be expected on another. Without a care-day port the list
// stays empty, which keeps the blanket absence stamp.
func (c *endedSessionCompletion) notScheduledForEndedSessions(ctx context.Context, activeGroupIDs []int64) ([]scheduleModels.StudentInstanceRef, error) {
	if c.deps.CareDays == nil {
		return nil, nil
	}
	datesByInstance, err := c.endedInstanceDates(ctx, activeGroupIDs)
	if err != nil || len(datesByInstance) == 0 {
		return nil, err
	}
	instanceIDs := make([]int64, 0, len(datesByInstance))
	for instanceID := range datesByInstance {
		instanceIDs = append(instanceIDs, instanceID)
	}
	// Not just the 'expected' rows: a broad day status (sick / excused /
	// class trip) is reported before anything knows whether the child was
	// booked, so an unbooked child can already sit here as 'absent' with the
	// status day owning the row. Ending the block resolves that, and
	// MarkNotScheduled only undoes the false absence on rows this read
	// returns (#1747 review).
	rows, err := c.deps.Participants.FindNotScheduledCandidatesByInstanceIDs(ctx, instanceIDs)
	if err != nil {
		return nil, fmt.Errorf("complete bridged instances: load candidate students: %w", err)
	}
	window, ok := coveringCareDayWindow(rows, datesByInstance)
	if !ok {
		return nil, nil
	}
	// Sessions can straddle midnight, so the blocks may span two dates. One
	// range read over the covering window resolves every (student, date)
	// pair at the cost of a single date.
	careDays, err := c.deps.CareDays.ResolveForRange(ctx, window.studentIDs, window.from, window.to)
	if err != nil {
		return nil, fmt.Errorf("complete bridged instances: resolve care days: %s..%s: %w", window.from, window.to, err)
	}
	exclusions := make([]scheduleModels.StudentInstanceRef, 0)
	for _, row := range rows {
		date, ok := datesByInstance[row.InstanceID]
		if ok && c.deps.CareDays.ExemptFromAbsence(careDays[row.StudentID][date]) {
			exclusions = append(exclusions, scheduleModels.StudentInstanceRef{StudentID: row.StudentID, InstanceID: row.InstanceID})
		}
	}
	return exclusions, nil
}

// endedInstanceDates maps each block bridged to one of the sessions to its
// date.
func (c *endedSessionCompletion) endedInstanceDates(ctx context.Context, activeGroupIDs []int64) (map[int64]timezone.Date, error) {
	instances, err := c.deps.Instances.List(ctx, &modelBase.QueryOptions{
		Filter: modelBase.NewFilter().In("active_group_id", int64Args(activeGroupIDs)...),
	})
	if err != nil {
		return nil, fmt.Errorf("complete bridged instances: load instances: %w", err)
	}
	dates := make(map[int64]timezone.Date, len(instances))
	for _, instance := range instances {
		dates[instance.ID] = timezone.Date(instance.Date)
	}
	return dates, nil
}

// careDayWindow is the inclusive date window and the distinct children one
// care-day read covers.
type careDayWindow struct {
	from, to   timezone.Date
	studentIDs []int64
}

func coveringCareDayWindow(rows []*scheduleModels.InstanceStudent, datesByInstance map[int64]timezone.Date) (careDayWindow, bool) {
	var window careDayWindow
	seen := make(map[int64]bool, len(rows))
	for _, row := range rows {
		date, ok := datesByInstance[row.InstanceID]
		if !ok {
			continue
		}
		if len(window.studentIDs) == 0 || date.Before(window.from) {
			window.from = date
		}
		if len(window.studentIDs) == 0 || date.After(window.to) {
			window.to = date
		}
		if !seen[row.StudentID] {
			seen[row.StudentID] = true
			window.studentIDs = append(window.studentIDs, row.StudentID)
		}
	}
	return window, len(window.studentIDs) > 0
}

// int64Args widens ids for the persistence-neutral Filter.In API.
func int64Args(ids []int64) []any {
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return args
}
