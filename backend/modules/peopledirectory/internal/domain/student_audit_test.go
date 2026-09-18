package domain

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The cases below moved here with the change-history diff (#3349); they
// previously drove services/users.diffStudentFields.

func strPtr(s string) *string { return &s }

// changeByField indexes a diff result by field name for convenient assertions.
func changeByField(changes []StudentFieldChange) map[string]StudentFieldChange {
	result := make(map[string]StudentFieldChange, len(changes))
	for _, change := range changes {
		result[change.FieldName] = change
	}
	return result
}

// TestDiffStudentFieldsNoChange verifies that an identical before/after pair,
// including equivalent nil and empty-string values, records nothing.
func TestDiffStudentFieldsNoChange(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		before, after StudentAuditSnapshot
	}{
		{
			name:   "identical empty students",
			before: StudentAuditSnapshot{Status: "active"},
			after:  StudentAuditSnapshot{Status: "active"},
		},
		{
			name:   "nil vs empty-string note are equal",
			before: StudentAuditSnapshot{SupervisorNotes: nil},
			after:  StudentAuditSnapshot{SupervisorNotes: strPtr("")},
		},
		{
			name:   "no recorded care end on either side",
			before: StudentAuditSnapshot{},
			after:  StudentAuditSnapshot{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Empty(t, DiffStudentFields(tc.before, tc.after))
		})
	}
}

// TestDiffStudentFieldsPerField verifies each tracked field produces exactly
// one change with the correct field token and display-ready German old/new
// values.
func TestDiffStudentFieldsPerField(t *testing.T) {
	t.Parallel()

	t.Run("status maps to German labels", func(t *testing.T) {
		changes := DiffStudentFields(
			StudentAuditSnapshot{Status: "pending"},
			StudentAuditSnapshot{Status: "active"},
		)
		require.Len(t, changes, 1)
		assert.Equal(t, StudentFieldStatus, changes[0].FieldName)
		assert.Equal(t, "Ausstehend", changes[0].OldValue)
		assert.Equal(t, "Aktiv", changes[0].NewValue)
	})

	t.Run("supervisor notes from empty to value", func(t *testing.T) {
		changes := DiffStudentFields(
			StudentAuditSnapshot{},
			StudentAuditSnapshot{SupervisorNotes: strPtr("Allergie beachten")},
		)
		require.Len(t, changes, 1)
		assert.Equal(t, StudentFieldSupervisorNotes, changes[0].FieldName)
		assert.Equal(t, "", changes[0].OldValue)
		assert.Equal(t, "Allergie beachten", changes[0].NewValue)
	})

	t.Run("extra info and health info change independently", func(t *testing.T) {
		changes := DiffStudentFields(
			StudentAuditSnapshot{ExtraInfo: strPtr("alt"), HealthInfo: strPtr("gesund")},
			StudentAuditSnapshot{ExtraInfo: strPtr("neu"), HealthInfo: strPtr("gesund")},
		)
		require.Len(t, changes, 1, "only extra_info changed")
		assert.Equal(t, StudentFieldExtraInfo, changes[0].FieldName)
	})

	t.Run("surrounding whitespace remains auditable", func(t *testing.T) {
		changes := DiffStudentFields(
			StudentAuditSnapshot{
				SupervisorNotes: strPtr("Hinweis"),
				ExtraInfo:       strPtr("Info"),
				HealthInfo:      strPtr("Gesund"),
			},
			StudentAuditSnapshot{
				SupervisorNotes: strPtr(" Hinweis "),
				ExtraInfo:       strPtr(" Info "),
				HealthInfo:      strPtr(" Gesund "),
			},
		)
		byField := changeByField(changes)
		require.Len(t, changes, 3)
		assert.Equal(t, " Hinweis ", byField[StudentFieldSupervisorNotes].NewValue)
		assert.Equal(t, " Info ", byField[StudentFieldExtraInfo].NewValue)
		assert.Equal(t, " Gesund ", byField[StudentFieldHealthInfo].NewValue)
	})

	t.Run("pickup status change", func(t *testing.T) {
		changes := DiffStudentFields(
			StudentAuditSnapshot{PickupStatus: strPtr("Allein")},
			StudentAuditSnapshot{PickupStatus: strPtr("Wird abgeholt")},
		)
		require.Len(t, changes, 1)
		assert.Equal(t, StudentFieldPickupStatus, changes[0].FieldName)
		assert.Equal(t, "Wird abgeholt", changes[0].NewValue)
	})

	t.Run("care end renders a German date", func(t *testing.T) {
		changes := DiffStudentFields(
			StudentAuditSnapshot{},
			StudentAuditSnapshot{CareEnd: "2026-07-31"},
		)
		require.Len(t, changes, 1)
		assert.Equal(t, StudentFieldCareEnd, changes[0].FieldName)
		assert.Equal(t, "", changes[0].OldValue)
		assert.Equal(t, "31.07.2026", changes[0].NewValue)
	})

	t.Run("departure plan change renders a stable weekday label", func(t *testing.T) {
		changes := DiffStudentFields(
			StudentAuditSnapshot{DepartureDays: departure.DepartureDays{}},
			StudentAuditSnapshot{DepartureDays: departure.DepartureDays{"mon": departure.DepartureBus}},
		)
		require.Len(t, changes, 1)
		assert.Equal(t, StudentFieldDepartureDays, changes[0].FieldName)
		assert.NotEqual(t, changes[0].OldValue, changes[0].NewValue)
		assert.Contains(t, changes[0].NewValue, "Mo: Bus")
		assert.Contains(t, changes[0].OldValue, "Mo: Allein")
	})

	t.Run("departure plan preserves secondary allowed modes", func(t *testing.T) {
		changes := DiffStudentFields(
			StudentAuditSnapshot{AllowedDepartureModes: departure.AllowedDepartureModes{
				"mon": {departure.DepartureBus},
			}},
			StudentAuditSnapshot{AllowedDepartureModes: departure.AllowedDepartureModes{
				"mon": {departure.DepartureBus, departure.DepartureAlone},
			}},
		)
		require.Len(t, changes, 1)
		assert.Equal(t, StudentFieldDepartureDays, changes[0].FieldName)
		assert.Contains(t, changes[0].OldValue, "Mo: Bus")
		assert.Contains(t, changes[0].NewValue, "Mo: Allein / Bus")
	})

	t.Run("departure companion note change", func(t *testing.T) {
		changes := DiffStudentFields(
			StudentAuditSnapshot{DepartureCompanionNote: strPtr("Geschwisterkind Mia")},
			StudentAuditSnapshot{DepartureCompanionNote: nil},
		)
		require.Len(t, changes, 1)
		assert.Equal(t, StudentFieldDepartureCompanionNote, changes[0].FieldName)
		assert.Equal(t, "Geschwisterkind Mia", changes[0].OldValue)
		assert.Equal(t, "", changes[0].NewValue)
	})
}

// TestDiffStudentFieldsMultipleFields verifies several simultaneous changes
// each record their own entry.
func TestDiffStudentFieldsMultipleFields(t *testing.T) {
	t.Parallel()

	changes := DiffStudentFields(
		StudentAuditSnapshot{Status: "active", SupervisorNotes: strPtr("alt")},
		StudentAuditSnapshot{Status: "inactive", SupervisorNotes: strPtr("neu")},
	)
	byField := changeByField(changes)
	require.Len(t, changes, 2)
	assert.Contains(t, byField, StudentFieldStatus)
	assert.Contains(t, byField, StudentFieldSupervisorNotes)
	assert.Equal(t, "Inaktiv", byField[StudentFieldStatus].NewValue)
}
