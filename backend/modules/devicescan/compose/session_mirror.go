package compose

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/application"
)

type MirroredSession = application.MirroredSession

// SessionMirror records a kiosk session start as a spontaneous timetable
// block.
type SessionMirror = application.SessionMirror

// NewSessionMirror accepts the consumer-owned session seam, bound by the root.
func NewSessionMirror(timetable interface {
	HasInstance(context.Context, int64) (bool, error)
	CreateInstance(context.Context, MirroredSession) (MirroredSession, error)
	AddSessionInstanceStaff(context.Context, int64, int64) error
	ActivityTitle(context.Context, int64) (string, error)
	CalendarParts(time.Time) (string, int)
}, publish application.MirrorPublisher, logger *slog.Logger) application.SessionMirror {
	return application.NewSessionMirror(timetable, publish, logger)
}
