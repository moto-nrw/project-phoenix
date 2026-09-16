package active

import "github.com/moto-nrw/project-phoenix/tenant"

// The retained presence service takes its optional collaborators at
// construction (#3214). Every option below replaces a former post-construction
// setter one to one: an option left out keeps the behaviour the service had
// without that collaborator, which is the shape the bare unit fixtures rely on.

// ServiceOption configures optional collaborators of the presence service.
type ServiceOption func(*service)

// WithSettings supplies the tenant-scoped settings resolver. Without it the
// presence mode and the sick/excused clear modes cannot be resolved and the
// affected commands fail loudly.
func WithSettings(resolver SettingsResolver) ServiceOption {
	return func(s *service) { s.settings = resolver }
}

// WithTenantRuntime supplies the transaction runtime bound to this service's
// repository pool. Commands that run without a request transaction, such as
// the cleanup jobs, open their own tenant transaction through it. A zero
// runtime leaves the service on the runtime carried by the request context,
// as the former setter did when the composition root never called it.
func WithTenantRuntime(runtime tenant.UnitOfWork) ServiceOption {
	return func(s *service) {
		if runtime.IsZero() {
			return
		}
		s.tenantRuntime = &runtime
	}
}

// WithGuardianWaker binds the parent-portal wake-up after attendance changes
// (#2252). Without it no guardian is woken.
func WithGuardianWaker(waker GuardianWaker) ServiceOption {
	return func(s *service) { s.guardianWaker = waker }
}
