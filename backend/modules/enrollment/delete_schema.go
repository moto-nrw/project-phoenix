package enrollment

import (
	"errors"
)

var (
	ErrFormSchemaNotFound    = errors.New("form schema not found")
	ErrFormSchemaHasPhases   = errors.New("form schema has enrollment phases")
	ErrFormSchemaHasRequests = errors.New("form schema has enrollment requests")
)
