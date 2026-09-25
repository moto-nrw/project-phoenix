package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/realtime"
)

// The Timetable owner's instance lifecycle (#3424 slice S1), bound to the
// collaborators the owner may not name: the Facilities rooms (through the
// retained rows), Care Plan's care days and day locks, Communication's
// cancellation notice, the Settings Platform's clock policy, the Audit
// Platform's Änderungsprotokoll, Security Runtime's content fingerprint
// and the realtime hub.

// timetableLifecycleInputs compose the lifecycle over the retained rows
// (Instances, InstanceStaff, Participants, Templates, Students, Rooms,
// DeviationEvents and the recovery Locks). Planning carries the retained
// planning repositories the rows do not (IdempotencyRepo, ExceptionRepo,
// CalendarPeriodRepo); every other field of it is filled here. Broadcaster
// and GuardianNotices are optional.
type timetableLifecycleInputs struct {
	Rows                repositories.TimetableOwnerRows
	Planning            timetableCompose.InstanceLifecycleDependencies
	Staff               usersModels.StaffRepository
	Sessions            studentpresence.SessionRecords
	Supervisions        studentpresence.SupervisionRecords
	Presence            timetableCompose.LifecyclePresence
	SessionEnder        timetableCompose.SessionEnder
	CareDays            careplan.CareDayQuery
	CareDayLocks        timetableCompose.CareDayLocks
	GuardianNotices     communication.CareCancellationPublisher
	Settings            timetableOperationSettingsSource
	Materialization     timetable.MaterializationCapability
	RecurrenceLock      timetable.RecurrenceWriteLock
	StartConflicts      timetable.StartConflictQuery
	SubstituteConflicts timetable.SubstituteConflictQuery
	Broadcaster         realtime.Broadcaster
	DB                  *bun.DB
	Logger              *slog.Logger
	Now                 func() time.Time
	EnforceTimePolicy   bool
}

func newTimetableLifecycle(in timetableLifecycleInputs) (*timetableCompose.InstanceLifecycleService, error) {
	if in.Rows.Locks == nil {
		return nil, errors.New("timetable instance lifecycle: the recovery locks are required")
	}
	var settings timetableCompose.LifecycleSettings
	if in.Settings != nil {
		settings = timetableOperationSettings{settings: in.Settings}
	}
	deps := in.Planning
	deps.InstanceRepo, deps.InstanceStaffRepo, deps.InstanceStudents = in.Rows.Instances, in.Rows.InstanceStaff, in.Rows.Participants
	deps.RecoveryRepo, deps.ActivityGroupRepo, deps.StudentRepo = in.Rows.Locks, in.Rows.Templates, in.Rows.Students
	deps.StaffRepo, deps.ActiveGroupRepo, deps.SupervisorRepo = in.Staff, in.Sessions, in.Supervisions
	deps.Presence, deps.ActiveService, deps.Rooms = in.Presence, in.SessionEnder, in.Rows.LifecycleRooms()
	deps.CareDays, deps.CareDayLocks = newTimetableCareDays(in.CareDays), in.CareDayLocks
	deps.Protocol = repositories.TimetableDeviationProtocol(in.Rows.DeviationEvents)
	deps.GuardianNotices = newTimetableGuardianNotices(in.GuardianNotices)
	deps.Broadcaster, deps.Settings = newTimetableLifecycleBroadcaster(in.Broadcaster), settings
	deps.Materialization, deps.RecurrenceLock = in.Materialization, in.RecurrenceLock
	deps.StartConflicts, deps.SubstituteConflicts = in.StartConflicts, in.SubstituteConflicts
	deps.ContentHash = securityruntime.Fingerprint
	deps.DB, deps.Logger, deps.Now, deps.EnforceTimePolicy = in.DB, in.Logger, in.Now, in.EnforceTimePolicy
	return timetableCompose.NewInstanceLifecycle(deps)
}

// timetableGuardianNotices binds Communication's care cancellation notice.
// A refused publication is wrapped in the owner's notice errors, keeping
// Communication's cause in the chain.
type timetableGuardianNotices struct {
	publisher communication.CareCancellationPublisher
}

func newTimetableGuardianNotices(publisher communication.CareCancellationPublisher) timetableCompose.GuardianNotices {
	if publisher == nil {
		return nil
	}
	return timetableGuardianNotices{publisher: publisher}
}

func (n timetableGuardianNotices) ValidateNoticeText(title, message string) error {
	_, _, err := communication.ValidateCareCancellationText(title, message)
	return err
}

func (n timetableGuardianNotices) NoticeReach(ctx context.Context, studentIDs []int64) (timetableCompose.GuardianNoticeAudience, error) {
	reach, err := n.publisher.CareCancellationReachFor(ctx, studentIDs)
	if err != nil {
		return timetableCompose.GuardianNoticeAudience{}, err
	}
	return timetableCompose.GuardianNoticeAudience{Enabled: reach.Enabled, DefaultOn: reach.DefaultOn, FamilyCount: reach.FamilyCount}, nil
}

func (n timetableGuardianNotices) PublishNotice(ctx context.Context, notice timetableCompose.GuardianNoticePublication) (timetableCompose.GuardianNoticePublished, error) {
	published, err := n.publisher.PublishCareCancellation(ctx, communication.CareCancellationInput{
		StudentIDs: notice.StudentIDs, Title: notice.Title, Body: notice.Body, CreatedBy: notice.CreatedBy,
	})
	switch {
	case errors.Is(err, communication.ErrCareCancellationDisabled):
		return timetableCompose.GuardianNoticePublished{}, fmt.Errorf("%w: %w", timetable.ErrGuardianNoticeDisabled, err)
	case errors.Is(err, communication.ErrParentAnnouncementValidation):
		return timetableCompose.GuardianNoticePublished{}, fmt.Errorf("%w: %w", timetable.ErrGuardianNoticeInvalid, err)
	case err != nil:
		return timetableCompose.GuardianNoticePublished{}, err
	}
	return timetableCompose.GuardianNoticePublished{AnnouncementID: published.AnnouncementID, RecipientCount: published.RecipientCount}, nil
}

// timetableLifecycleBroadcaster delivers the lifecycle's events through the
// realtime hub; the event types pass through unchanged.
type timetableLifecycleBroadcaster struct {
	broadcaster realtime.Broadcaster
}

func newTimetableLifecycleBroadcaster(broadcaster realtime.Broadcaster) timetableCompose.LifecycleBroadcaster {
	if broadcaster == nil {
		return nil
	}
	return timetableLifecycleBroadcaster{broadcaster: broadcaster}
}

func realtimeEventOf(event timetableCompose.LifecycleEvent) realtime.Event {
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

func (b timetableLifecycleBroadcaster) BroadcastToGroup(tenantID int64, topic string, event timetableCompose.LifecycleEvent) error {
	return b.broadcaster.BroadcastToGroup(tenantID, topic, realtimeEventOf(event))
}

func (b timetableLifecycleBroadcaster) BroadcastToTenant(tenantID int64, event timetableCompose.LifecycleEvent) error {
	return b.broadcaster.BroadcastToTenant(tenantID, realtimeEventOf(event))
}

// timetableLifecycleSchedulers are the scheduler's auto start and auto end
// over the lifecycle, with the owner's start check.
func newTimetableLifecycleSchedulers(rows repositories.TimetableOwnerRows, lifecycle timetable.InstanceLifecycle, conflicts timetable.StartConflictQuery, logger *slog.Logger) (timetable.InstanceAutoStart, timetable.InstanceAutoEnd, error) {
	autoStart, err := timetableCompose.NewInstanceAutoStart(timetableCompose.InstanceAutoStartDependencies{
		InstanceRepo:      rows.Instances,
		InstanceStaffRepo: rows.InstanceStaff,
		Lifecycle:         lifecycle,
		Rooms:             rows.LifecycleRooms(),
		Conflicts:         conflicts,
		Logger:            logger,
	})
	if err != nil {
		return nil, nil, err
	}
	autoEnd, err := timetableCompose.NewInstanceAutoEnd(rows.Instances, lifecycle)
	if err != nil {
		return nil, nil, err
	}
	return autoStart, autoEnd, nil
}
