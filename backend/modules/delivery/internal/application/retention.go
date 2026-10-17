package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Retention deletes a school's delivery data after the fixed window (#3955).
type Retention struct {
	store ports.RetentionStore
	now   func() time.Time
}

func NewRetention(store ports.RetentionStore) *Retention {
	if store == nil {
		panic("delivery application: retention requires its store")
	}
	return &Retention{store: store, now: time.Now}
}

// DeleteExpiredDeliveries removes the school's mail and push intents and mail delivery
// logs whose window ended, and clears the key-only welcome mails. The logs go
// first: they point at the mails they record.
func (r *Retention) DeleteExpiredDeliveries(ctx context.Context) (int, int, error) {
	school, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("delivery retention: tenant is required: %w", err)
	}
	tenantID := school.Int64()
	if tenantID <= 0 {
		return 0, 0, errors.New("delivery: tenant is required")
	}
	before := r.now().Add(-domain.RetentionWindow)
	var result domain.RetentionResult
	if result.DeliveryLogs, err = r.store.DeleteExpiredEmailDeliveryLogs(ctx, tenantID, before); err != nil {
		return 0, 0, err
	}
	if result.EmailIntents, err = r.store.DeleteExpiredEmailIntents(ctx, tenantID, before, domain.KeyOnlyTemplates); err != nil {
		return 0, 0, err
	}
	if result.ClearedEmails, err = r.store.ClearExpiredKeyOnlyEmailIntents(ctx, tenantID, before, domain.KeyOnlyTemplates); err != nil {
		return 0, 0, err
	}
	if result.PushIntents, err = r.store.DeleteExpiredPushIntents(ctx, tenantID, before); err != nil {
		return 0, 0, err
	}
	return result.Total(), int(domain.RetentionWindow.Hours() / 24), nil
}
