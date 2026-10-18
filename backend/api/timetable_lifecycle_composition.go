package api

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
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

// NewTimetableLifecycle composes the instance lifecycle over deps and the
// retained rows and binds the collaborators the owner may not name. deps
// carries the planning repositories the rows do not (IdempotencyRepo,
// ExceptionRepo, CalendarPeriodRepo), the database and the owners the
// lifecycle reads directly; the rows (Instances, InstanceStaff,
// Participants, Templates, Students, Rooms, DeviationEvents and the recovery
// Locks), Care Plan's care days, Communication's cancellation notice, the
// operational settings and the broadcaster are bound here. guardianNotices
// and broadcaster are optional.
func NewTimetableLifecycle(
	deps timetablePorts.InstanceLifecycleDependencies, rows timetableCompose.OwnerRows, careDays careplan.CareDayQuery,
	guardianNotices communication.CareCancellationPublisher, settings TimetableSettings, broadcaster realtime.Broadcaster,
) (timetablePorts.InstanceLifecycle, error) {
	if rows.Locks == nil {
		return nil, errors.New("timetable instance lifecycle: the recovery locks are required")
	}
	var lifecycleSettings timetablePorts.LifecycleSettings
	if settings != nil {
		lifecycleSettings = timetableOperationSettings{settings: settings}
	}
	deps.InstanceRepo, deps.InstanceStaffRepo, deps.InstanceStudents = rows.Instances, rows.InstanceStaff, rows.Participants
	deps.RecoveryRepo, deps.ActivityGroupRepo, deps.StudentRepo = rows.Locks, rows.Templates, NewTimetableLifecycleStudents(rows.Students)
	deps.Rooms = rows.Rooms
	deps.CareDays = newTimetableCareDays(careDays)
	deps.Protocol = rows.Deviations
	deps.GuardianNotices = newTimetableGuardianNotices(guardianNotices)
	deps.Broadcaster, deps.Settings = newTimetableLifecycleBroadcaster(broadcaster), lifecycleSettings
	deps.ContentHash = securityruntime.Fingerprint
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
