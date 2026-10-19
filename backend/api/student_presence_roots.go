package api

import (
	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// The roots below compose Student Presence, its session records and the
// joined Timetable presence reads from the database (#4010). Most bind
// several owners' compositions over the same database (Student Presence,
// Timetable, Care Plan, People Directory, Facilities), so the root builds
// them where it mounts them.

// NewPickupReviewBlocks previews the blocks a pickup change under review
// would excuse over the joined presence reads of the database.
func NewPickupReviewBlocks(db transactionruntime.Database) carePlanCompose.PickupReviewBlocks {
	return PickupReviewBlocks{reads: owners.MustPresenceReads(db)}
}
