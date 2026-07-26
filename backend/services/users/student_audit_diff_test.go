package users

import (
	"testing"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func strPtr(s string) *string { return &s }

// editByField indexes a diff result by field name for convenient assertions.
func editByField(edits []*auditModels.StudentFieldEdit) map[string]*auditModels.StudentFieldEdit {
	m := make(map[string]*auditModels.StudentFieldEdit, len(edits))
	for _, e := range edits {
		m[e.FieldName] = e
	}
	return m
}

// TestDiffStudentFields_NoChange verifies that an identical before/after pair
// (including pointer-vs-value and whitespace equivalence) records nothing.
func TestDiffStudentFields_NoChange(t *testing.T) {
	cases := []struct {
		name          string
		before, after *userModels.Student
	}{
		{
			name:   "identical empty students",
			before: &userModels.Student{Status: userModels.StudentStatusActive},
			after:  &userModels.Student{Status: userModels.StudentStatusActive},
		},
		{
			name:   "nil vs empty-string note are equal",
			before: &userModels.Student{SupervisorNotes: nil},
			after:  &userModels.Student{SupervisorNotes: strPtr("")},
		},
		{
			name:   "whitespace-only difference is trimmed away",
			before: &userModels.Student{ExtraInfo: strPtr("Hallo")},
			after:  &userModels.Student{ExtraInfo: strPtr("  Hallo  ")},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Empty(t, diffStudentFields(tc.before, tc.after))
		})
	}
}

// TestDiffStudentFields_PerField verifies each tracked field produces exactly
// one edit with the correct field token and display-ready German old/new
// values.
func TestDiffStudentFields_PerField(t *testing.T) {
	t.Run("status maps to German labels", func(t *testing.T) {
		edits := diffStudentFields(
			&userModels.Student{Status: userModels.StudentStatusPending},
			&userModels.Student{Status: userModels.StudentStatusActive},
		)
		require.Len(t, edits, 1)
		e := edits[0]
		assert.Equal(t, auditModels.StudentFieldStatus, e.FieldName)
		assert.Equal(t, "Ausstehend", *e.OldValue)
		assert.Equal(t, "Aktiv", *e.NewValue)
	})

	t.Run("supervisor notes from empty to value", func(t *testing.T) {
		edits := diffStudentFields(
			&userModels.Student{},
			&userModels.Student{SupervisorNotes: strPtr("Allergie beachten")},
		)
		require.Len(t, edits, 1)
		assert.Equal(t, auditModels.StudentFieldSupervisorNotes, edits[0].FieldName)
		assert.Equal(t, "", *edits[0].OldValue)
		assert.Equal(t, "Allergie beachten", *edits[0].NewValue)
	})

	t.Run("extra info and health info change independently", func(t *testing.T) {
		edits := diffStudentFields(
			&userModels.Student{ExtraInfo: strPtr("alt"), HealthInfo: strPtr("gesund")},
			&userModels.Student{ExtraInfo: strPtr("neu"), HealthInfo: strPtr("gesund")},
		)
		require.Len(t, edits, 1, "only extra_info changed")
		assert.Equal(t, auditModels.StudentFieldExtraInfo, edits[0].FieldName)
	})

	t.Run("pickup status change", func(t *testing.T) {
		edits := diffStudentFields(
			&userModels.Student{PickupStatus: strPtr("Allein")},
			&userModels.Student{PickupStatus: strPtr("Wird abgeholt")},
		)
		require.Len(t, edits, 1)
		assert.Equal(t, auditModels.StudentFieldPickupStatus, edits[0].FieldName)
		assert.Equal(t, "Wird abgeholt", *edits[0].NewValue)
	})

	t.Run("departure plan change renders a stable weekday label", func(t *testing.T) {
		edits := diffStudentFields(
			&userModels.Student{DepartureDays: userModels.DepartureDays{}},
			&userModels.Student{DepartureDays: userModels.DepartureDays{"mon": userModels.DepartureBus}},
		)
		require.Len(t, edits, 1)
		e := edits[0]
		assert.Equal(t, auditModels.StudentFieldDepartureDays, e.FieldName)
		assert.NotEqual(t, *e.OldValue, *e.NewValue)
		assert.Contains(t, *e.NewValue, "Mo: Bus")
		assert.Contains(t, *e.OldValue, "Mo: Allein")
	})

	t.Run("departure plan preserves secondary allowed modes", func(t *testing.T) {
		edits := diffStudentFields(
			&userModels.Student{AllowedDepartureModes: userModels.AllowedDepartureModes{
				"mon": {userModels.DepartureBus},
			}},
			&userModels.Student{AllowedDepartureModes: userModels.AllowedDepartureModes{
				"mon": {userModels.DepartureBus, userModels.DepartureAlone},
			}},
		)
		require.Len(t, edits, 1)
		e := edits[0]
		assert.Equal(t, auditModels.StudentFieldDepartureDays, e.FieldName)
		assert.Contains(t, *e.OldValue, "Mo: Bus")
		assert.Contains(t, *e.NewValue, "Mo: Allein / Bus")
	})

	t.Run("departure companion note change", func(t *testing.T) {
		edits := diffStudentFields(
			&userModels.Student{DepartureCompanionNote: strPtr("Geschwisterkind Mia")},
			&userModels.Student{DepartureCompanionNote: nil},
		)
		require.Len(t, edits, 1)
		e := edits[0]
		assert.Equal(t, auditModels.StudentFieldDepartureCompanionNote, e.FieldName)
		assert.Equal(t, "Geschwisterkind Mia", *e.OldValue)
		assert.Equal(t, "", *e.NewValue)
	})
}

// TestDiffStudentFields_MultipleFields verifies several simultaneous changes
// each record their own edit.
func TestDiffStudentFields_MultipleFields(t *testing.T) {
	edits := diffStudentFields(
		&userModels.Student{
			Status:          userModels.StudentStatusActive,
			SupervisorNotes: strPtr("alt"),
		},
		&userModels.Student{
			Status:          userModels.StudentStatusInactive,
			SupervisorNotes: strPtr("neu"),
		},
	)
	byField := editByField(edits)
	require.Len(t, edits, 2)
	assert.Contains(t, byField, auditModels.StudentFieldStatus)
	assert.Contains(t, byField, auditModels.StudentFieldSupervisorNotes)
	assert.Equal(t, "Inaktiv", *byField[auditModels.StudentFieldStatus].NewValue)
}
