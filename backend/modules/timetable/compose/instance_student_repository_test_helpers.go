//go:build testhooks

package compose

import (
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
	"github.com/uptrace/bun"
)

// NewInstanceStudentRepository binds Timetable and Student Presence; preview returns blocks a pickup change excuses.
func NewInstanceStudentRepository(db *bun.DB, capability timetable.InstanceStudentCapability, presence application.ParticipantPresence, preview application.PartialAbsences) scheduleModels.InstanceStudentRepository {
	return application.NewInstanceStudentRepository(retainedPresenceReads(db), capability, presence, preview)
}
