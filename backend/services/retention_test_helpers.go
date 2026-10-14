package services

import (
	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// RetentionTestOwners are the retained owners the Worker's nightly retention
// steps bind (#3950), composed over the test database for the root's
// retention suite: the deletion log of the Audit command and the
// unregistered tag scans over the Device Fleet owner, as the service graph
// composes them.
type RetentionTestOwners struct {
	Audit                *auditCompose.Command
	UnregisteredTagScans auditCompose.UnregisteredTagScanService
}

// NewRetentionTestOwners composes the retention owners over db.
func NewRetentionTestOwners(db *bun.DB) (RetentionTestOwners, error) {
	command, err := auditCompose.NewCommand(NewTestAuditStore(db), func(auditCompose.AppendObservation) {})
	if err != nil {
		return RetentionTestOwners{}, err
	}
	fleet, err := NewDeviceFleet(db)
	if err != nil {
		return RetentionTestOwners{}, err
	}
	scans, err := auditCompose.NewUnregisteredTagScanService(
		newUnregisteredTagScanRepository(fleet),
		auditCompose.UnregisteredTagScanRuntime{TenantID: tenant.FromContext},
	)
	if err != nil {
		return RetentionTestOwners{}, err
	}
	return RetentionTestOwners{Audit: command, UnregisteredTagScans: scans}, nil
}
