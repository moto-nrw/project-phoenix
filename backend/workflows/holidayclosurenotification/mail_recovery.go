package holidayclosurenotification

import (
	"context"
	"fmt"
)

// mailRecovery includes mail without a school event, while preserving the
// same per-mail idempotency and unknown-outcome boundary as school retries.
func (s *Service) mailRecovery(ctx context.Context, organizationID int64, pending *Pending) ([]string, error) {
	mails, err := s.Mails.HolidayClosureMailRecovery(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var tokens []string
	for _, mail := range mails {
		tokens = append(tokens, fmt.Sprintf("mail:%s:%t", mail.Key, mail.Unknown))
		if mail.Unknown {
			pending.UnknownMails++
		} else {
			pending.RetryableMails++
			pending.mailKeys = append(pending.mailKeys, mail.Key)
		}
	}
	return tokens, nil
}

func (s *Service) retryPendingMails(ctx context.Context, organizationID int64, keys []string) (retried int64, err error) {
	if len(keys) == 0 {
		return 0, nil
	}
	err = s.Units.WithinTraeger(ctx, organizationID, func(ctx context.Context) error {
		var err error
		retried, err = s.Mails.RetryFailedTraegerEmails(ctx, organizationID, keys)
		return err
	})
	return retried, err
}
