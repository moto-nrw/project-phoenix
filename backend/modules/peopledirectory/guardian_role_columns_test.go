package peopledirectory_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/stretchr/testify/assert"
)

func TestGuardianRoleColumnsKeepTheOtherRelationshipFields(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []string{"relationship_type", "is_emergency_contact", "can_pickup", "emergency_priority", "guardian_role", "permissions"},
		peopledirectory.GuardianRoleColumns())
}

func TestStudentRecordCareDayRules(t *testing.T) {
	t.Parallel()
	running := peopledirectory.StudentRecord{Status: peopledirectory.StudentStatusActive}
	ended := peopledirectory.StudentRecord{Status: peopledirectory.StudentStatusActive, EnrolledUntil: "2026-10-16"}
	alumnus := peopledirectory.StudentRecord{Status: peopledirectory.StudentStatusAlumnus}
	pending := peopledirectory.StudentRecord{Status: peopledirectory.StudentStatusPending}

	assert.True(t, running.IsActive())
	assert.False(t, pending.IsActive())
	assert.False(t, running.CareEndedOn("2026-10-19"))
	assert.False(t, ended.CareEndedOn("2026-10-16"), "the last care day is still in care")
	assert.True(t, ended.CareEndedOn("2026-10-17"))
	assert.True(t, running.PickupEligibleOn("2026-10-19"))
	assert.False(t, ended.PickupEligibleOn("2026-10-19"))
	assert.False(t, alumnus.PickupEligibleOn("2026-10-19"))
}
