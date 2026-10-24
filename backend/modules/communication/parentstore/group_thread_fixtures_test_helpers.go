//go:build testhooks

package parentstore

import (
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/parentpostgres"
	"github.com/uptrace/bun"
)

// NewGroupThreadStore exposes the group thread store (#4784) to the
// messaging service tests, which compose the service without the root.
func NewGroupThreadStore(db *bun.DB) *parentpostgres.GroupThreadStore {
	return parentpostgres.NewGroupThreadStore(postgresDatabase(db))
}
