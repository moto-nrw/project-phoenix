package audit

import "context"

// NewTenantHandoff hands the Audit repositories the transaction of a school
// or an administrative unit of work; outside them it hands none.
func NewTenantHandoff(tenantID func(context.Context) int64, isAdmin func(context.Context) bool, transaction func(context.Context) (any, bool)) func(context.Context) (any, bool, int64) {
	return NewAppendHandoff(tenantID, isAdmin, nil, transaction)
}

// NewAppendHandoff also hands over the Träger transaction of ADR 0105, with no
// school (#4692): a tenantless ledger such as the Träger document downloads
// joins it, while a school ledger still finds no school and is refused. The
// Träger policy and the request role keep the write to the Träger's own rows.
func NewAppendHandoff(tenantID func(context.Context) int64, isAdmin func(context.Context) bool, traeger func(context.Context) int64, transaction func(context.Context) (any, bool)) func(context.Context) (any, bool, int64) {
	return func(ctx context.Context) (any, bool, int64) {
		schoolID := tenantID(ctx)
		inTraeger := traeger != nil && traeger(ctx) > 0
		if schoolID <= 0 && !isAdmin(ctx) && !inTraeger {
			return nil, false, 0
		}
		transaction, ok := transaction(ctx)
		return transaction, ok, schoolID
	}
}
