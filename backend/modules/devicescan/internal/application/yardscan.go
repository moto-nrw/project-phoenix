package application

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
)

// routeYardScan applies the Schulhof rule of ADR 0019 (#3282) while blocks
// run in the yard. A child in the day roster of a running block joins that
// block, the newest one on several matches. Any other child stays out of
// the blocks: in a released yard the remaining candidates are the Freispiel
// sessions, and none left means the caller opens one next to the blocks; a
// yard without release keeps every session under the ordinary room rules.
//
// A nil selection hands the returned candidates back to the ordinary choice;
// blocksRun reports that sessions other than Freispiel run in the yard.
// Without them, or without the roster and activity seams, the candidates are
// the given sessions unchanged.
func (s *Service) routeYardScan(ctx context.Context, sessions []ports.Session, room facilities.Room, studentID, deviceID int64) (selection *selectedSession, candidates []ports.Session, blocksRun bool, err error) {
	if s.rosters == nil || s.activities == nil || len(sessions) == 0 {
		return nil, sessions, false, nil
	}
	freeplay, blocks, err := s.splitYardSessions(ctx, sessions)
	if err != nil {
		return nil, nil, false, err
	}
	if len(freeplay) == len(sessions) {
		return nil, sessions, false, nil
	}
	selection, err = s.rosteredBlock(ctx, blocks, room, studentID, deviceID)
	if err != nil || selection != nil {
		return selection, nil, true, err
	}
	if isReleasedSchulhofRoom(room) {
		return nil, freeplay, true, nil
	}
	return nil, sessions, true, nil
}

// openFreeplayNextToBlocks finds or opens the device-less Freispiel session
// of a released yard in which blocks run. The ordinary start refuses an
// occupied room, so it goes through the room session of ADR 0018, which
// phone moves share. The session may be another scan's, so it is never
// marked as created by this one.
func (s *Service) openFreeplayNextToBlocks(ctx context.Context, room facilities.Room) (*selectedSession, error) {
	activity, err := s.schulhofActivity(ctx)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to find Schulhof activity", slog.String("error", err.Error()))
		return nil, devicescan.Internal(devicescan.MessageSchulhofNotConfigured, nil)
	}
	session, err := s.sessions.EnsureRoomSession(ctx, ports.NewSession{ActivityID: activity.ID, RoomID: room.ID})
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to open Schulhof session next to running blocks",
			slog.Int64("room_id", room.ID),
			slog.String("error", err.Error()),
		)
		return nil, specialRoomCreationError(room.Name)
	}
	session.Activity = activity
	s.logger.InfoContext(ctx, "opened Schulhof session next to running blocks",
		slog.Int64("room_id", room.ID),
		slog.Int64("active_group_id", session.ID),
	)
	return &selectedSession{Session: session, RoomName: room.Name}, nil
}

// splitYardSessions separates the Freispiel sessions, which run a system
// activity, from the bookable blocks, which run a regular activity. A
// session without a template is a spontaneous block the kiosk cannot book,
// so it lands in neither list. The loaded activity stays on the session for
// the capacity check.
func (s *Service) splitYardSessions(ctx context.Context, sessions []ports.Session) (freeplay, blocks []ports.Session, err error) {
	loaded := make(map[int64]*ports.Activity, len(sessions))
	for _, session := range sessions {
		if session.TemplateID == nil {
			continue
		}
		activity, ok := loaded[*session.TemplateID]
		if !ok {
			activity, err = s.activities.Find(ctx, *session.TemplateID)
			if err != nil || activity == nil {
				s.logger.ErrorContext(ctx, "failed to get activity of yard session",
					slog.Int64("active_group_id", session.ID),
					slog.Int64("activity_group_id", *session.TemplateID),
					slog.String("error", errorText(err)),
				)
				return nil, nil, devicescan.Internal(devicescan.MessageGetActivityFailed, nil)
			}
			loaded[*session.TemplateID] = activity
		}
		session.Activity = activity
		if activity.IsSystem {
			freeplay = append(freeplay, session)
		} else {
			blocks = append(blocks, session)
		}
	}
	return freeplay, blocks, nil
}

// rosteredBlock returns the newest block whose day roster lists the
// student, or nil when none does.
func (s *Service) rosteredBlock(ctx context.Context, blocks []ports.Session, room facilities.Room, studentID, deviceID int64) (*selectedSession, error) {
	if len(blocks) == 0 {
		return nil, nil
	}
	ids := make([]int64, 0, len(blocks))
	for _, block := range blocks {
		ids = append(ids, block.ID)
	}
	rostered, err := s.rosters.SessionsRosteringStudent(ctx, studentID, ids)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to read block rosters in room",
			slog.Int64("room_id", room.ID),
			slog.Int64("student_id", studentID),
			slog.String("error", err.Error()),
		)
		return nil, devicescan.Internal(devicescan.MessageFindActiveGroupsFailed, nil)
	}
	matched := make(map[int64]bool, len(rostered))
	for _, id := range rostered {
		matched[id] = true
	}
	var selected *ports.Session
	for i := range blocks {
		if matched[blocks[i].ID] && (selected == nil || blocks[i].StartTime.After(selected.StartTime)) {
			selected = &blocks[i]
		}
	}
	if selected == nil {
		return nil, nil
	}
	deviceScoped := selected.DeviceID != nil && *selected.DeviceID == deviceID
	s.logger.DebugContext(ctx, "selected rostered block in room",
		slog.Int("block_count", len(blocks)),
		slog.Int64("room_id", room.ID),
		slog.Int64("active_group_id", selected.ID),
		slog.Bool("device_matched", deviceScoped),
	)
	return &selectedSession{Session: *selected, RoomName: room.Name, DeviceScoped: deviceScoped}, nil
}
