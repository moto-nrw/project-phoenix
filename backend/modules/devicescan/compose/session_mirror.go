package compose

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/application"
)

type MirroredSession = application.MirroredSession

// SessionMirror records a kiosk session start as a spontaneous timetable
// block.
type SessionMirror = application.SessionMirror

// NewSessionMirror accepts the consumer-owned session seam, bound by the root.
func NewSessionMirror(timetable application.MirrorTimetable, publish application.MirrorPublisher, logger *slog.Logger) application.SessionMirror {
	return application.NewSessionMirror(timetable, publish, logger)
}
