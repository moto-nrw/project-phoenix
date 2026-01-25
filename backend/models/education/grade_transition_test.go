package education

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGradeTransition_Validate(t *testing.T) {
	t.Run("valid transition", func(t *testing.T) {
		transition := &GradeTransition{
			AcademicYear: "2025-2026",
			Status:       TransitionStatusDraft,
			CreatedBy:    1,
		}
		err := transition.Validate()
		require.NoError(t, err)
	})

	t.Run("empty academic year", func(t *testing.T) {
		transition := &GradeTransition{
			AcademicYear: "",
			CreatedBy:    1,
		}
		err := transition.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "academic year is required")
	})

	t.Run("whitespace only academic year", func(t *testing.T) {
		transition := &GradeTransition{
			AcademicYear: "   ",
			CreatedBy:    1,
		}
		err := transition.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "academic year is required")
	})

	t.Run("invalid academic year format - no dash", func(t *testing.T) {
		transition := &GradeTransition{
			AcademicYear: "20252026",
			CreatedBy:    1,
		}
		err := transition.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "format YYYY-YYYY")
	})

	t.Run("invalid academic year format - wrong length", func(t *testing.T) {
		transition := &GradeTransition{
			AcademicYear: "25-26",
			CreatedBy:    1,
		}
		err := transition.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "format YYYY-YYYY")
	})

	t.Run("invalid status", func(t *testing.T) {
		transition := &GradeTransition{
			AcademicYear: "2025-2026",
			Status:       "invalid",
			CreatedBy:    1,
		}
		err := transition.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid status")
	})

	t.Run("empty status defaults to draft", func(t *testing.T) {
		transition := &GradeTransition{
			AcademicYear: "2025-2026",
			Status:       "",
			CreatedBy:    1,
		}
		err := transition.Validate()
		require.NoError(t, err)
		assert.Equal(t, TransitionStatusDraft, transition.Status)
	})

	t.Run("missing created_by", func(t *testing.T) {
		transition := &GradeTransition{
			AcademicYear: "2025-2026",
			CreatedBy:    0,
		}
		err := transition.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "created_by is required")
	})

	t.Run("negative created_by", func(t *testing.T) {
		transition := &GradeTransition{
			AcademicYear: "2025-2026",
			CreatedBy:    -1,
		}
		err := transition.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "created_by is required")
	})

	t.Run("valid applied status", func(t *testing.T) {
		transition := &GradeTransition{
			AcademicYear: "2025-2026",
			Status:       TransitionStatusApplied,
			CreatedBy:    1,
		}
		err := transition.Validate()
		require.NoError(t, err)
	})

	t.Run("valid reverted status", func(t *testing.T) {
		transition := &GradeTransition{
			AcademicYear: "2025-2026",
			Status:       TransitionStatusReverted,
			CreatedBy:    1,
		}
		err := transition.Validate()
		require.NoError(t, err)
	})
}

func TestGradeTransition_StatusHelpers(t *testing.T) {
	t.Run("IsDraft", func(t *testing.T) {
		transition := &GradeTransition{Status: TransitionStatusDraft}
		assert.True(t, transition.IsDraft())
		assert.False(t, transition.IsApplied())
		assert.False(t, transition.IsReverted())
	})

	t.Run("IsApplied", func(t *testing.T) {
		transition := &GradeTransition{Status: TransitionStatusApplied}
		assert.False(t, transition.IsDraft())
		assert.True(t, transition.IsApplied())
		assert.False(t, transition.IsReverted())
	})

	t.Run("IsReverted", func(t *testing.T) {
		transition := &GradeTransition{Status: TransitionStatusReverted}
		assert.False(t, transition.IsDraft())
		assert.False(t, transition.IsApplied())
		assert.True(t, transition.IsReverted())
	})
}

func TestGradeTransition_CanModify(t *testing.T) {
	t.Run("can modify draft", func(t *testing.T) {
		transition := &GradeTransition{Status: TransitionStatusDraft}
		assert.True(t, transition.CanModify())
	})

	t.Run("cannot modify applied", func(t *testing.T) {
		transition := &GradeTransition{Status: TransitionStatusApplied}
		assert.False(t, transition.CanModify())
	})

	t.Run("cannot modify reverted", func(t *testing.T) {
		transition := &GradeTransition{Status: TransitionStatusReverted}
		assert.False(t, transition.CanModify())
	})
}

func TestGradeTransition_CanApply(t *testing.T) {
	t.Run("can apply draft with mappings", func(t *testing.T) {
		transition := &GradeTransition{
			Status: TransitionStatusDraft,
			Mappings: []*GradeTransitionMapping{
				{FromClass: "1a", ToClass: strPtr("2a")},
			},
		}
		assert.True(t, transition.CanApply())
	})

	t.Run("cannot apply draft without mappings", func(t *testing.T) {
		transition := &GradeTransition{Status: TransitionStatusDraft}
		assert.False(t, transition.CanApply())
	})

	t.Run("cannot apply applied", func(t *testing.T) {
		transition := &GradeTransition{
			Status: TransitionStatusApplied,
			Mappings: []*GradeTransitionMapping{
				{FromClass: "1a", ToClass: strPtr("2a")},
			},
		}
		assert.False(t, transition.CanApply())
	})

	t.Run("cannot apply reverted", func(t *testing.T) {
		transition := &GradeTransition{
			Status: TransitionStatusReverted,
			Mappings: []*GradeTransitionMapping{
				{FromClass: "1a", ToClass: strPtr("2a")},
			},
		}
		assert.False(t, transition.CanApply())
	})
}

func TestGradeTransition_CanRevert(t *testing.T) {
	t.Run("can revert applied", func(t *testing.T) {
		transition := &GradeTransition{Status: TransitionStatusApplied}
		assert.True(t, transition.CanRevert())
	})

	t.Run("cannot revert draft", func(t *testing.T) {
		transition := &GradeTransition{Status: TransitionStatusDraft}
		assert.False(t, transition.CanRevert())
	})

	t.Run("cannot revert reverted", func(t *testing.T) {
		transition := &GradeTransition{Status: TransitionStatusReverted}
		assert.False(t, transition.CanRevert())
	})
}

func TestGradeTransitionMapping_Validate(t *testing.T) {
	t.Run("valid mapping with to_class", func(t *testing.T) {
		mapping := &GradeTransitionMapping{
			TransitionID: 1,
			FromClass:    "1a",
			ToClass:      strPtr("2a"),
		}
		err := mapping.Validate()
		require.NoError(t, err)
	})

	t.Run("valid mapping without to_class (graduate)", func(t *testing.T) {
		mapping := &GradeTransitionMapping{
			TransitionID: 1,
			FromClass:    "4a",
			ToClass:      nil,
		}
		err := mapping.Validate()
		require.NoError(t, err)
	})

	t.Run("empty from_class", func(t *testing.T) {
		mapping := &GradeTransitionMapping{
			TransitionID: 1,
			FromClass:    "",
			ToClass:      strPtr("2a"),
		}
		err := mapping.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "from_class is required")
	})

	t.Run("whitespace only from_class", func(t *testing.T) {
		mapping := &GradeTransitionMapping{
			TransitionID: 1,
			FromClass:    "   ",
			ToClass:      strPtr("2a"),
		}
		err := mapping.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "from_class is required")
	})

	t.Run("missing transition_id", func(t *testing.T) {
		mapping := &GradeTransitionMapping{
			TransitionID: 0,
			FromClass:    "1a",
			ToClass:      strPtr("2a"),
		}
		err := mapping.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "transition_id is required")
	})

	t.Run("from equals to", func(t *testing.T) {
		mapping := &GradeTransitionMapping{
			TransitionID: 1,
			FromClass:    "1a",
			ToClass:      strPtr("1a"),
		}
		err := mapping.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be the same")
	})
}

func TestGradeTransitionMapping_GetAction(t *testing.T) {
	t.Run("promote action", func(t *testing.T) {
		mapping := &GradeTransitionMapping{
			FromClass: "1a",
			ToClass:   strPtr("2a"),
		}
		assert.Equal(t, ActionPromoted, mapping.GetAction())
	})

	t.Run("graduate action", func(t *testing.T) {
		mapping := &GradeTransitionMapping{
			FromClass: "4a",
			ToClass:   nil,
		}
		assert.Equal(t, ActionGraduated, mapping.GetAction())
	})
}

func TestGradeTransitionMapping_IsGraduating(t *testing.T) {
	t.Run("is graduating when to_class is nil", func(t *testing.T) {
		mapping := &GradeTransitionMapping{ToClass: nil}
		assert.True(t, mapping.IsGraduating())
	})

	t.Run("not graduating when to_class is set", func(t *testing.T) {
		mapping := &GradeTransitionMapping{ToClass: strPtr("2a")}
		assert.False(t, mapping.IsGraduating())
	})
}

func TestGradeTransitionHistory_Validate(t *testing.T) {
	t.Run("valid history promoted", func(t *testing.T) {
		history := &GradeTransitionHistory{
			TransitionID: 1,
			StudentID:    1,
			PersonName:   "Max Mustermann",
			FromClass:    "1a",
			ToClass:      strPtr("2a"),
			Action:       ActionPromoted,
		}
		err := history.Validate()
		require.NoError(t, err)
	})

	t.Run("valid history graduated", func(t *testing.T) {
		history := &GradeTransitionHistory{
			TransitionID: 1,
			StudentID:    1,
			PersonName:   "Max Mustermann",
			FromClass:    "4a",
			ToClass:      nil,
			Action:       ActionGraduated,
		}
		err := history.Validate()
		require.NoError(t, err)
	})

	t.Run("missing transition_id", func(t *testing.T) {
		history := &GradeTransitionHistory{
			TransitionID: 0,
			StudentID:    1,
			PersonName:   "Max Mustermann",
			FromClass:    "1a",
			Action:       ActionPromoted,
		}
		err := history.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "transition_id is required")
	})

	t.Run("missing student_id", func(t *testing.T) {
		history := &GradeTransitionHistory{
			TransitionID: 1,
			StudentID:    0,
			PersonName:   "Max Mustermann",
			FromClass:    "1a",
			Action:       ActionPromoted,
		}
		err := history.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "student_id is required")
	})

	t.Run("missing person_name", func(t *testing.T) {
		history := &GradeTransitionHistory{
			TransitionID: 1,
			StudentID:    1,
			PersonName:   "",
			FromClass:    "1a",
			Action:       ActionPromoted,
		}
		err := history.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "person_name is required")
	})

	t.Run("missing from_class", func(t *testing.T) {
		history := &GradeTransitionHistory{
			TransitionID: 1,
			StudentID:    1,
			PersonName:   "Max Mustermann",
			FromClass:    "",
			Action:       ActionPromoted,
		}
		err := history.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "from_class is required")
	})

	t.Run("invalid action", func(t *testing.T) {
		history := &GradeTransitionHistory{
			TransitionID: 1,
			StudentID:    1,
			PersonName:   "Max Mustermann",
			FromClass:    "1a",
			Action:       "invalid",
		}
		err := history.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid action")
	})
}

// ============================================================================
// Interface Method Tests - GradeTransition
// ============================================================================

func TestGradeTransition_GetID(t *testing.T) {
	transition := &GradeTransition{}
	transition.ID = 123

	id := transition.GetID()
	assert.Equal(t, int64(123), id)
}

func TestGradeTransition_GetCreatedAt(t *testing.T) {
	now := time.Now()
	transition := &GradeTransition{}
	transition.CreatedAt = now

	createdAt := transition.GetCreatedAt()
	assert.Equal(t, now, createdAt)
}

func TestGradeTransition_GetUpdatedAt(t *testing.T) {
	now := time.Now()
	transition := &GradeTransition{}
	transition.UpdatedAt = now

	updatedAt := transition.GetUpdatedAt()
	assert.Equal(t, now, updatedAt)
}

func TestGradeTransition_TableName(t *testing.T) {
	transition := &GradeTransition{}
	assert.Equal(t, "education.grade_transitions", transition.TableName())
}

// ============================================================================
// Interface Method Tests - GradeTransitionMapping
// ============================================================================

func TestGradeTransitionMapping_GetID(t *testing.T) {
	mapping := &GradeTransitionMapping{}
	mapping.ID = 456

	id := mapping.GetID()
	assert.Equal(t, int64(456), id)
}

func TestGradeTransitionMapping_GetCreatedAt(t *testing.T) {
	now := time.Now()
	mapping := &GradeTransitionMapping{}
	mapping.CreatedAt = now

	createdAt := mapping.GetCreatedAt()
	assert.Equal(t, now, createdAt)
}

func TestGradeTransitionMapping_GetUpdatedAt(t *testing.T) {
	now := time.Now()
	mapping := &GradeTransitionMapping{}
	mapping.UpdatedAt = now

	updatedAt := mapping.GetUpdatedAt()
	assert.Equal(t, now, updatedAt)
}

func TestGradeTransitionMapping_TableName(t *testing.T) {
	mapping := &GradeTransitionMapping{}
	assert.Equal(t, "education.grade_transition_mappings", mapping.TableName())
}

// ============================================================================
// Interface Method Tests - GradeTransitionHistory
// ============================================================================

func TestGradeTransitionHistory_GetID(t *testing.T) {
	history := &GradeTransitionHistory{}
	history.ID = 789

	id := history.GetID()
	assert.Equal(t, int64(789), id)
}

func TestGradeTransitionHistory_GetCreatedAt(t *testing.T) {
	now := time.Now()
	history := &GradeTransitionHistory{}
	history.CreatedAt = now

	createdAt := history.GetCreatedAt()
	assert.Equal(t, now, createdAt)
}

func TestGradeTransitionHistory_GetUpdatedAt(t *testing.T) {
	now := time.Now()
	history := &GradeTransitionHistory{}
	history.UpdatedAt = now

	updatedAt := history.GetUpdatedAt()
	assert.Equal(t, now, updatedAt)
}

func TestGradeTransitionHistory_TableName(t *testing.T) {
	history := &GradeTransitionHistory{}
	assert.Equal(t, "education.grade_transition_history", history.TableName())
}

func TestGradeTransitionHistory_WasGraduated(t *testing.T) {
	t.Run("returns true for graduated action", func(t *testing.T) {
		history := &GradeTransitionHistory{Action: ActionGraduated}
		assert.True(t, history.WasGraduated())
	})

	t.Run("returns false for promoted action", func(t *testing.T) {
		history := &GradeTransitionHistory{Action: ActionPromoted}
		assert.False(t, history.WasGraduated())
	})

	t.Run("returns false for unchanged action", func(t *testing.T) {
		history := &GradeTransitionHistory{Action: ActionUnchanged}
		assert.False(t, history.WasGraduated())
	})
}

func TestGradeTransitionHistory_WasPromoted(t *testing.T) {
	t.Run("returns true for promoted action", func(t *testing.T) {
		history := &GradeTransitionHistory{Action: ActionPromoted}
		assert.True(t, history.WasPromoted())
	})

	t.Run("returns false for graduated action", func(t *testing.T) {
		history := &GradeTransitionHistory{Action: ActionGraduated}
		assert.False(t, history.WasPromoted())
	})

	t.Run("returns false for unchanged action", func(t *testing.T) {
		history := &GradeTransitionHistory{Action: ActionUnchanged}
		assert.False(t, history.WasPromoted())
	})
}

// ============================================================================
// Additional Edge Case Tests
// ============================================================================

func TestGradeTransitionMapping_Validate_TrimsWhitespace(t *testing.T) {
	t.Run("trims whitespace from to_class", func(t *testing.T) {
		toClass := "  2a  "
		mapping := &GradeTransitionMapping{
			TransitionID: 1,
			FromClass:    "1a",
			ToClass:      &toClass,
		}
		err := mapping.Validate()
		require.NoError(t, err)
		assert.Equal(t, "2a", *mapping.ToClass)
	})

	t.Run("empty string to_class becomes nil", func(t *testing.T) {
		toClass := "   "
		mapping := &GradeTransitionMapping{
			TransitionID: 1,
			FromClass:    "1a",
			ToClass:      &toClass,
		}
		err := mapping.Validate()
		require.NoError(t, err)
		assert.Nil(t, mapping.ToClass)
	})
}

func TestGradeTransitionHistory_Validate_TrimsWhitespace(t *testing.T) {
	t.Run("trims whitespace from to_class", func(t *testing.T) {
		toClass := "  2a  "
		history := &GradeTransitionHistory{
			TransitionID: 1,
			StudentID:    1,
			PersonName:   "Test Student",
			FromClass:    "1a",
			ToClass:      &toClass,
			Action:       ActionPromoted,
		}
		err := history.Validate()
		require.NoError(t, err)
		assert.Equal(t, "2a", *history.ToClass)
	})

	t.Run("empty string to_class becomes nil", func(t *testing.T) {
		toClass := "   "
		history := &GradeTransitionHistory{
			TransitionID: 1,
			StudentID:    1,
			PersonName:   "Test Student",
			FromClass:    "1a",
			ToClass:      &toClass,
			Action:       ActionPromoted,
		}
		err := history.Validate()
		require.NoError(t, err)
		assert.Nil(t, history.ToClass)
	})
}

func TestGradeTransitionHistory_Validate_ValidUnchangedAction(t *testing.T) {
	history := &GradeTransitionHistory{
		TransitionID: 1,
		StudentID:    1,
		PersonName:   "Test Student",
		FromClass:    "1a",
		ToClass:      strPtr("1a"),
		Action:       ActionUnchanged,
	}
	err := history.Validate()
	require.NoError(t, err)
}

// Helper function
func strPtr(s string) *string {
	return &s
}
