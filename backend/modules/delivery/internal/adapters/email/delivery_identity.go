package email

import (
	"errors"
	"fmt"
	"strings"

	mailcontract "github.com/moto-nrw/project-phoenix/modules/delivery/mail"
	"github.com/wneessen/go-mail"
)

// headerDeliveryToken carries the random token of a mail. Together with the
// Message-ID it lets a delivery event find the mail. Recipients can read it.
const headerDeliveryToken = mail.Header("X-Moto-Outbox-Id")

// setDeliveryIdentity gives a mail from the outbox an identity that stays the
// same across retries. A mail without a token gets a generated Message-ID.
func setDeliveryIdentity(msg *mail.Msg, message mailcontract.Message) {
	_, domain, hasDomain := strings.Cut(message.From.Address, "@")
	if message.DeliveryToken == "" || !hasDomain || domain == "" {
		msg.SetMessageID()
		return
	}
	msg.SetMessageIDWithValue(message.DeliveryToken + "@" + domain)
	msg.SetGenHeader(headerDeliveryToken, message.DeliveryToken)
}

// classifySendError marks the refusals a retry cannot change. Only a 5xx answer
// of the server to this mail counts:
//   - A network error or 4xx stays retryable.
//   - A 5xx to MAIL FROM says our sender is wrong, not this mail. It stays
//     retryable, so a configuration fix still delivers the queue.
//   - Dial and login errors are no SendError and stay retryable for the same
//     reason.
func classifySendError(err error) error {
	var sendErr *mail.SendError
	if !errors.As(err, &sendErr) || sendErr.ErrorCode() < 500 || sendErr.ErrorCode() > 599 {
		return err
	}
	switch sendErr.Reason {
	case mail.ErrSMTPRcptTo:
		return fmt.Errorf("%w: %w: %w", mailcontract.ErrPermanentRejection, mailcontract.ErrRecipientRejected, err)
	case mail.ErrSMTPData, mail.ErrSMTPDataClose:
		return fmt.Errorf("%w: %w", mailcontract.ErrPermanentRejection, err)
	default:
		return err
	}
}
