package active

// ConfigureForTest applies construction-time options to a service the
// behaviour tests already built through their shared fixtures. Production
// composition passes the same options to NewService; there is no setter.
func ConfigureForTest(svc Service, options ...ServiceOption) {
	target := svc.(*service)
	for _, option := range options {
		option(target)
	}
}
