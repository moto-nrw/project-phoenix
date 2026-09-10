package application

import (
	"context"
	"fmt"
	"log/slog"

	"time"
)

// MirroredSession contains only the timetable fields the kiosk mirror needs.
type MirroredSession struct {
	ID, RoomID                                           int64
	Date, Title, StartTime, EndTime                      string
	ActivityGroupID, ActiveGroupID, CreatedBy, StartedBy *int64
	StartedAt                                            *time.Time
}

// MirrorTimetable is the consumer-owned seam for best-effort timetable writes.
type MirrorTimetable interface {
	HasInstance(context.Context, int64) (bool, error)
	CreateInstance(context.Context, MirroredSession) (MirroredSession, error)
	AddStaff(context.Context, int64, int64) error
	ActivityTitle(context.Context, int64) (string, error)
	CalendarParts(time.Time) (date string, minutes int)
}

// MirrorPublisher queues the existing instance-started notification for commit.
type MirrorPublisher func(context.Context, MirroredSession)

type sessionMirror struct {
	timetable MirrorTimetable
	publish   MirrorPublisher
	logger    *slog.Logger
}

func NewSessionMirror(timetable MirrorTimetable, publish MirrorPublisher, logger *slog.Logger) SessionMirror {
	if timetable == nil {
		return nil
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &sessionMirror{timetable: timetable, publish: publish, logger: logger}
}

func (s *sessionMirror) MirrorSession(ctx context.Context, group LifecycleSession, supervisorIDs []int64) {
	existing, err := s.timetable.HasInstance(ctx, group.ID)
	if err != nil {
		s.logger.WarnContext(ctx, "failed to check mirrored timetable instance",
			slog.Int64("active_group_id", group.ID),
			slog.String("error", err.Error()),
		)
		return
	}
	if existing {
		return
	}
	startedAt := group.StartTime
	date, minutes := s.timetable.CalendarParts(startedAt)
	startMinutes := min(minutes, 23*60+30)
	endMinutes := min(startMinutes+60, 23*60+59)
	title := "RFID-Aktivität"
	if group.ActivityID != nil {
		if activity, err := s.timetable.ActivityTitle(ctx, *group.ActivityID); err == nil && activity != "" {
			title = activity
		}
	}
	var startedBy *int64
	for _, id := range supervisorIDs {
		if id > 0 {
			startedBy = &id
			break
		}
	}
	instance, err := s.timetable.CreateInstance(ctx, MirroredSession{
		Date: date, ActivityGroupID: group.ActivityID, Title: title,
		StartTime: mirrorClock(startMinutes), EndTime: mirrorClock(endMinutes), RoomID: group.RoomID,
		ActiveGroupID: &group.ID,
		CreatedBy:     startedBy, StartedBy: startedBy, StartedAt: &startedAt,
	})
	if err != nil {
		s.logger.WarnContext(ctx, "failed to mirror IoT session to timetable",
			slog.Int64("active_group_id", group.ID),
			slog.String("error", err.Error()),
		)
		return
	}
	seen := make(map[int64]bool, len(supervisorIDs))
	for _, id := range supervisorIDs {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		if err := s.timetable.AddStaff(ctx, instance.ID, id); err != nil {
			s.logger.WarnContext(ctx, "failed to mirror IoT session staff to timetable",
				slog.Int64("instance_id", instance.ID),
				slog.Int64("staff_id", id),
				slog.String("error", err.Error()),
			)
		}
	}
	if s.publish != nil {
		s.publish(ctx, instance)
	}
}

func mirrorClock(minutes int) string { return fmt.Sprintf("%02d:%02d:00", minutes/60, minutes%60) }
