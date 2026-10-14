package domain

// PrecedingMailState is the delivery state of the mail a guardian welcome
// follows: the invitation or the access notice that granted the new access
// (#3534).
type PrecedingMailState int

const (
	// PrecedingMailQueued has not been accepted by the mail transport yet.
	PrecedingMailQueued PrecedingMailState = iota
	// PrecedingMailSent was accepted by the mail transport.
	PrecedingMailSent
	// PrecedingMailCancelled was withdrawn before it was sent.
	PrecedingMailCancelled
	// PrecedingMailUndeliverable exhausted its delivery attempts.
	PrecedingMailUndeliverable
)

// GuardianWelcomeOrder decides whether the welcome follows its preceding
// mail. The welcome goes out only after the transport accepted that mail, so
// a guardian never reads "welcome" before the mail that lets them in. It is
// dropped when that mail can no longer arrive: withdrawn, undeliverable, or
// gone. found is false when the preceding mail no longer exists.
func GuardianWelcomeOrder(state PrecedingMailState, found bool) (send, drop bool) {
	if !found {
		return false, true
	}
	switch state {
	case PrecedingMailSent:
		return true, false
	case PrecedingMailCancelled, PrecedingMailUndeliverable:
		return false, true
	default:
		return false, false
	}
}
