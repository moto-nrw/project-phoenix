package migrations

import (
	"context"
	"errors"

	"github.com/uptrace/bun"
)

const requestChildStorageCutoverVersion = "1.15.385"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     requestChildStorageCutoverVersion,
		Description: "Cut over submitted offering choices and effective care bookings with previous-image compatibility (#2714)",
		DependsOn:   []string{requestChildStorageBackfillVersion},
	})
	Migrations.MustRegister(requestChildStorageCutoverUp, requestChildStorageCutoverDown)
}

func requestChildStorageCutoverUp(ctx context.Context, db *bun.DB) error {
	return finalizeRequestChildStorage(ctx, db, installRequestChildStorageCompatibility)
}

func requestChildStorageCutoverDown(context.Context, *bun.DB) error {
	return errors.New("request child storage cutover rollback deploys the previous application image; retain the compatibility schema")
}
