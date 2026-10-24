package email

import (
	"errors"
	"regexp"
	"strings"
)

// addressInText matches an e-mail address inside free text. go-mail appends
// "affected recipient(s): <address>" to every SendError, and SMTP servers echo
// the rejected mailbox in their reply ("550 5.1.1 <jane@example.com>: User
// unknown"), so an unredacted send error carries the recipient into every log
// line and outbox row that records it (#2108).
var addressInText = regexp.MustCompile(`[A-Za-z0-9.!#$%&'*+/=?^_{|}~-]+@[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)*`)

// redactedError keeps the SMTP error's text readable for diagnosis (status
// code, server reply) while the addresses in it are replaced. Unwrap keeps
// errors.Is and errors.As working on the original.
type redactedError struct {
	err  error
	text string
}

func (e *redactedError) Error() string { return e.text }
func (e *redactedError) Unwrap() error { return e.err }

// redactAddresses returns err with every address in its text replaced by
// "[address]". knownAddresses are the input and canonical recipient forms
// from go-mail, which also cover valid non-dot-atom addresses. The Message-ID
// go-mail reports in a SendError also has the form local@host; it stays,
// because it is how a failed send is found in the provider's logs.
func redactAddresses(err error, knownAddresses ...string) error {
	if err == nil {
		return nil
	}
	text := err.Error()
	messageID := sendErrorMessageID(err)
	redacted := text
	for _, address := range knownAddresses {
		if address != "" {
			redacted = strings.ReplaceAll(redacted, address, "[address]")
		}
	}
	redacted = addressInText.ReplaceAllStringFunc(redacted, func(match string) string {
		if messageID != "" && match == messageID {
			return match
		}
		return "[address]"
	})
	if redacted == text {
		return err
	}
	return &redactedError{err: err, text: redacted}
}

// sendErrorMessageID returns the bare Message-ID (without angle brackets) of
// the message a go-mail SendError reports, or "".
func sendErrorMessageID(err error) string {
	var withID interface{ MessageID() string }
	if !errors.As(err, &withID) {
		return ""
	}
	return strings.TrimSuffix(strings.TrimPrefix(withID.MessageID(), "<"), ">")
}
