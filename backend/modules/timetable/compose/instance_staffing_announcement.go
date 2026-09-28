package compose

// AnnounceStaffingChanged sends one tenant-wide staffing_deviation_changed
// event through the lifecycle's broadcaster; source names the emitting flow
// (#1844). The timetable HTTP adapter announces its staffing saves with it
// after their tenant transaction commits. Without a broadcaster it sends
// nothing.
func (s *InstanceLifecycleService) AnnounceStaffingChanged(tenantID int64, source string) error {
	if s.deps.Broadcaster == nil {
		return nil
	}
	return s.deps.Broadcaster.BroadcastToTenant(tenantID, LifecycleEvent{Type: LifecycleEventStaffingDeviationChanged, Source: &source})
}
