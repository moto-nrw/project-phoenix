package timetable

import "testing"

func TestInstanceStatusPredicates(t *testing.T) {
	t.Parallel()
	if !IsPlannedInstanceStatus(InstanceStatusPlanned) || IsPlannedInstanceStatus(InstanceStatusCancelled) || IsPlannedInstanceStatus("") {
		t.Error("only the planned status plans the block")
	}
	if !(&ScheduledInstance{Status: InstanceStatusCancelled}).IsCancelled() || (&ScheduledInstance{Status: InstanceStatusPlanned}).IsCancelled() {
		t.Error("only the cancelled status cancels the block")
	}
}
