package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	staffMasterDataContractDatesVersion     = "1.15.252"
	staffMasterDataContractDatesDescription = "Enforce staff contract dates not preceding entry date"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     staffMasterDataContractDatesVersion,
		Description: staffMasterDataContractDatesDescription,
		DependsOn:   []string{staffStammdatenVersion},
	})
	Migrations.MustRegister(staffMasterDataContractDatesUp, staffMasterDataContractDatesDown)
}

func staffMasterDataContractDatesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.252: Enforcing staff contract date ordering...")
	_, err := db.NewRaw(`
		ALTER TABLE users.staff_master_data
			ADD CONSTRAINT chk_staff_master_data_contract_dates
			CHECK (
				entry_date IS NULL OR
				(contract_end_date IS NULL OR contract_end_date >= entry_date) AND
				(probation_end_date IS NULL OR probation_end_date >= entry_date)
			);
	`).Exec(ctx)
	return err
}

func staffMasterDataContractDatesDown(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`ALTER TABLE users.staff_master_data DROP CONSTRAINT IF EXISTS chk_staff_master_data_contract_dates`).Exec(ctx)
	return err
}
