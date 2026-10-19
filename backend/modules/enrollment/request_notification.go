package enrollment

import (
	"context"
	"fmt"
)

// PinDecisionNotificationMode freezes the delivery choice on the first
// parent-notifiable decision. Later sibling decisions reuse the stored choice.
func (m *Module) PinDecisionNotificationMode(ctx context.Context, requestID int64, proposed string) (string, error) {
	if proposed != "digest" && proposed != "immediate" {
		return "", fmt.Errorf("invalid enrollment decision notification mode %q", proposed)
	}
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return "", err
	}
	return m.engine.PinDecisionNotificationMode(ctx, requestID, proposed)
}
