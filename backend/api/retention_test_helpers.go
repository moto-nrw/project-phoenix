package api

import (
	"errors"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	"github.com/moto-nrw/project-phoenix/modules/delivery"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/filestorage"
	filestorageCompose "github.com/moto-nrw/project-phoenix/modules/filestorage/compose"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// RetentionTestOwners are the retained owners the Worker's nightly retention
// steps bind (#3950), composed over the test database for the root's
// retention suite as the service graph composes them: the deletion log of
// the Audit command, the Settings Platform, the unregistered tag scans over
// the Device Fleet owner, the Care Plan status days, the parent messages, the
// staff sick notes, the delivery data and the technical remains of #4046.
type RetentionTestOwners struct {
	Audit                *auditCompose.Command
	Settings             settingsCompose.BatchSettingsService
	UnregisteredTagScans devicefleet.UnregisteredTagScanRetention
	StatusDays           careplan.StudentStatusDayRetention
	ParentMessages       communication.ParentMessageCleanup
	StaffSickNotes       workforce.SickNoteRetention
	Deliveries           delivery.Retention
	FileCleanups         filestorage.CleanupRetention
	StaffCleanups        workforce.CleanupRetention
	CarePlanCleanups     careplan.CleanupRetention
	SubmissionLimits     enrollment.SubmissionRateLimitRetention
}

// NewRetentionTestOwners composes the retention owners over db; unit is the
// tenant runtime the settings resolve in.
func NewRetentionTestOwners(db transactionruntime.Database, unit tenant.UnitOfWork) (RetentionTestOwners, error) {
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
	carePlan, err := NewUnobservedCarePlan(db, NewUnobservedTimetableDependencies(db))
	if err != nil {
		return RetentionTestOwners{}, err
	}
	statusDays, ok := carePlan.(careplan.StudentStatusDayRetention)
	if !ok {
		return RetentionTestOwners{}, errors.New("care plan does not delete expired status days")
	}
	return RetentionTestOwners{
		Audit: command, Settings: settings.Settings, UnregisteredTagScans: fleet, StatusDays: statusDays,
		ParentMessages:   communicationCompose.NewParentMessageRetention(db.Pool(), settings.Settings),
		StaffSickNotes:   workforceCompose.NewSickNoteRetention(settings.Settings),
		Deliveries:       deliveryCompose.NewRetention(),
		FileCleanups:     filestorageCompose.NewCleanupRetention(),
		StaffCleanups:    workforceCompose.NewCleanupRetention(),
		CarePlanCleanups: carePlanCompose.NewCleanupRetention(),
		SubmissionLimits: enrollmentCompose.New(),
	}, nil
}
