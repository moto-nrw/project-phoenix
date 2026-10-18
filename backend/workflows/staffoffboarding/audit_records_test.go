package staffoffboarding_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/workflows/staffoffboarding"
)

func TestNewAbsenceDeletionKeepsTheDeletedRowAsPayload(t *testing.T) {
	t.Parallel()
	staffID, absenceID, actorID := int64(41), int64(73), int64(29)
	absence := workforce.StaffAbsence{ID: absenceID, StaffID: staffID, AbsenceType: "vacation", DateStart: "2026-10-12", DateEnd: "2026-10-16"}

	deletion, err := staffoffboarding.NewAbsenceDeletion(absence, actorID)

	require.NoError(t, err)
	payload, err := json.Marshal(absence)
	require.NoError(t, err)
	assert.Equal(t, staffoffboarding.AbsenceDeletion{
		StaffID: staffID, AbsenceID: absenceID, DeletedBy: actorID, Payload: payload, Note: "Personal-Offboarding",
	}, deletion)
}

func TestNewStaffDeletionCountsTheRemovedRowsAndTheStaffMember(t *testing.T) {
	t.Parallel()
	tenantID, staffID := int64(17), int64(41)
	result := staffoffboarding.Result{
		StaffID:    staffID,
		Membership: schoolmembership.Retirement{GroupAssignments: 10, ClassAssignments: 11},
		Timetable:  timetable.OffboardingCounts{PlannedSupervisors: 12, InstanceAssignments: 13},
		Workforce:  workforce.OffboardingCounts{Substitutions: 14, Shifts: 15, Series: 16, Absences: 17},
		Access:     staffoffboarding.AccessResult{PermissionsRevoked: 18},
	}

	deletion := staffoffboarding.NewStaffDeletion(staffoffboarding.Actor{TenantID: tenantID, Username: "Frau Weber"}, result)

	assert.Equal(t, staffoffboarding.StaffDeletion{
		TenantID: tenantID, StaffID: staffID, RecordsDeleted: 1 + 10 + 11 + 12 + 13 + 14 + 15 + 16 + 17 + 18,
		DeletedBy: "Frau Weber", Reason: "staff offboarding",
		Counts: map[string]int64{
			"group_teacher": 10, "class_teachers": 11, "activity_supervisors": 12, "timetable_instance_staff": 13,
			"group_substitutions": 14, "staff_shifts": 15, "staff_shift_series": 16, "staff_absences": 17,
			"account_permissions": 18,
		},
	}, deletion)
}

func TestNewStaffDeletionWithoutDisplayNameIsWrittenBySystem(t *testing.T) {
	t.Parallel()

	deletion := staffoffboarding.NewStaffDeletion(staffoffboarding.Actor{TenantID: 17}, staffoffboarding.Result{StaffID: 41})

	assert.Equal(t, "system", deletion.DeletedBy)
	assert.Equal(t, 1, deletion.RecordsDeleted)
}
