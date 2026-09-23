package migrations

const requestChildStorageContractVersion = "1.15.414"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:      requestChildStorageContractVersion,
		Description:  "Retire request-child compatibility storage after the rollback window (#2719)",
		DependsOn:    []string{requestChildStorageCutoverVersion},
		Precondition: requestChildStorageContractPrecondition,
	})
	Migrations.MustRegister(requestChildStorageContractUp, requestChildStorageContractDown)
}
