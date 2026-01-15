// Package realtime provides Server-Sent Events (SSE) infrastructure for real-time notifications.
package realtime

import (
	"github.com/moto-nrw/project-phoenix/internal/core/port"
)

// Broadcaster is an alias to port.Broadcaster for backward compatibility.
// The canonical interface is defined in internal/core/port/broadcaster.go.
type Broadcaster = port.Broadcaster

// Ensure Hub implements the Broadcaster interface
var _ Broadcaster = (*Hub)(nil)
