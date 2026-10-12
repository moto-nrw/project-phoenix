package compose

import (
	"testing"

	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/stretchr/testify/assert"
)

// The scheduled-instance view carries the duty flag of the linked template
// (#3822) that the plan export prints without a head count.
func TestScheduledInstanceOfMarksDuties(t *testing.T) {
	t.Parallel()

	for templateType, want := range map[string]bool{
		timetable.GroupTypeDuty:     true,
		"":                          false,
		timetable.GroupTypeActivity: false,
	} {
		row := &scheduleModels.ActivityInstance{TemplateType: templateType}
		assert.Equal(t, want, ScheduledInstanceOf(row).IsDuty, "template type %q", templateType)
	}
}
