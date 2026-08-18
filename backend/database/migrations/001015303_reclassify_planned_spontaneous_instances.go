package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	reclassifyPlannedSpontaneousInstancesVersion     = "1.15.303"
	reclassifyPlannedSpontaneousInstancesDescription = "Reclassify planned spontaneous activity instances as planned blocks"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     reclassifyPlannedSpontaneousInstancesVersion,
		Description: reclassifyPlannedSpontaneousInstancesDescription,
		DependsOn:   []string{createActivityInstancesVersion},
	})
	Migrations.MustRegister(reclassifyPlannedSpontaneousInstancesUp, reclassifyPlannedSpontaneousInstancesDown)
}

// is_spontaneous now records the creation origin, not the offering link
// (#2299). Genuinely spontaneous instances are created ad-hoc AT START — the
// web spontaneous flow creates and starts in one transaction and the kiosk
// mirror inserts status='active' directly — so no persisted row is ever both
// spontaneous and still 'planned'. Every planned row flagged spontaneous is
// therefore a planning-module block that only lacked an offering link; flip it
// so the lifecycle time guards apply. Started/completed/cancelled rows keep
// their flag: their origin is ambiguous and no start guard applies to them.
func reclassifyPlannedSpontaneousInstancesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.303: Reclassifying planned spontaneous activity instances as planned blocks...")
	res, err := db.NewRaw(`
		UPDATE schedule.activity_instances
		SET is_spontaneous = FALSE
		WHERE is_spontaneous = TRUE
		  AND status = 'planned'
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("reclassify planned spontaneous instances: %w", err)
	}
	if rows, err := res.RowsAffected(); err == nil {
		fmt.Printf("Migration 1.15.303: reclassified %d instance(s)\n", rows)
	}
	return nil
}

func reclassifyPlannedSpontaneousInstancesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.303: nothing to restore (the pre-#2299 flag values are not recoverable)")
	return nil
}
