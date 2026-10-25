//go:build testhooks

package owners

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"time"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	deliveryPolicy "github.com/moto-nrw/project-phoenix/modules/delivery/devices"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	exportTransferCompose "github.com/moto-nrw/project-phoenix/modules/exporttransfer/compose"
	filestorageCompose "github.com/moto-nrw/project-phoenix/modules/filestorage/compose"
	schedulerPorts "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"
	settingsModule "github.com/moto-nrw/project-phoenix/modules/settings"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/moto-nrw/project-phoenix/workflows/studentdeletion"
	studentdeletioncompose "github.com/moto-nrw/project-phoenix/workflows/studentdeletion/compose"
)

// NewRetentionTestOwners composes the retained owners the Worker's nightly
// retention steps bind (#3950) over db as the service graph composes them,
// and returns the deletion log of the Audit command, the Settings Platform
// and the owners' retention steps in the order the Worker runs them: the
// unregistered tag scans over the Device Fleet owner, the Care Plan status
// days, the parent messages, the settled enrollments, the staff sick notes,
// the delivery data, the setting protocol and export journal of #4044 and
// the technical remains of #4046. unit is the tenant runtime the settings
// resolve in.
func NewRetentionTestOwners(db transactionruntime.Database, unit tenant.UnitOfWork) (*auditCompose.Command, settingsCompose.BatchSettingsService, []schedulerPorts.RetentionStep, error) {
	command, err := auditCompose.NewCommand(NewTestAuditStore(db), func(auditCompose.AppendObservation) {})
	if err != nil {
		return nil, nil, nil, err
	}
	settings, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return nil, nil, nil, err
	}
	fleet, err := NewDeviceFleet(db)
	if err != nil {
		return nil, nil, nil, err
	}
	carePlan, err := NewUnobservedCarePlan(db, MustNewUnobservedTimetable(db), MustNewPeopleDirectory(db))
	if err != nil {
		return nil, nil, nil, err
	}
	statusDays, ok := carePlan.(careplan.StudentStatusDayRetention)
	if !ok {
		return nil, nil, nil, errors.New("care plan does not delete expired status days")
	}
	enrollments, err := newSettledEnrollmentTestRetention(db, settings.Settings)
	if err != nil {
		return nil, nil, nil, err
	}
	steps := dataRetentionSteps(
		settings.Settings, fleet, statusDays,
		communicationCompose.NewParentMessageRetention(db.Pool(), settingsModule.NewParentMessageRetentionValues(settings.Settings)),
		enrollments, workforceCompose.NewSickNoteRetention(StaffSickNoteRetentionSettings{Settings: settings.Settings}),
		command, deliveryCompose.NewRetention(),
	)
	steps = append(steps, protocolRetentionSteps(command, settingsCompose.NewAuditRetention(), exportTransferCompose.NewJournalRetention())...)
	steps = append(steps, cleanupJobRetentionSteps(
		filestorageCompose.NewCleanupRetention(), workforceCompose.NewCleanupRetention(),
		carePlanCompose.NewCleanupRetention(), enrollmentCompose.New(),
	)...)
	announcements, err := newRetentionTestAnnouncements(db, settings.Settings)
	if err != nil {
		return nil, nil, nil, err
	}
	contentSteps, err := newSchoolContentRetentionSteps(db, settings.Settings, announcements, MustNewUnobservedTimetable(db))
	if err != nil {
		return nil, nil, nil, err
	}
	steps = append(steps, contentSteps...)
	return command, settings.Settings, steps, nil
}

// newRetentionTestAnnouncements composes the parent announcements the way
// the roots do, over File Storage's attachment purger, and returns their
// retention (#4045). No mail or push is wired: the announcements the tests
// delete have nothing pending, and the purger only queues cleanup intents,
// so the object store is never reached.
func newRetentionTestAnnouncements(db transactionruntime.Database, settings settingsCompose.BatchSettingsService) (communication.ParentAnnouncementRetention, error) {
	purger, err := filestorageCompose.NewAttachmentPurger(filestorageCompose.AttachmentPurgerDependencies{
		DB: db.Pool(), Objects: unreachableObjects{}, Observe: func(filestorageCompose.Observation) {},
	})
	if err != nil {
		return nil, err
	}
	announcements, err := communicationCompose.NewParentAnnouncementsWithAttachments(communicationCompose.ParentAnnouncementConfig{
		Repo:     parentStore.NewParentAnnouncementRepository(db.Pool(), NewParentAnnouncementAudience(db, ParentAnnouncementApplicants(enrollmentCompose.New()))),
		Settings: settingsModule.NewParentAnnouncementValues(settings), Logger: slog.Default(), Fingerprints: SecurityFingerprints{},
	}, purger, CommunicationRequestRuntime{}, deliveryPolicy.ParentDeliveryPolicy{})
	if err != nil {
		return nil, err
	}
	retention, ok := announcements.(communication.ParentAnnouncementRetention)
	if !ok {
		return nil, errors.New("parent announcements lack their retention")
	}
	return retention, nil
}

// unreachableObjects is the object store of a composition that never moves
// bytes.
type unreachableObjects struct{}

func (unreachableObjects) SavePrivate(context.Context, string, int64, string, io.Reader) (int64, error) {
	return 0, errors.New("retention test: no object store")
}

func (unreachableObjects) OpenPrivate(context.Context, string, int64, string) (interface {
	io.ReadSeekCloser
	ModTime() time.Time
}, error) {
	return nil, errors.New("retention test: no object store")
}

func (unreachableObjects) RemovePrivate(context.Context, string, int64, string) error {
	return errors.New("retention test: no object store")
}

// newSettledEnrollmentTestRetention composes the cleanup of settled
// enrollments as the service graph does: the Enrollment deletion module over
// the owner, the People Directory guardians, the audited adjustments and
// deletions, Care Plan's bookings and the Delivery outbox.
func newSettledEnrollmentTestRetention(db transactionruntime.Database, settings settingsCompose.BatchSettingsService) (enrollment.SettledEnrollmentRetention, error) {
	repos, err := NewEnrollmentFlowTestRepositories(db)
	if err != nil {
		return nil, err
	}
	reposAudit := NewEnrollmentFlowAuditRows(db)
	reposCare := NewEnrollmentFlowCarePlanRows(db)
	people, err := NewPeopleDirectory(db)
	if err != nil {
		return nil, err
	}
	deliveries, err := NewEnrollmentFlowDelivery(db)
	if err != nil {
		return nil, err
	}
	return NewEnrollmentDeletionModule(EnrollmentDeletionSources{
		Owner: repos.Enrollment(), Guardians: enrollmentGuardianDirectory{guardians: people},
		CountAuditAdjustments: reposAudit.EnrollmentOfferingAdjustment.CountForDeletion,
		CountBookings:         reposCare.CarePlan.CountCareOfferingBookings,
		Audit:                 reposAudit.EnrollmentDeletionAudit,
		Delivery:              enrollmentDeliveryAdapter{module: deliveries},
		Settings:              settings,
	}).Cleanup, nil
}

// NewStudentTestRetention composes the automatic deletion of children
// (#3962) over the production student deletion composition and the real
// owners, logging into audit. The Feedback owner is not part of this graph;
// its count stays zero.
func NewStudentTestRetention(db transactionruntime.Database, audit *auditCompose.Command, settings settingsCompose.BatchSettingsService) (*studentdeletion.Retention, error) {
	timetableOwner := MustNewUnobservedTimetable(db)
	carePlan, err := NewUnobservedCarePlan(db, timetableOwner, MustNewPeopleDirectory(db))
	if err != nil {
		return nil, err
	}
	people, err := NewPeopleDirectory(db)
	if err != nil {
		return nil, err
	}
	owners, err := NewStudentDeletionOwners(db)
	if err != nil {
		return nil, err
	}
	workflow, err := NewStudentDeletion(studentdeletioncompose.Dependencies{
		DB: db.Pool(), Owners: owners, Directory: people, CarePlan: carePlan, Timetable: timetableOwner, Feedback: noFeedback{},
		IsVerifiedStaff:       func(context.Context) (bool, error) { return false, nil },
		LockCareBookingWrites: MustNewTimetableRecurrenceLock(db).LockRecurrenceWrites,
		Audit:                 audit,
	}, NewStudentActivityEnrollments())
	if err != nil {
		return nil, err
	}
	return newStudentRetention(workflow, settings, nil)
}

type noFeedback struct{}

func (noFeedback) CountForStudent(context.Context, int64) (int, error) { return 0, nil }
