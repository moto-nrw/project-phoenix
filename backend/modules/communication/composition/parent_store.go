package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/parentpostgres"
	"github.com/uptrace/bun"
)

func parentDatabase(db *bun.DB) parentpostgres.Database {
	return parentpostgres.NewCompositionParentDatabase(db)
}
