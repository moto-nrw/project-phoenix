package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const holidayGuestErasureVersion = "1.15.678"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     holidayGuestErasureVersion,
		Description: "Schul-Löschung entfernt die Feriengäste der ausrichtenden OGS (#4666)",
		DependsOn: []string{
			holidayGuestsVersion,           // the three holiday guest tables
			retireDirectPermissionsVersion, // the erasure functions this one extends
		},
	})
	Migrations.MustRegister(holidayGuestErasureUp, holidayGuestErasureDown)
}

func holidayGuestErasureUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		for _, step := range []func(context.Context, bun.Tx) error{
			installHolidayGuestErasurePlan, installHolidayGuestErasurePreflight, installHolidayGuestErasureResidual,
			installHolidayGuestErasureDelete, installHolidayGuestErasurePreview,
		} {
			if err := step(ctx, tx); err != nil {
				return fmt.Errorf("install the holiday guest erasure: %w", err)
			}
		}
		return nil
	})
}

// holidayGuestErasureDown reinstalls the 1.15.646 functions. With the holiday
// guest tables still present they fail closed until 1.15.677 is rolled back.
func holidayGuestErasureDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		for _, step := range []func(context.Context, bun.Tx) error{
			installRetiredDirectPermissionsPlan, installRetiredDirectPermissionsPreflight, installRetiredDirectPermissionsResidual,
			installRetiredDirectPermissionsDelete, installRetiredDirectPermissionsPreview,
		} {
			if err := step(ctx, tx); err != nil {
				return fmt.Errorf("restore the erasure without holiday guests: %w", err)
			}
		}
		return nil
	})
}
