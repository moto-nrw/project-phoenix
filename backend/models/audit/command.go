package audit

import "context"

// Command is the single append capability for Audit-owned ledgers. Callers
// pass the event while their authoritative transaction is still active; an
// append error must therefore abort that producer transaction.
type Command interface {
	Append(context.Context, any) error
}

// AppendOnceCommand is the idempotent companion capability for ledgers with
// a database uniqueness constraint. It reports whether this call inserted the
// event while preserving the same append-only command boundary.
type AppendOnceCommand interface {
	AppendOnce(context.Context, any) (bool, error)
}

// AppendStore is the persistence port implemented by the Audit Postgres
// adapter. Composition code supplies its transaction resolver explicitly.
type AppendStore interface {
	Append(context.Context, any) error
}
