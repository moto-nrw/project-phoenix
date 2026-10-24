package audit

import "context"

func NewTenantHandoff(tenantID func(context.Context) int64, isAdmin func(context.Context) bool, transaction func(context.Context) (any, bool)) func(context.Context) (any, bool, int64) {
	return func(ctx context.Context) (any, bool, int64) {
		schoolID := tenantID(ctx)
		if schoolID <= 0 && !isAdmin(ctx) {
			return nil, false, 0
		}
		transaction, ok := transaction(ctx)
		return transaction, ok, schoolID
	}

}
