package compose

import (
	auditScans "github.com/moto-ogs/project-phoenix/modules/auditlog/internal/adapters/auditpostgres"
	auditService "github.com/moto-ogs/project-phoenix/modules/auditlog/internal/application"
)

type (
	AppendObservation          = auditService.AppendObservation
	Command                    = auditService.Command
	ConsentRecorder            = auditService.ConsentRecorder
	UnregisteredTagScanRuntime = auditScans.UnregisteredTagScanRuntime
	UnregisteredTagScanService = auditScans.UnregisteredTagScanService
)

var (
	NewCommand                    = auditService.NewCommand
	NewConsentRecorder            = auditService.NewConsentRecorder
	NewUnregisteredTagScanService = auditScans.NewUnregisteredTagScanService
)
