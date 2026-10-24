package httpadapter

import "context"

// Transactions runs the root-bound tenant transaction without exposing its driver.
type Transactions func(context.Context, int64, func(context.Context) error) error

func (run Transactions) WithinTenant(ctx context.Context, id int64, callback func(context.Context) error) error {
	return run(ctx, id, callback)
}
