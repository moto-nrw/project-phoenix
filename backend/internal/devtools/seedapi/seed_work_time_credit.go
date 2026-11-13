package api

// withSeedPackUpTime closes the live demo block with five minutes pack-up
// time (#6852), so the day view shows the credit. The block lasts seconds, so
// a rounding would only cancel it again; the school keeps the defaults (both
// off) for every other block.
func withSeedPackUpTime(rt *Runtime, action func() error) error {
	return withTemporarySeedSetting(rt, rt.TenantAuth, "tracking.pack_up_minutes", 5, 0, action)
}
