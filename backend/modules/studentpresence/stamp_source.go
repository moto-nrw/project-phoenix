package studentpresence

// StampSource is the channel an automatic work-session check-in names: a web
// request stamps as the app, a kiosk-driven one as NFC. Student Presence sits
// below Workforce (ADR 0058, #3417); the root maps the channel onto
// Workforce's stamp source.
type StampSource int

// The stamp channels of the automatic check-in.
const (
	StampApp StampSource = iota + 1
	StampNFC
)

// String names the channel in logs.
func (s StampSource) String() string {
	switch s {
	case StampApp:
		return "app"
	case StampNFC:
		return "nfc"
	default:
		return "unknown"
	}
}
