//go:build testhooks

package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// NewInstanceStudentRepository binds Timetable and Student Presence; preview returns blocks a pickup change excuses.
func NewInstanceStudentRepository(reads application.PresenceReads, capability timetable.InstanceStudentCapability, presence application.ParticipantPresence, preview application.PartialAbsences) ports.InstanceStudentRecords {
	return application.NewInstanceStudentRepository(reads, capability, presence, preview)
}
