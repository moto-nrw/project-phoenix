// Package staffstore constructs Communication's native staff-message stores.
package staffstore

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/staffpostgres"
	"github.com/uptrace/bun"
)

func postgresDatabase(db *bun.DB) staffpostgres.Database { return staffpostgres.NewAmbientDatabase(db) }
func ProjectionDatabase(db *bun.DB) func(context.Context) (bun.IDB, int64, error) {
	return staffpostgres.NewAmbientDatabase(db)
}
