// Package shared holds the pieces genuinely used by more than one IoT
// sub-package: the pinned error strings of the card lookups and the
// unregistered-tag audit hook. It is fenced by Go's internal-package rule to
// api/iot/... and keeps a deliberately narrow surface: the strings are the
// public device-scan contract's, and the legacy multi-domain error mapping
// moved next to the session and feedback handlers that still need it (#2698).
package shared

import "github.com/moto-nrw/project-phoenix/modules/devicescan"

// Error message constants for reuse across IoT handlers. All three are part
// of the PyrePortal error contract (see the guard test at api/iot).
const (
	ErrMsgPersonNotStudent = devicescan.MessagePersonNotStudent
	ErrMsgRFIDTagNotFound  = devicescan.MessageRFIDTagNotFound
	ErrCodeRFIDTagNotFound = devicescan.CodeRFIDTagNotFound
)
