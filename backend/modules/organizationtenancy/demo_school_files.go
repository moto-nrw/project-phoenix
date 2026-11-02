package organizationtenancy

import "context"

// DemoSchoolFiles removes the uploads of retired demo schools in the public
// demo (#5544). It reuses the upload namespaces and the removal of the school
// erasure (ADR 0092), not its operator order, receipt or ledger: the demo
// keeps no customer school, and a demo school's database rows stay hidden
// until the demo database is rebuilt.
//
// The work list comes from the database only: every demo school hidden for at
// least an hour that has no file erasure marker and is not the school of an
// order still being seeded. The standing demo school is never selected. Per
// school, every namespace is removed and counted again; only an empty result
// writes the marker. A school that fails keeps no marker and is retried by
// the next run; the other schools of the run go on.
type DemoSchoolFiles interface {
	// EraseRetiredDemoSchoolFiles works through one batch. It returns how many
	// schools it finished and how many objects it removed; the error joins
	// the failures of the schools it could not finish.
	EraseRetiredDemoSchoolFiles(ctx context.Context) (schools int, files int64, err error)
}
