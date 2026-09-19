package active

// RecordErrors keeps the legacy error representation at the record adapter.
// MissingRecordError must retain both legacy and SQL not-found classification.
type RecordErrors interface {
	WrapRecordError(operation string, err error) error
	MissingRecordError(operation string) error
}
