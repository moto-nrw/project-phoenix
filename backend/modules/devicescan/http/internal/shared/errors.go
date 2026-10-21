// Package shared holds the pieces genuinely used by more than one IoT
// sub-package: the pinned error strings of the card lookups and the
// unregistered-tag audit hook. It is fenced by Go's internal-package rule to
// api/iot/... and keeps a deliberately narrow surface: the strings are the
// public device-scan contract's, and the legacy multi-domain error mapping
// moved next to the session and feedback handlers that still need it (#2698).
package shared

import "github.com/moto-nrw/project-phoenix/modules/devicescan"

// Error message constants for reuse across IoT handlers. PyrePortal branches
// on the registry codes iot.rfid_tag_not_found and iot.person_not_student,
// not on this text; checkIoTErrorCodes (api/iot_error_codes_contract_test.go)
// pins both codes through the production router.
const (
	ErrMsgPersonNotStudent = devicescan.MessagePersonNotStudent
	ErrMsgRFIDTagNotFound  = devicescan.MessageRFIDTagNotFound
	ErrCodeRFIDTagNotFound = devicescan.CodeRFIDTagNotFound
)
