package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
)

// dailyCheckoutTime parses the tenant's daily checkout gate as today's
// instant. Nil means no gate: daily checkout is always available.
func (s *Service) dailyCheckoutTime(ctx context.Context) (*time.Time, error) {
	raw := s.settings.DailyCheckoutTime(ctx)
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ":")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid checkout time format: %s", raw)
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return nil, fmt.Errorf("invalid hour in checkout time: %s", raw)
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return nil, fmt.Errorf("invalid minute in checkout time: %s", raw)
	}
	now := s.now()
	gate := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	return &gate, nil
}

// shouldUpgradeToDailyCheckout reports whether a plain checkout is rewritten
// to "checked_out_daily": the child went home, without being asked.
//
// Deliberately stricter than shouldShowDailyCheckoutWithGroup: the upgrade
// decides FOR the child, so it only fires where leaving is unambiguous (the
// child's own group room). PyrePortal builds its destination modal only for
// "checked_out", so anything upgraded here shows no buttons at all.
func (s *Service) shouldUpgradeToDailyCheckout(ctx context.Context, action string, student *ports.Student, currentVisit *ports.CurrentVisit) bool {
	if action != devicescan.ScanActionCheckedOut {
		return false
	}
	if !dailyCheckoutPreconditions(student, currentVisit) {
		return false
	}
	if !s.isAfterCheckoutTimeGate(ctx, student) {
		return false
	}
	return s.isFromOwnGroupRoom(ctx, student, currentVisit)
}

// dailyCheckoutPreconditions reports whether both daily-checkout gates have
// what they need: a group membership and a resolved session on the visit.
func dailyCheckoutPreconditions(student *ports.Student, currentVisit *ports.CurrentVisit) bool {
	if student == nil || student.GroupID == nil {
		return false
	}
	return currentVisit != nil && currentVisit.Session != nil
}

// isAfterCheckoutTimeGate reports whether the current time is past the gate
// that applies to this student: pickup time minus delta when per-student
// checkout is enabled and the child has a pickup time, otherwise the global
// daily checkout time.
func (s *Service) isAfterCheckoutTimeGate(ctx context.Context, student *ports.Student) bool {
	if gate, ok := s.perStudentCheckoutGate(ctx, student); ok {
		return gate
	}
	return s.isAfterGlobalCheckoutTime(ctx)
}

// perStudentCheckoutGate evaluates the per-student window. The second value
// is false when it does not apply (disabled, no pickup plan, no pickup
// time), which sends the caller to the global gate.
func (s *Service) perStudentCheckoutGate(ctx context.Context, student *ports.Student) (bool, bool) {
	if enabled, err := s.settings.PerStudentCheckoutEnabled(ctx); err != nil || !enabled || s.pickups == nil {
		return false, false
	}
	now := s.now()
	pickup, err := s.pickups.Effective(ctx, student.ID, s.clock.Day(now))
	if err != nil || pickup == nil || pickup.Time == nil {
		return false, false
	}
	delta := 15
	if configured, err := s.settings.PerStudentCheckoutDeltaMinutes(ctx); err == nil {
		delta = configured
	}
	todayPickup := time.Date(now.Year(), now.Month(), now.Day(), pickup.Time.Hour(), pickup.Time.Minute(), 0, 0, now.Location())
	threshold := todayPickup.Add(-time.Duration(delta) * time.Minute)
	return now.After(threshold), true
}

// isAfterGlobalCheckoutTime evaluates the global gate. No configured time
// means no gate.
func (s *Service) isAfterGlobalCheckoutTime(ctx context.Context) bool {
	gate, err := s.dailyCheckoutTime(ctx)
	if err != nil {
		return false
	}
	if gate == nil {
		return true
	}
	return s.now().After(*gate)
}

// shouldShowDailyCheckoutWithGroup reports whether the kiosk may OFFER "nach
// Hause" after this checkout; it drives the daily_checkout_available flag.
//
// Cross-repo contract: PyrePortal only builds the destination modal for
// action "checked_out", so this predicate stays independent of the upgrade
// above. With checkout.daily_checkout_from_all_rooms_enabled every room
// qualifies; otherwise the own-group-room plus Schulhof policy applies.
func (s *Service) shouldShowDailyCheckoutWithGroup(ctx context.Context, student *ports.Student, currentVisit *ports.CurrentVisit) bool {
	if !dailyCheckoutPreconditions(student, currentVisit) {
		return false
	}
	if !s.isAfterCheckoutTimeGate(ctx, student) {
		return false
	}
	if s.dailyCheckoutFromAllRoomsEnabled(ctx) {
		return true
	}
	if s.isFromOwnGroupRoom(ctx, student, currentVisit) {
		return true
	}
	// Checked last: it costs a room lookup, and the common case is a child
	// leaving their own group room.
	return s.isSchulhofRoom(ctx, currentVisit.Session.RoomID)
}

func (s *Service) dailyCheckoutFromAllRoomsEnabled(ctx context.Context) bool {
	enabled, err := s.settings.DailyCheckoutFromAllRoomsEnabled(ctx)
	if err != nil {
		s.logger.WarnContext(ctx, "could not resolve daily-checkout room policy",
			slog.String("error", err.Error()),
		)
		return false
	}
	return enabled
}

// isFromOwnGroupRoom reports whether the visit being closed happened in the
// room of the student's education group. A group without a room is
// unconstrained, so every room counts as its own.
func (s *Service) isFromOwnGroupRoom(ctx context.Context, student *ports.Student, currentVisit *ports.CurrentVisit) bool {
	if s.groups == nil {
		return false
	}
	group, err := s.groups.Find(ctx, *student.GroupID)
	if err != nil || group == nil {
		return false
	}
	if group.RoomID == nil {
		return true
	}
	return currentVisit.Session.RoomID == *group.RoomID
}

// isSchulhofRoom reports whether roomID is this tenant's canonical Schulhof.
// A school that never provisioned the yard has no such room, which is normal
// and simply means no; the scan must not fail over it.
//
// Deliberately NOT switched to the room release (#3064): home checkout is a
// separate permission, and releasing a room must never grant it.
func (s *Service) isSchulhofRoom(ctx context.Context, roomID int64) bool {
	room, err := s.findCanonicalSchulhofRoom(ctx)
	if err != nil {
		if !errors.Is(err, facilities.ErrRoomNotFound) {
			// A non-canonical or unprotected room named "Schulhof" silently
			// disables "nach Hause" at the yard kiosk; worth surfacing.
			s.logger.WarnContext(ctx, "could not resolve Schulhof room for daily-checkout gate",
				slog.Int64("room_id", roomID),
				slog.String("error", err.Error()),
			)
		}
		return false
	}
	return room != nil && room.ID == roomID
}
