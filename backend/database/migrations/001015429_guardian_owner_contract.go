package migrations

const guardianOwnerContractVersion = "1.15.429"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:      guardianOwnerContractVersion,
		Description:  "Retire the users.students_guardians rollback mirror after the rollback window (#2757)",
		DependsOn:    []string{guardianOwnerCutoverVersion, parentDeclarationsVersion},
		Precondition: guardianOwnerContractPrecondition,
	})
	Migrations.MustRegister(guardianOwnerContractUp, guardianOwnerContractDown)
}
