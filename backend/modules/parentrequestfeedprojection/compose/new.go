// Package compose builds the parent request feed projection for the roots,
// which bind it to Care Plan's request feed (ADR 0058).
package compose

import parentrequestfeedprojection "github.com/moto-nrw/project-phoenix/modules/parentrequestfeedprojection/internal/adapters/postgres"

type Database = parentrequestfeedprojection.Database
type Reader = parentrequestfeedprojection.Reader

// NewReader builds the projection's reader over the caller's transaction.
func NewReader(database Database) *Reader { return parentrequestfeedprojection.NewReader(database) }
