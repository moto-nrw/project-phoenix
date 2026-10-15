package application

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/ports"
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

// DeleteExpired removes the school's mail and push intents and mail delivery
// logs whose window ended, and clears the key-only welcome mails. The logs go
// first: they point at the mails they record.
func (r *Retention) DeleteExpired(ctx context.Context, tenantID int64) (domain.RetentionResult, error) {
	if tenantID <= 0 {
		return domain.RetentionResult{}, errors.New("delivery: tenant is required")
	}
	before := r.now().Add(-domain.RetentionWindow)
	var result domain.RetentionResult
	var err error
	if result.DeliveryLogs, err = r.store.DeleteExpiredEmailDeliveryLogs(ctx, tenantID, before); err != nil {
		return domain.RetentionResult{}, err
	}
	if result.EmailIntents, err = r.store.DeleteExpiredEmailIntents(ctx, tenantID, before, domain.KeyOnlyTemplates); err != nil {
		return domain.RetentionResult{}, err
	}
	if result.ClearedEmails, err = r.store.ClearExpiredKeyOnlyEmailIntents(ctx, tenantID, before, domain.KeyOnlyTemplates); err != nil {
		return domain.RetentionResult{}, err
	}
	if result.PushIntents, err = r.store.DeleteExpiredPushIntents(ctx, tenantID, before); err != nil {
		return domain.RetentionResult{}, err
	}
	return result, nil
}
