package active_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func statusRow(id int64, status string, reportedAt time.Time) *activeModels.StudentStatusDay {
	return &activeModels.StudentStatusDay{
		Model:      modelBase.Model{ID: id},
		StudentID:  90,
		Date:       timezone.NewDate(2026, 5, 25),
		Status:     status,
		ReportedAt: reportedAt,
		Source:     activeModels.StudentStatusSourcePlanned,
	}
}

func TestResolveEffectiveStatus_Precedence(t *testing.T) {
	base := time.Date(2026, 5, 25, 8, 0, 0, 0, time.UTC)

	t.Run("empty rows resolve to nothing", func(t *testing.T) {
		eff := activeService.ResolveEffectiveStatus(nil)
		assert.False(t, eff.Sick)
		assert.False(t, eff.ClassTrip)
		assert.False(t, eff.Excused)
	})

	t.Run("sick wins over class trip and excused", func(t *testing.T) {
		eff := activeService.ResolveEffectiveStatus([]*activeModels.StudentStatusDay{
			statusRow(1, activeModels.StudentStatusDayExcused, base),
			statusRow(2, activeModels.StudentStatusDayClassTrip, base.Add(time.Hour)),
			statusRow(3, activeModels.StudentStatusDaySick, base.Add(2*time.Hour)),
		})
		assert.True(t, eff.Sick)
		assert.False(t, eff.ClassTrip)
		assert.False(t, eff.Excused)
		require.NotNil(t, eff.SickSince)
		assert.Equal(t, base.Add(2*time.Hour), *eff.SickSince)
	})

	t.Run("class trip wins over excused", func(t *testing.T) {
		eff := activeService.ResolveEffectiveStatus([]*activeModels.StudentStatusDay{
			statusRow(1, activeModels.StudentStatusDayExcused, base),
			statusRow(2, activeModels.StudentStatusDayClassTrip, base),
		})
		assert.True(t, eff.ClassTrip)
		assert.False(t, eff.Excused)
		require.NotNil(t, eff.ClassTripSince)
	})

	t.Run("latest reported wins within a status", func(t *testing.T) {
		eff := activeService.ResolveEffectiveStatus([]*activeModels.StudentStatusDay{
			statusRow(1, activeModels.StudentStatusDayExcused, base),
			statusRow(2, activeModels.StudentStatusDayExcused, base.Add(time.Hour)),
		})
		assert.True(t, eff.Excused)
		require.NotNil(t, eff.ExcusedSince)
		assert.Equal(t, base.Add(time.Hour), *eff.ExcusedSince)
	})
}

func TestApplyAndClearLiveStatusForToday(t *testing.T) {
	now := time.Date(2026, 5, 25, 9, 30, 0, 0, time.UTC)
	student := &userModels.Student{}

	activeService.ApplyLiveStatusForToday(student, activeModels.StudentStatusDaySick, now)
	require.NotNil(t, student.Sick)
	require.NotNil(t, student.Excused)
	assert.True(t, *student.Sick)
	assert.False(t, *student.Excused)
	require.NotNil(t, student.SickSince)
	assert.Nil(t, student.ExcusedSince)

	activeService.ApplyLiveStatusForToday(student, activeModels.StudentStatusDayExcused, now.Add(time.Hour))
	assert.False(t, *student.Sick)
	assert.True(t, *student.Excused)
	assert.Nil(t, student.SickSince)
	require.NotNil(t, student.ExcusedSince)

	activeService.ApplyLiveStatusForToday(student, activeModels.StudentStatusDayClassTrip, now)
	assert.False(t, *student.Sick)
	assert.False(t, *student.Excused)

	activeService.ApplyLiveStatusForToday(student, activeModels.StudentStatusDaySick, now)
	activeService.ClearLiveStatusForToday(student, activeModels.StudentStatusDaySick)
	assert.False(t, *student.Sick)
	assert.Nil(t, student.SickSince)
}
