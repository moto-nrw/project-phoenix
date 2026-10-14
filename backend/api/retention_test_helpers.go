package api

import (
	"errors"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// RetentionTestOwners are the retained owners the Worker's nightly retention
// steps bind (#3950), composed over the test database for the root's
// retention suite as the service graph composes them: the deletion log of
// the Audit command, the Settings Platform, the unregistered tag scans over
// the Device Fleet owner, the Care Plan status days and the parent messages.
type RetentionTestOwners struct {
	Audit                *auditCompose.Command
	Settings             settingsCompose.BatchSettingsService
	UnregisteredTagScans auditCompose.UnregisteredTagScanService
	StatusDays           careplan.StudentStatusDayRetention
	ParentMessages       communication.ParentMessageCleanup
}

// NewRetentionTestOwners composes the retention owners over pool; unit is
// the tenant runtime the settings resolve in.
func NewRetentionTestOwners(pool *bun.DB, unit tenant.UnitOfWork) (RetentionTestOwners, error) {
	db, err := transactionruntime.NewDatabase(pool)
	if err != nil {
		return RetentionTestOwners{}, err
	}
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
	scans, err := services.NewUnregisteredTagScans(fleet)
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
		ParentMessages: communicationCompose.NewParentMessageRetention(pool, settings.Settings),
	}, nil
}
