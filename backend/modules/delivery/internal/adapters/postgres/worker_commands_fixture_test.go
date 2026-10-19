package postgres_test

import (
	"context"
	"encoding/json"
	"time"

	postgresadapter "github.com/moto-nrw/project-phoenix/modules/delivery/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
)

// The fixture supplies the committed command boundaries formerly opened by
// row adapters. Assertions below still exercise the real PostgreSQL records.
type workerCommandsForTest struct {
	*postgresadapter.Store
	withinAdmin func(context.Context, func(context.Context) error) error
}

func newWorkerCommandsForTest(records *postgresadapter.Store, withinAdmin func(context.Context, func(context.Context) error) error) *workerCommandsForTest {
	if records == nil || withinAdmin == nil {
		panic("delivery: worker records and unit of work are required")
	}
	return &workerCommandsForTest{Store: records, withinAdmin: withinAdmin}
}

func (c *workerCommandsForTest) Claim(ctx context.Context, transport domain.Transport, limit int, now, leaseExpiresAt time.Time) (result0 []domain.Intent, err error) {
	err = c.withinAdmin(ctx, func(txCtx context.Context) error {
		result0, err = c.Store.Claim(txCtx, transport, limit, now, leaseExpiresAt)
		return err
	})
	return
}

func (c *workerCommandsForTest) DeadLetterExpiredDispatches(ctx context.Context, transport domain.Transport, now time.Time) (result0 int64, err error) {
	err = c.withinAdmin(ctx, func(txCtx context.Context) error {
		result0, err = c.Store.DeadLetterExpiredDispatches(txCtx, transport, now)
		return err
	})
	return
}

func (c *workerCommandsForTest) StartDelivery(ctx context.Context, transport domain.Transport, id int64, token string, leaseExpiresAt time.Time) (result0 string, result1 bool, err error) {
	err = c.withinAdmin(ctx, func(txCtx context.Context) error {
		result0, result1, err = c.Store.StartDelivery(txCtx, transport, id, token, leaseExpiresAt)
		return err
	})
	return
}

func (c *workerCommandsForTest) FinalizeSent(ctx context.Context, transport domain.Transport, id int64, token string, providerResult json.RawMessage, sentAt time.Time) (result0 bool, err error) {
	err = c.withinAdmin(ctx, func(txCtx context.Context) error {
		result0, err = c.Store.FinalizeSent(txCtx, transport, id, token, providerResult, sentAt)
		return err
	})
	return
}

func (c *workerCommandsForTest) FinalizeCancelled(ctx context.Context, transport domain.Transport, id int64, token, reason string, cancelledAt time.Time) (result0 bool, err error) {
	err = c.withinAdmin(ctx, func(txCtx context.Context) error {
		result0, err = c.Store.FinalizeCancelled(txCtx, transport, id, token, reason, cancelledAt)
		return err
	})
	return
}

func (c *workerCommandsForTest) FinalizeDeferred(ctx context.Context, transport domain.Transport, id int64, token, reason string, nextRetryAt time.Time) (result0 bool, err error) {
	err = c.withinAdmin(ctx, func(txCtx context.Context) error {
		result0, err = c.Store.FinalizeDeferred(txCtx, transport, id, token, reason, nextRetryAt)
		return err
	})
	return
}

func (c *workerCommandsForTest) FinalizeFailure(ctx context.Context, transport domain.Transport, id int64, token string, attempts int, lastError string, nextRetryAt time.Time, maxAttempts int) (result0 domain.FinalizeResult, err error) {
	err = c.withinAdmin(ctx, func(txCtx context.Context) error {
		result0, err = c.Store.FinalizeFailure(txCtx, transport, id, token, attempts, lastError, nextRetryAt, maxAttempts)
		return err
	})
	return
}
