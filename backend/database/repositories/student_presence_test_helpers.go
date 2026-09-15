package repositories

import (
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/uptrace/bun"
)

// NewStudentPresenceForTests composes the Student Presence owner for
// behaviour tests that consume its capability directly, such as the data
// import's retention consents (#2708). The production root composes the same
// module behind its own observation sink.
func NewStudentPresenceForTests(db *bun.DB) studentpresence.Capability {
	return newStudentPresence(db)
}
