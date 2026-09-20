package migrations

func init() {
	MigrationRegistry.Register(&Migration{
		Version:      "1.15.399",
		Description:  "Retire student compatibility storage after verified rollback window (#2760)",
		DependsOn:    []string{studentCareAbsenceCompatibilityVersion},
		Precondition: studentOwnerContractPrecondition,
	})
	Migrations.MustRegister(studentOwnerContractUp, studentOwnerContractDown)
}
