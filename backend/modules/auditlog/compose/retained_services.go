package compose

import auditService "github.com/moto-nrw/project-phoenix/services/audit"

// The retained Audit Platform services the legacy composition binds (#2747):
// the fail-closed append command, the consent recorder over it and the
// unregistered-tag scan log, named through this composition instead of the
// application package.
type (
	AppendObservation          = auditService.AppendObservation
	Command                    = auditService.Command
	ConsentRecorder            = auditService.ConsentRecorder
	UnregisteredTagScanRuntime = auditService.UnregisteredTagScanRuntime
	UnregisteredTagScanService = auditService.UnregisteredTagScanService
)

var (
	NewCommand                    = auditService.NewCommand
	NewConsentRecorder            = auditService.NewConsentRecorder
	NewUnregisteredTagScanService = auditService.NewUnregisteredTagScanService
)
