package services

import (
	"errors"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// RetentionTestOwners are the retained owners the Worker's nightly retention
// steps bind (#3950), composed over the test database for the root's
// retention suite as the service graph composes them: the deletion log of
// the Audit command, the Settings Platform, the unregistered tag scans over
// the Device Fleet owner and the Care Plan status days.
type RetentionTestOwners struct {
	Audit                *auditCompose.Command
	Settings             settingsCompose.BatchSettingsService
	UnregisteredTagScans auditCompose.UnregisteredTagScanService
	StatusDays           careplan.StudentStatusDayRetention
}

// NewRetentionTestOwners composes the retention owners over db; unit is the
// tenant runtime the settings resolve in.
func NewRetentionTestOwners(db *bun.DB, unit tenant.UnitOfWork) (RetentionTestOwners, error) {
	command, err := auditCompose.NewCommand(NewTestAuditStore(db), func(auditCompose.AppendObservation) {})
	if err != nil {
		return RetentionTestOwners{}, err
	}
	settings, err := NewSettingsTestModule(db, unit)
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
	carePlan, err := NewUnobservedCarePlan(db, NewUnobservedTimetableDependencies(db))
	if err != nil {
		return RetentionTestOwners{}, err
	}
	statusDays, ok := carePlan.(careplan.StudentStatusDayRetention)
	if !ok {
		return RetentionTestOwners{}, errors.New("care plan does not delete expired status days")
	}
	return RetentionTestOwners{
		Audit: command, Settings: settings.Settings, UnregisteredTagScans: scans, StatusDays: statusDays,
	}, nil
}
