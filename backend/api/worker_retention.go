package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	carePlanModule "github.com/moto-nrw/project-phoenix/modules/careplan"
	communicationModule "github.com/moto-nrw/project-phoenix/modules/communication"
	deliveryModule "github.com/moto-nrw/project-phoenix/modules/delivery"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	enrollmentModule "github.com/moto-nrw/project-phoenix/modules/enrollment"
	exportTransferModule "github.com/moto-nrw/project-phoenix/modules/exporttransfer"
	fileStorageModule "github.com/moto-nrw/project-phoenix/modules/filestorage"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	schedulerRuntimeCompose "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/compose"
	schedulerPorts "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"
	schoolMembershipCompose "github.com/moto-nrw/project-phoenix/modules/schoolmembership/compose"
	settingsModule "github.com/moto-nrw/project-phoenix/modules/settings"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	studentDirectoryView "github.com/moto-nrw/project-phoenix/modules/studentdirectoryview/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/moto-nrw/project-phoenix/workflows/staffoffboarding"
	"github.com/moto-nrw/project-phoenix/workflows/studentdeletion"
)

// workerRetention is the retention capability of the nightly cleanup (#3950):
// the owners' deletion steps and their deletion log, which the Worker runs,
// and the ledger of each school's last successful run.
type workerRetention struct {
	steps []schedulerPorts.RetentionStep
	log   schedulerPorts.RetentionLog
	runs  schedulerPorts.CleanupRunLedger
}

// retentionOwners are the owner capabilities whose data the nightly cleanup
// deletes, and the deletion log. Each owner resolves its own window.
type retentionOwners struct {
	deletionLog interface {
		LogRetention(ctx context.Context, kind string, deleted, retentionDays int) error
	}
	// settings resolves the retention windows of the school in context.
	settings interface {
		ResolveInt(ctx context.Context, key string) (int, error)
	}
	unregisteredTagScans devicefleet.UnregisteredTagScanRetention
	statusDays           carePlanModule.StudentStatusDayRetention
	parentMessages       communicationModule.ParentMessageCleanup
	// settledEnrollments deletes the withdrawn and lapsed enrollments no
	// child was created from (#3960).
	settledEnrollments enrollmentModule.SettledEnrollmentRetention
	// staffSickNotes deletes the sick notes (AU-Bescheinigungen) after the
	// school's window and queues their files (#3961).
	staffSickNotes workforceModule.SickNoteRetention
	// auditLedgers deletes the expired rows of the append-only audit ledgers
	// through their fixed database functions (#3956).
	auditLedgers interface {
		DeleteExpiredAuthEvents(ctx context.Context) (deleted, retentionDays int, err error)
		DeleteExpiredDataAccessLog(ctx context.Context) (deleted, retentionDays int, err error)
		DeleteExpiredDeletionLog(ctx context.Context) (deleted, retentionDays int, err error)
		// DeleteExpiredProtocol deletes the protocol of one of
		// auditModels.ProtocolRetentionTypes after its fixed window (#4044).
		DeleteExpiredProtocol(ctx context.Context, kind string) (deleted, retentionDays int, err error)
	}
	// deliveries deletes the delivered mail and push intents and the mail
	// delivery logs after their fixed window (#3955).
	deliveries deliveryModule.Retention
	// The finished cleanup jobs, care-withdrawal completions and enrollment
	// rate-limit buckets of their owners, each after its fixed window (#4046).
	fileCleanups     fileStorageModule.CleanupRetention
	staffCleanups    workforceModule.CleanupRetention
	carePlanCleanups carePlanModule.CleanupRetention
	submissionLimits enrollmentModule.SubmissionRateLimitRetention
	// The setting protocol and the export journal of their owners, each
	// after its fixed window (#4044).
	settingAudit    settingsModule.AuditRetention
	exportTransfers exportTransferModule.JournalRetention
}

// studentRetention deletes the children whose last care day lies more than
// the school's window back, through the student deletion workflow (#3962).
// It reaches the Worker beside the owners: the workflow composes it.
type studentRetention interface {
	DeleteExpiredStudents(ctx context.Context) (studentdeletion.RetentionResult, error)
}

// staffRetention deletes the staff members whose exit lies more than the
// school's time-tracking window back, through the workflow beside the
// offboarding (#3963). It reaches the Worker beside the owners as well.
type staffRetention interface {
	DeleteExpiredStaff(ctx context.Context) (staffoffboarding.RetentionResult, error)
}

// guardianRetention deletes the parent contacts without a child after the
// school's window (#4043): People Directory deletes, Identity & Access names
// the contacts an open invitation still needs.
type guardianRetention struct {
	people      peopleModule.ChildlessGuardianRetention
	invitations peopleModule.OpenGuardianInvitations
}

// newGuardianRetention composes the deletion of parent contacts without a
// child over the People Directory the root builds over db and the open
// invitations of identity.
func newGuardianRetention(db transactionruntime.Database, identity *identityaccess.Module) guardianRetention {
	return guardianRetention{
		people:      peopleCompose.MustNewReads(db.Pool(), studentDirectoryView.NewReader()),
		invitations: openGuardianInvitations(identity),
	}
}

// openGuardianInvitations binds the People Directory port to the open
// invitations of Identity & Access.
func openGuardianInvitations(identity interface {
	ListOpenGuardianInvitations(ctx context.Context, guardianProfileIDs []int64) ([]identityaccess.GuardianInvitation, error)
}) peopleModule.OpenGuardianInvitations {
	return func(ctx context.Context, guardianIDs []int64) (map[int64]bool, error) {
		invitations, err := identity.ListOpenGuardianInvitations(ctx, guardianIDs)
		if err != nil {
			return nil, err
		}
		invited := make(map[int64]bool, len(invitations))
		for _, invitation := range invitations {
			invited[invitation.GuardianProfileID] = true
		}
		return invited, nil
	}
}

// newWorkerRetention composes the nightly retention. attendance deletes the
// daily attendance after the school's window (#3957), guardians the parent
// contacts without a child (#4043); like the workflows they reach the Worker
// as arguments, not as fields of the owners (ADR 0059).
func newWorkerRetention(owners retentionOwners, students studentRetention, staff staffRetention, attendance studentpresence.AttendanceRetention, guardians guardianRetention) (workerRetention, error) {
	if owners.deletionLog == nil || owners.settings == nil {
		return workerRetention{}, fmt.Errorf("compose worker retention: deletion log and settings are required")
	}
	if err := settingsCompose.VerifyPreloadKeys(settingsModule.RetentionWindowKeys()); err != nil {
		return workerRetention{}, fmt.Errorf("compose worker retention: %w", err)
	}
	deletions := owners.deletionLog
	return workerRetention{
		steps: schedulerPorts.RetentionPlan(studentRetentionStep(students), staffRetentionStep(staff),
			append(append(retentionSteps(owners), attendanceRetentionSteps(attendance, owners.settings)...),
				guardianRetentionSteps(guardians, owners.settings)...)),
		log: func(ctx context.Context, kind string, outcome schedulerPorts.RetentionOutcome) error {
			return deletions.LogRetention(ctx, kind, outcome.Deleted, outcome.RetentionDays)
		},
		runs: schedulerRuntimeCompose.NewCleanupRunLedger(),
	}, nil
}

// retentionSteps lists the deletions of the nightly cleanup of one school.
// A missing owner leaves its step out.
func retentionSteps(owners retentionOwners) []schedulerPorts.RetentionStep {
	var steps []schedulerPorts.RetentionStep
	if owners.unregisteredTagScans != nil {
		steps = append(steps, ownerRetentionStep(auditModels.DeletionTypeUnregisteredTagScanRetention, owners.unregisteredTagScans.DeleteUnregisteredTagScansAfterRetention))
	}
	if owners.statusDays != nil {
		statusDays := owners.statusDays
		steps = append(steps, schedulerPorts.RetentionStep{
			Kind: auditModels.DeletionTypeStatusDayRetention,
			Run: func(ctx context.Context) (schedulerPorts.RetentionOutcome, error) {
				window := settingsModule.NewStatusDayRetention(owners.settings)
				days, err := window.Days(ctx)
				if err != nil {
					return schedulerPorts.RetentionOutcome{}, err
				}
				deleted, err := statusDays.DeleteExpiredStudentStatusDays(ctx, days)
				if errors.Is(err, carePlanModule.ErrStatusDayRetentionWindow) {
					return schedulerPorts.RetentionOutcome{}, window.SettingError(err)
				}
				return schedulerPorts.RetentionOutcome{Deleted: deleted, RetentionDays: days}, err
			},
		})
	}
	if owners.parentMessages != nil {
		messages := owners.parentMessages
		steps = append(steps, schedulerPorts.RetentionStep{
			Kind: auditModels.DeletionTypeParentMessageRetention,
			Run: func(ctx context.Context) (schedulerPorts.RetentionOutcome, error) {
				result, err := messages.CleanupExpiredParentMessages(ctx)
				return schedulerPorts.RetentionOutcome{Deleted: result.Deleted(), RetentionDays: result.RetentionDays}, err
			},
		})
	}
	if owners.settledEnrollments != nil {
		enrollments := owners.settledEnrollments
		steps = append(steps, schedulerPorts.RetentionStep{
			Kind: auditModels.DeletionTypeSettledEnrollmentRetention,
			Run: func(ctx context.Context) (schedulerPorts.RetentionOutcome, error) {
				result, err := enrollments.CleanupSettledEnrollments(ctx)
				return schedulerPorts.RetentionOutcome{Deleted: result.Deleted(), RetentionDays: result.RetentionDays}, err
			},
		})
	}
	if owners.staffSickNotes != nil {
		steps = append(steps, ownerRetentionStep(auditModels.DeletionTypeStaffSickNoteRetention, owners.staffSickNotes.DeleteExpiredStaffSickNotes))
	}
	if owners.auditLedgers != nil {
		ledgers := owners.auditLedgers
		steps = append(steps,
			ownerRetentionStep(auditModels.DeletionTypeAuthEventRetention, ledgers.DeleteExpiredAuthEvents),
			ownerRetentionStep(auditModels.DeletionTypeDataAccessLogRetention, ledgers.DeleteExpiredDataAccessLog),
			ownerRetentionStep(auditModels.DeletionTypeDeletionLogRetention, ledgers.DeleteExpiredDeletionLog),
		)
	}
	if owners.deliveries != nil {
		steps = append(steps, ownerRetentionStep(auditModels.DeletionTypeDeliveryRetention, owners.deliveries.DeleteExpiredDeliveries))
	}
	steps = append(steps, protocolRetentionSteps(owners)...)
	return append(steps, cleanupJobRetentionSteps(owners)...)
}

// attendanceRetentionSteps is the deletion of the daily attendance after the
// school's window (#3957).
func attendanceRetentionSteps(attendance studentpresence.AttendanceRetention, settings settingsModule.StatisticsRetentionSource) []schedulerPorts.RetentionStep {
	if attendance == nil {
		return nil
	}
	return []schedulerPorts.RetentionStep{{
		Kind: auditModels.DeletionTypeAttendanceRetention,
		Run: func(ctx context.Context) (schedulerPorts.RetentionOutcome, error) {
			window := settingsModule.NewAttendanceRetention(settings)
			days, err := window.Days(ctx)
			if err != nil {
				return schedulerPorts.RetentionOutcome{}, err
			}
			deleted, err := attendance.DeleteExpiredAttendance(ctx, days)
			if errors.Is(err, studentpresence.ErrAttendanceRetentionWindow) {
				return schedulerPorts.RetentionOutcome{}, window.SettingError(err)
			}
			return schedulerPorts.RetentionOutcome{Deleted: deleted, RetentionDays: days}, err
		},
	}}
}

// guardianRetentionSteps is the deletion of the parent contacts without a
// child after the school's window (#4043).
func guardianRetentionSteps(guardians guardianRetention, settings settingsModule.StatisticsRetentionSource) []schedulerPorts.RetentionStep {
	if guardians.people == nil || guardians.invitations == nil {
		return nil
	}
	return []schedulerPorts.RetentionStep{{
		Kind: auditModels.DeletionTypeGuardianRetention,
		Run: func(ctx context.Context) (schedulerPorts.RetentionOutcome, error) {
			window := settingsModule.NewGuardianRetention(settings)
			days, err := window.Days(ctx)
			if err != nil {
				return schedulerPorts.RetentionOutcome{}, err
			}
			deleted, err := guardians.people.DeleteExpiredChildlessGuardians(ctx, days, guardians.invitations)
			if errors.Is(err, peopleModule.ErrGuardianRetentionWindow) {
				return schedulerPorts.RetentionOutcome{}, window.SettingError(err)
			}
			return schedulerPorts.RetentionOutcome{Deleted: deleted, RetentionDays: days}, err
		},
	}}
}

// studentRetentionStep is the automatic deletion of children (#3962). It
// runs first: the shorter windows of the other steps then apply to the rows
// of the children that stay (schedulerPorts.RetentionPlan).
func studentRetentionStep(students studentRetention) schedulerPorts.SubjectRetention {
	if students == nil {
		return schedulerPorts.SubjectRetention{}
	}
	return schedulerPorts.SubjectRetention{
		Kind: auditModels.DeletionTypeStudentRetention,
		Run: func(ctx context.Context) (schedulerPorts.RetentionOutcome, error) {
			result, err := students.DeleteExpiredStudents(ctx)
			return schedulerPorts.RetentionOutcome{Deleted: result.Deleted, RetentionDays: result.RetentionDays}, err
		},
	}
}

// staffRetentionStep is the automatic deletion of staff (#3963). It runs
// with the deletion of children before the other steps.
func staffRetentionStep(staff staffRetention) schedulerPorts.SubjectRetention {
	if staff == nil {
		return schedulerPorts.SubjectRetention{}
	}
	return schedulerPorts.SubjectRetention{
		Kind: auditModels.DeletionTypeStaffRetention,
		Run: func(ctx context.Context) (schedulerPorts.RetentionOutcome, error) {
			result, err := staff.DeleteExpiredStaff(ctx)
			return schedulerPorts.RetentionOutcome{Deleted: result.Deleted, RetentionDays: result.RetentionDays}, err
		},
	}
}

// newStaffRetention composes the automatic deletion of staff over the
// workflow beside the offboarding (#3963). The root builds the owners over
// db: School Membership with the Workforce employment profile, Workforce,
// Student Presence and the People Directory. The window is the school's
// time-tracking window; each staff member runs in a savepoint of the
// cleanup's tenant transaction and is logged into audit by the system.
func newStaffRetention(db transactionruntime.Database, audit auditModels.Command, settings interface {
	ResolveInt(ctx context.Context, key string) (int, error)
}, logger *slog.Logger) (*staffoffboarding.Retention, error) {
	memberDeps := schoolMembershipCompose.Dependencies{
		DB: db.Pool(), Observe: func(schoolMembershipCompose.Observation) {},
		Employment: MembershipStaffEmployment(workforceCompose.MustNewStaffEmployment(db.Pool())),
	}
	membership, err := schoolMembershipCompose.New(memberDeps)
	if err != nil {
		return nil, err
	}
	retired, err := schoolMembershipCompose.NewOffboarding(memberDeps)
	if err != nil {
		return nil, err
	}
	presence, err := presenceCompose.New(presenceCompose.Dependencies{DB: db.Pool(), Observe: func(presenceCompose.Observation) {}})
	if err != nil {
		return nil, err
	}
	plannedSupervisions, err := NewCleanupTimetable(db)
	if err != nil {
		return nil, err
	}
	retiredTimetable, ok := plannedSupervisions.(staffoffboarding.RetiredTimetable)
	if !ok {
		return nil, fmt.Errorf("compose staff retention: timetable lacks retired-staff deletion")
	}
	return staffoffboarding.NewRetention(staffoffboarding.RetentionDependencies{
		Window: settingsModule.StaffRetentionWindow(settings),
		Tenant: tenant.FromContext, Today: calendar.TodayDate,
		Membership: retired, LiveStaff: membership, Workforce: workforceCompose.NewRetiredStaffRecords(), Timetable: retiredTimetable,
		SupervisionIDs: func(ctx context.Context, staffID int64) ([]int64, error) {
			supervisions, err := presence.QueryGroupSupervisions(ctx, studentpresence.GroupSupervisionFilter{StaffIDs: []int64{staffID}})
			ids := make([]int64, 0, len(supervisions))
			for _, supervision := range supervisions {
				ids = append(ids, supervision.ID)
			}
			return ids, err
		},
		RemoveSupervision: presence.RemoveSupervision,
		People:            peopleCompose.MustNewReads(db.Pool(), studentDirectoryView.NewReader()),
		AppendAudit: func(ctx context.Context, erasure staffoffboarding.Erasure) error {
			return audit.Append(ctx, auditModels.NewStaffRetentionDeletion(
				tenant.FromContext(ctx), erasure.StaffID, erasure.Records(), erasure.PersonAnonymized, erasure.Counts(),
			))
		},
		Isolate: func(ctx context.Context, fn func(context.Context) error) error {
			err := tenant.WithSavepoint(ctx, fn)
			if errors.Is(err, tenant.ErrSavepointControl) {
				return errors.Join(staffoffboarding.ErrIsolationFailed, err)
			}
			return err
		},
		Logger: logger,
	})
}

// newStudentRetention composes the automatic deletion of children over the
// student deletion workflow (#3962): the school's window from the settings,
// each child in a savepoint of the cleanup's tenant transaction.
func newStudentRetention(workflow *studentdeletion.Workflow, settings interface {
	ResolveInt(ctx context.Context, key string) (int, error)
}, logger *slog.Logger) (*studentdeletion.Retention, error) {
	return studentdeletion.NewRetention(studentdeletion.RetentionDependencies{
		Workflow: workflow,
		Window:   settingsModule.StudentRetentionWindow(settings),
		Tenant:   tenant.FromContext,
		Isolate: func(ctx context.Context, fn func(context.Context) error) error {
			err := tenant.WithSavepoint(ctx, fn)
			if errors.Is(err, tenant.ErrSavepointControl) {
				return errors.Join(studentdeletion.ErrIsolationFailed, err)
			}
			return err
		},
		Logger: logger,
	})
}

// protocolRetentionSteps are the fixed-window deletions of the protocols of
// #4044: one step per protocol, so each logs its own data kind.
func protocolRetentionSteps(owners retentionOwners) []schedulerPorts.RetentionStep {
	var steps []schedulerPorts.RetentionStep
	if owners.auditLedgers != nil {
		ledgers := owners.auditLedgers
		for _, kind := range auditModels.ProtocolRetentionTypes() {
			steps = append(steps, ownerRetentionStep(kind, func(ctx context.Context) (int, int, error) {
				return ledgers.DeleteExpiredProtocol(ctx, kind)
			}))
		}
	}
	if owners.settingAudit != nil {
		steps = append(steps, ownerRetentionStep(auditModels.DeletionTypeSettingAuditRetention, owners.settingAudit.DeleteExpiredSettingChanges))
	}
	if owners.exportTransfers != nil {
		steps = append(steps, ownerRetentionStep(auditModels.DeletionTypeExportTransferRetention, owners.exportTransfers.DeleteExpiredTransfers))
	}
	return steps
}

// cleanupJobRetentionSteps are the fixed-window deletions of the technical
// remains (#4046).
func cleanupJobRetentionSteps(owners retentionOwners) []schedulerPorts.RetentionStep {
	var steps []schedulerPorts.RetentionStep
	if owners.fileCleanups != nil {
		steps = append(steps, ownerRetentionStep(auditModels.DeletionTypeFileCleanupRetention, owners.fileCleanups.DeleteFinishedCleanups))
	}
	if owners.staffCleanups != nil {
		steps = append(steps, ownerRetentionStep(auditModels.DeletionTypeStaffCleanupRetention, owners.staffCleanups.DeleteFinishedStaffCleanups))
	}
	if owners.carePlanCleanups != nil {
		steps = append(steps,
			ownerRetentionStep(auditModels.DeletionTypeStudentDocumentCleanupRetention, owners.carePlanCleanups.DeleteFinishedStudentDocumentCleanups),
			ownerRetentionStep(auditModels.DeletionTypeCareWithdrawalRetention, owners.carePlanCleanups.DeleteFinishedWithdrawalCompletions),
		)
	}
	if owners.submissionLimits != nil {
		steps = append(steps, ownerRetentionStep(auditModels.DeletionTypeSubmissionRateLimitRetention, owners.submissionLimits.DeleteExpiredSubmissionRateLimits))
	}
	return steps
}

// ownerRetentionStep runs an owner deletion that reports its deleted rows and
// the window it applied: the same for every school, or the school's window
// the owner resolves itself.
func ownerRetentionStep(kind string, deleteExpired func(context.Context) (int, int, error)) schedulerPorts.RetentionStep {
	return schedulerPorts.RetentionStep{
		Kind: kind,
		Run: func(ctx context.Context) (schedulerPorts.RetentionOutcome, error) {
			deleted, days, err := deleteExpired(ctx)
			return schedulerPorts.RetentionOutcome{Deleted: deleted, RetentionDays: days}, err
		},
	}
}
