package api

import (
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/services"
)

// NewStudentPresenceForTests composes the Student Presence owner for
// behaviour tests that consume its capability directly, such as the data
// import's retention consents (#2708). The production root composes the same
// module behind its own observation sink.
func NewStudentPresenceForTests(db transactionruntime.Database) studentpresence.Capability {
	return services.NewUnloggedStudentPresence(db.Pool())
}
