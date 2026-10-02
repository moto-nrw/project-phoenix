package migrations

const presenceContractVersion = "1.15.432"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     presenceContractVersion,
		Description: "Retire the rollback-only presence mirror on schedule.activity_instances and schedule.instance_students (#2763)",
		DependsOn: []string{
			presenceCutoverVersion,
			dropPersonalPINColumnsVersion, // preserves ladder order
		},
		Precondition: presenceContractPrecondition,
	})
	Migrations.MustRegister(presenceContractUp, presenceContractDown)
}
