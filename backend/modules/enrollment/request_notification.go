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
	var mode string
	err := m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		var err error
		mode, err = m.engine.PinDecisionNotificationMode(ctx, requestID, proposed)
		return err
	})
	return mode, err
}
