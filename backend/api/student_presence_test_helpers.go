//go:build testhooks

package api

import (
	"time"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// NewStudentPresenceForTests composes the Student Presence owner for
// behaviour tests that consume its capability directly, such as the data
// import's retention consents (#2708). The production root composes the same
// module behind its own observation sink.
func NewStudentPresenceForTests(db transactionruntime.Database) studentpresence.Capability {
	return owners.NewUnloggedStudentPresence(db)
}

// NewPresenceSessionRecords builds the Student Presence session records
// without owner directories: the reads that project rooms, activities or
// devices fail loudly, and supervision reads carry no staff members. The
// optional clock fixes the supervision "today".
func NewPresenceSessionRecords(db transactionruntime.Database, clocks ...func() time.Time) *presenceCompose.SessionRecords {
	return owners.ComposePresenceSessionRecords(presenceCompose.SessionRecordDependencies{DB: db.Pool(), Now: OptionalClock(clocks)})
}
