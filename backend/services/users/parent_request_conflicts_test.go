package users

import (
	"testing"

	"github.com/stretchr/testify/assert"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

func TestParentRequestConflictKeys(t *testing.T) {
	t.Parallel()

	t.Run("two absences on one day collide whatever status they ask for", func(t *testing.T) {
		t.Parallel()
		sick := ParentRequestConflictKeys(ParentRequestConflictInput{
			RequestType: userModels.ParentRequestTypeExcusedAbsence, Dates: []string{"2026-09-01"},
		})
		excused := ParentRequestConflictKeys(ParentRequestConflictInput{
			RequestType: userModels.ParentRequestTypeExcusedAbsence, Dates: []string{"2026-09-01"},
		})
		assert.Equal(t, []string{"absence:2026-09-01"}, sick)
		assert.Equal(t, sick, excused)
	})

	t.Run("absences on different days do not collide", func(t *testing.T) {
		t.Parallel()
		first := ParentRequestConflictKeys(ParentRequestConflictInput{
			RequestType: userModels.ParentRequestTypeExcusedAbsence, Dates: []string{"2026-09-01"},
		})
		second := ParentRequestConflictKeys(ParentRequestConflictInput{
			RequestType: userModels.ParentRequestTypeExcusedAbsence, Dates: []string{"2026-09-02"},
		})
		assert.NotEqual(t, first, second)
	})

	t.Run("a weekly plan occupies one key per changed weekday", func(t *testing.T) {
		t.Parallel()
		keys := ParentRequestConflictKeys(ParentRequestConflictInput{
			RequestType: userModels.ParentRequestTypeCareSchedule, Weekdays: []int{3, 1, 1}, CareKind: "booking",
		})
		assert.Equal(t, []string{"care:1:booking", "care:3:booking"}, keys)
	})

	t.Run("different care aspects of one weekday do not collide", func(t *testing.T) {
		t.Parallel()
		booking := ParentRequestConflictKeys(ParentRequestConflictInput{
			RequestType: userModels.ParentRequestTypeCareSchedule, Weekdays: []int{2}, CareKind: "booking",
		})
		pickup := ParentRequestConflictKeys(ParentRequestConflictInput{
			RequestType: userModels.ParentRequestTypeCareSchedule, Weekdays: []int{2}, CareKind: "pickup",
		})
		assert.NotEqual(t, booking, pickup)
	})

	t.Run("master data collides per field, not per target", func(t *testing.T) {
		t.Parallel()
		name := ParentRequestConflictKeys(ParentRequestConflictInput{
			RequestType: userModels.ParentRequestTypeMasterData, Target: "person", Field: "last_name",
		})
		class := ParentRequestConflictKeys(ParentRequestConflictInput{
			RequestType: userModels.ParentRequestTypeMasterData, Target: "student", Field: "school_class",
		})
		assert.Equal(t, []string{"md:person:last_name"}, name)
		assert.NotEqual(t, name, class)
	})

	t.Run("offering validity is open ended, so one offering is one key", func(t *testing.T) {
		t.Parallel()
		early := ParentRequestConflictKeys(ParentRequestConflictInput{
			RequestType: userModels.ParentRequestTypeOffering, OfferingID: 42,
		})
		late := ParentRequestConflictKeys(ParentRequestConflictInput{
			RequestType: userModels.ParentRequestTypeOffering, OfferingID: 42,
		})
		assert.Equal(t, []string{"offer:42"}, early)
		assert.Equal(t, early, late)
	})

	t.Run("an unreadable request occupies no key", func(t *testing.T) {
		t.Parallel()
		assert.Empty(t, ParentRequestConflictKeys(ParentRequestConflictInput{RequestType: "unknown"}))
		assert.Empty(t, ParentRequestConflictKeys(ParentRequestConflictInput{
			RequestType: userModels.ParentRequestTypeMasterData, Target: "person",
		}))
		assert.Empty(t, ParentRequestConflictKeys(ParentRequestConflictInput{
			RequestType: userModels.ParentRequestTypeCareSchedule, Weekdays: []int{0, 9},
		}))
	})
}
