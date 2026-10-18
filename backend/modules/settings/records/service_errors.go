package records

import "errors"

var ErrRuntimeUnavailable = errors.New("settings runtime is not configured")

// ErrHomeLayoutUnavailable is returned when the service was built without the
// repository it needs.
var ErrHomeLayoutUnavailable = errors.New("home layout store is not configured")

// errPresenceModeSwitchBlockedMsg is the German user-facing copy returned when
// an operator tries to flip operations.presence_mode while students are still
// checked in for the day.
//
//nolint:staticcheck // ST1005: German user-facing message
const errPresenceModeSwitchBlockedMsg = "Moduswechsel während aktiver Anwesenheit nicht möglich. Bitte zunächst Tagesabschluss durchführen."

// ErrPresenceModeSwitchBlocked is the sentinel returned by
// CheckPresenceModeSwitch so callers can branch on it via errors.Is without
// relying on string equality. Mirrors the SettingsService error pattern
// (DefinitionNotFoundError / InvalidValueError).
//
//nolint:staticcheck // ST1005: German user-facing message
var ErrPresenceModeSwitchBlocked = errors.New(errPresenceModeSwitchBlockedMsg)
