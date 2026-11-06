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

// classifySendError marks the refusals a retry cannot change. A 5xx alone does
// not say that: servers also answer with it to a blocked sender, an exhausted
// quota or missing authentication, and calling those final would put a whole
// queue's addresses on the suppression list. Only the enhanced status of the
// answer names the cause:
//   - 5.1.1/2/3/4/6/10 at RCPT TO or DATA: the destination address is unusable.
//   - 5.6.x at DATA: this mail's content cannot be delivered.
//
// Everything else stays retryable, up to the mail's attempts: network errors,
// 4xx, other 5xx and answers of servers without enhanced status codes. Dial and
// login errors are no SendError and stay retryable for the same reason.
func classifySendError(err error) error {
	var sendErr *mail.SendError
	if !errors.As(err, &sendErr) || sendErr.ErrorCode() < 500 || sendErr.ErrorCode() > 599 {
		return err
	}
	enhanced := sendErr.EnhancedStatusCode()
	atData := sendErr.Reason == mail.ErrSMTPData || sendErr.Reason == mail.ErrSMTPDataClose
	// Other 5.1.x statuses describe sender failures, valid addresses or
	// unspecified causes; none proves that the recipient must be suppressed.
	recipientRejected := enhanced == "5.1.1" || enhanced == "5.1.2" || enhanced == "5.1.3" ||
		enhanced == "5.1.4" || enhanced == "5.1.6" || enhanced == "5.1.10"
	switch {
	case recipientRejected && (sendErr.Reason == mail.ErrSMTPRcptTo || atData):
		return fmt.Errorf("%w: %w: %w", mailcontract.ErrPermanentRejection, mailcontract.ErrRecipientRejected, err)
	case strings.HasPrefix(enhanced, "5.6.") && atData:
		return fmt.Errorf("%w: %w", mailcontract.ErrPermanentRejection, err)
	default:
		return err
	}
}
