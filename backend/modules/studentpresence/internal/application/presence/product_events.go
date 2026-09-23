package presence

// productEventTracker is the capture operation this service needs.
// The composition root owns the tracker's lifetime.
type productEventTracker interface {
	Capture(distinctID, event string, props map[string]any)
}
