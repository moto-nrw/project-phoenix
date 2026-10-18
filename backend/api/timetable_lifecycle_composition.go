package api

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/delivery/compose/realtime"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// The Timetable owner's instance lifecycle (#3424 slice S1), bound to the
// collaborators the owner may not name: the Facilities rooms (through the
// retained rows), Care Plan's care days and day locks, Communication's
// cancellation notice, the Settings Platform's clock policy, the Audit
// Platform's Änderungsprotokoll, Security Runtime's content fingerprint
// and the realtime hub.

// NewTimetableLifecycle composes the instance lifecycle over the retained
// rows and binds the collaborators the owner may not name.
func NewTimetableLifecycle(in TimetableLifecycleInputs) (timetablePorts.InstanceLifecycle, error) {
	if in.Rows.Locks == nil {
		return nil, errors.New("timetable instance lifecycle: the recovery locks are required")
	}
	var settings timetablePorts.LifecycleSettings
	if in.Settings != nil {
		settings = timetableOperationSettings{settings: in.Settings}
	}
	deps := in.Planning
	deps.InstanceRepo, deps.InstanceStaffRepo, deps.InstanceStudents = in.Rows.Instances, in.Rows.InstanceStaff, in.Rows.Participants
	deps.RecoveryRepo, deps.ActivityGroupRepo, deps.StudentRepo = in.Rows.Locks, in.Rows.Templates, NewTimetableLifecycleStudents(in.Rows.Students)
	deps.StaffRepo, deps.ActiveGroupRepo, deps.SupervisorRepo = in.Staff, in.Sessions, in.Supervisions
	deps.Presence, deps.ActiveService, deps.Rooms = in.Presence, in.SessionEnder, in.Rows.Rooms
	deps.CareDays, deps.CareDayLocks = newTimetableCareDays(in.CareDays), in.CareDayLocks
	deps.Protocol = in.Rows.Deviations
	deps.GuardianNotices = newTimetableGuardianNotices(in.GuardianNotices)
	deps.Broadcaster, deps.Settings = newTimetableLifecycleBroadcaster(in.Broadcaster), settings
	deps.Materialization, deps.RecurrenceLock = in.Materialization, in.RecurrenceLock
	deps.StartConflicts, deps.SubstituteConflicts = in.StartConflicts, in.SubstituteConflicts
	deps.ContentHash = securityruntime.Fingerprint
	deps.Logger, deps.Now, deps.EnforceTimePolicy = in.Logger, in.Now, in.EnforceTimePolicy
	return timetableCompose.NewInstanceLifecycle(deps)
}

// timetableGuardianNotices binds Communication's care cancellation notice.
// A refused publication is wrapped in the owner's notice errors, keeping
// Communication's cause in the chain.
type timetableGuardianNotices struct {
	publisher communication.CareCancellationPublisher
}

func newTimetableGuardianNotices(publisher communication.CareCancellationPublisher) timetablePorts.GuardianNotices {
	if publisher == nil {
		return nil
	}
	return timetableGuardianNotices{publisher: publisher}
}

func (n timetableGuardianNotices) ValidateNoticeText(title, message string) error {
	_, _, err := communication.ValidateCareCancellationText(title, message)
	return err
}

func (n timetableGuardianNotices) NoticeReach(ctx context.Context, studentIDs []int64) (timetablePorts.GuardianNoticeAudience, error) {
	reach, err := n.publisher.CareCancellationReachFor(ctx, studentIDs)
	if err != nil {
		return timetablePorts.GuardianNoticeAudience{}, err
	}
	return timetablePorts.GuardianNoticeAudience{FamilyCount: reach.FamilyCount}, nil
}

func (n timetableGuardianNotices) PublishNotice(ctx context.Context, notice timetablePorts.GuardianNoticePublication) (timetablePorts.GuardianNoticePublished, error) {
	published, err := n.publisher.PublishCareCancellation(ctx, communication.CareCancellationInput{
		StudentIDs: notice.StudentIDs, Title: notice.Title, Body: notice.Body, CreatedBy: notice.CreatedBy,
	})
	switch {
	case errors.Is(err, communication.ErrParentAnnouncementValidation):
		return timetablePorts.GuardianNoticePublished{}, fmt.Errorf("%w: %w", timetable.ErrGuardianNoticeInvalid, err)
	case err != nil:
		return timetablePorts.GuardianNoticePublished{}, err
	}
	return timetablePorts.GuardianNoticePublished{AnnouncementID: published.AnnouncementID, RecipientCount: published.RecipientCount}, nil
}

// timetableLifecycleBroadcaster delivers the lifecycle's events through the
// realtime hub; the event types pass through unchanged.
type timetableLifecycleBroadcaster struct {
	broadcaster realtime.Broadcaster
}

func newTimetableLifecycleBroadcaster(broadcaster realtime.Broadcaster) timetablePorts.LifecycleBroadcaster {
	if broadcaster == nil {
		return nil
	}
	return timetableLifecycleBroadcaster{broadcaster: broadcaster}
}

func realtimeEventOf(event timetablePorts.LifecycleEvent) realtime.Event {
	return realtime.NewEvent(realtime.EventType(event.Type), event.ActiveGroupID, realtime.EventData{
		InstanceID:        event.InstanceID,
		InstanceDate:      event.InstanceDate,
		InstanceStartTime: event.InstanceStartTime,
		RoomID:            event.RoomID,
		RoomName:          event.RoomName,
		ActivityName:      event.ActivityName,
		SupervisorIDs:     event.SupervisorIDs,
		StudentIDs:        event.StudentIDs,
		GroupIDs:          event.GroupIDs,
		Reason:            event.Reason,
		Source:            event.Source,
	})
}

func (b timetableLifecycleBroadcaster) BroadcastToGroup(tenantID int64, topic string, event timetablePorts.LifecycleEvent) error {
	return b.broadcaster.BroadcastToGroup(tenantID, topic, realtimeEventOf(event))
}

func (b timetableLifecycleBroadcaster) BroadcastToTenant(tenantID int64, event timetablePorts.LifecycleEvent) error {
	return b.broadcaster.BroadcastToTenant(tenantID, realtimeEventOf(event))
}
