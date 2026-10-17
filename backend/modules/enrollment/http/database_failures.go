package enrollmenthttp

// DatabaseFailures supplies driver-free facts for HTTP retry and refusal paths.
// The composition root binds Transaction Runtime; the HTTP adapter owns this port.
type DatabaseFailures struct {
	Transient  func(error) bool
	Constraint func(error) bool
}
