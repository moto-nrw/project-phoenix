package owners

import (
	"context"

	notificationPorts "github.com/moto-ogs/project-phoenix/workflows/holidayclosurenotification/ports"
)

// HolidayClosureMailRecovery maps durable Delivery outcomes to the workflow's recovery contract.
func (m HolidayClosureMail) HolidayClosureMailRecovery(ctx context.Context, organizationID int64) ([]notificationPorts.ClosureMailRecovery, error) {
	mails, err := m.UnresolvedHolidayClosureEmails(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	result := make([]notificationPorts.ClosureMailRecovery, 0, len(mails))
	for _, mail := range mails {
		result = append(result, notificationPorts.ClosureMailRecovery{Key: mail.IdempotencyKey, Unknown: mail.Unknown})
	}
	return result, nil
}
