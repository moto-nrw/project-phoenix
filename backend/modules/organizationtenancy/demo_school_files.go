package organizationtenancy

import "context"

// DemoSchoolErasure erases retired demo schools of the public demo: first
// their uploads (#5544, ADR 0148), then their database rows (#5559,
// ADR 0189). It reuses the upload namespaces, preflight and deletion of the
// school erasure (ADR 0092), not its operator order, receipt or ledger: the
// demo keeps no customer school.
//
// Both work lists come from the database only. Files are due for every school
// hidden for at least an hour with a recorded demo origin, no file erasure
// marker, and not the school of an order still being seeded. The standing
// demo school is never selected. Per school, every namespace is removed and
// counted again; only an empty result writes the marker.
//
// The database rows are due when the school has a demo origin and the file
// marker, was retired at least 90 days ago and no demo order names it any
// more. Each school is erased in a transaction of its own. A school whose
// tables another transaction locks for more than 100 ms is busy and waits for
// the next run without failing this one.
//
// In both steps a school that fails keeps what it had and is retried by the
// next run; the other schools of the run go on.
type DemoSchoolErasure interface {
	// EraseRetiredDemoSchoolFiles works through one batch. It returns how many
	// schools it finished and how many objects it removed; the error joins
	// the failures of the schools it could not finish.
	EraseRetiredDemoSchoolFiles(ctx context.Context) (schools int, files int64, err error)
	// EraseRetiredDemoSchoolDatabases works through one batch. It returns how
	// many schools it erased and how many were busy; the error joins the
	// failures of the schools it could not erase.
	EraseRetiredDemoSchoolDatabases(ctx context.Context) (erased, busy int, err error)
}
