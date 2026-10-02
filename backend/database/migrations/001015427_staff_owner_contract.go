package migrations

const staffOwnerContractVersion = "1.15.427"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:      staffOwnerContractVersion,
		Description:  "Retire staff compatibility storage after the rollback window (#2754)",
		DependsOn:    []string{staffOwnerCutoverVersion, schoolSetupsVersion},
		Precondition: staffOwnerContractPrecondition,
	})
	Migrations.MustRegister(staffOwnerContractUp, staffOwnerContractDown)
}
