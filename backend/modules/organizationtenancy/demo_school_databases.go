package organizationtenancy

import "context"

// DemoSchoolDatabases erases retired demo schools of the public demo from
// the database (#5559, ADR 0189). It reuses the preflight and deletion of the
// school erasure (ADR 0092) through one database function only the demo role
// may call, without the operator's order, receipt or ledger.
//
// A school is due when it has a demo origin, its uploads are erased (#5544),
// it was retired at least 90 days ago and no demo order names it any more.
// The work list comes from the database only. Each school is erased in a
// transaction of its own; a school that fails keeps every row and is retried
// by the next run, and the other schools of the run go on.
type DemoSchoolDatabases interface {
	// EraseRetiredDemoSchoolDatabases works through one batch. It returns how
	// many schools it erased; the error joins the failures of the schools it
	// could not erase.
	EraseRetiredDemoSchoolDatabases(ctx context.Context) (schools int, err error)
}
