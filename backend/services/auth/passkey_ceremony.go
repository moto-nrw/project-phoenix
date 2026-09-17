package auth

import (
	"context"
	"database/sql"
	"errors"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// The completion of a passkey ceremony writes twice: it consumes the
// ceremony and then stores the credential or its use. Both portals run that
// pair in one administrative transaction with the same rule, so the rule
// lives here and the operator flow in services/platform uses it too.

// accountMissing separates the account lookup's missing row from a failed
// read: only the missing account refuses the ceremony, a read failure rolls
// the consumption back.
func accountMissing(err error) bool {
	return errors.Is(err, sql.ErrNoRows) || errors.Is(err, modelBase.ErrNotFound)
}

// ceremonyRejection marks a ceremony the verification refused.
type ceremonyRejection struct {
	cause error
}

func (r ceremonyRejection) Error() string { return r.cause.Error() }

func (r ceremonyRejection) Unwrap() error { return r.cause }

// RejectCeremony marks cause as a refusal of the ceremony rather than a
// failure of the request: the consumption commits and cause reaches the
// caller unchanged.
func RejectCeremony(cause error) error {
	return ceremonyRejection{cause: cause}
}

// CompleteCeremony runs a ceremony completion in one administrative
// transaction. A rejection commits: the consumed ceremony stays spent, so
// the same challenge cannot be tried again, and the caller receives the
// rejection's cause. Any other failure rolls back, so the ceremony can be
// completed again after a failed read or write. The unit of work comes from
// the request context, which the API root attaches to every route.
func CompleteCeremony(ctx context.Context, db *bun.DB, fn func(context.Context) error) error {
	var rejected error
	err := tenant.WithAdminTx(ctx, db, func(txCtx context.Context, _ bun.Tx) error {
		rejected = nil
		err := fn(txCtx)
		var rejection ceremonyRejection
		if errors.As(err, &rejection) {
			rejected = rejection.cause
			return nil
		}
		return err
	})
	if err != nil {
		return err
	}
	return rejected
}
