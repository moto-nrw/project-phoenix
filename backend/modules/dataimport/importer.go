package dataimport

import "context"

// BatchAudit binds a replayable upload to its actor and semantic options.
// Options is fingerprinted and must never be used as a metric label.
type BatchAudit struct {
	EntityType string
	Filename   string
	AccountID  int64
	Options    string
}

// RowImporter is the import execution contract, independent of the workflow's
// concrete implementation and per-row persistence configuration.
type RowImporter[T any] interface {
	Import(context.Context, ImportRequest[T]) (*ImportResult[T], error)
	ImportBatches(context.Context, ImportRequest[T], BatchAudit) (*ImportResult[T], error)
	RecordAuditInTransaction(context.Context, string, string, *ImportResult[T], int64, bool, int64) error
}
