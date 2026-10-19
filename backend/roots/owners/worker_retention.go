package owners

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	carePlanModule "github.com/moto-nrw/project-phoenix/modules/careplan"
	communicationModule "github.com/moto-nrw/project-phoenix/modules/communication"
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

// retentionDeletionLog is the deletion log every retention step reports to.
type retentionDeletionLog interface {
	LogRetention(ctx context.Context, kind string, deleted, retentionDays int) error
}

// retentionSettings resolves the retention windows of the school in context.
type retentionSettings interface {
	ResolveInt(ctx context.Context, key string) (int, error)
}

// retentionAuditLedgers deletes the expired rows of the append-only audit
// ledgers through their fixed database functions (#3956).
type retentionAuditLedgers interface {
	DeleteExpiredAuthEvents(ctx context.Context) (deleted, retentionDays int, err error)
	DeleteExpiredDataAccessLog(ctx context.Context) (deleted, retentionDays int, err error)
	DeleteExpiredDeletionLog(ctx context.Context) (deleted, retentionDays int, err error)
	// DeleteExpiredProtocol deletes the protocol of one of
	// auditModels.ProtocolRetentionTypes after its fixed window (#4044).
	DeleteExpiredProtocol(ctx context.Context, kind string) (deleted, retentionDays int, err error)
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

// guardianRetention deletes parent contacts that no longer have a child.
// Identity & Access names the contacts whose open invitation must keep them.
type guardianRetention struct {
	people      peopleModule.ChildlessGuardianRetention
	invitations peopleModule.OpenGuardianInvitations
}

func newGuardianRetention(db transactionruntime.Database, invitations identityaccess.GuardianInvitationReads) guardianRetention {
	return guardianRetention{
		people:      peopleCompose.MustNewReads(db.Pool(), studentDirectoryView.NewReader()),
		invitations: openGuardianInvitations(invitations),
	}
}

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

// newWorkerRetention composes the nightly cleanup over the deletion log, the
// settings the windows resolve from, the subject deletions and the owner
// steps, in the order dataRetentionSteps, protocolRetentionSteps and
// cleanupJobRetentionSteps list them. attendance deletes the daily
// attendance after the school's window (#3957); like the workflows it
// reaches the Worker as an argument, not as a field of the owners (ADR 0059).
func newWorkerRetention(
	deletionLog retentionDeletionLog, settings retentionSettings, students studentRetention, staff staffRetention,
	steps []schedulerPorts.RetentionStep, attendance studentpresence.AttendanceRetention, guardians guardianRetention,
) (workerRetention, error) {
	if deletionLog == nil || settings == nil {
		return workerRetention{}, fmt.Errorf("compose worker retention: deletion log and settings are required")
	}
	if err := settingsCompose.NewPreloadKeyVerifier()(settingsModule.RetentionWindowKeys()); err != nil {
		return workerRetention{}, fmt.Errorf("compose worker retention: %w", err)
	}
	deletions := deletionLog
	return workerRetention{
		steps: schedulerPorts.RetentionPlan(studentRetentionStep(students), staffRetentionStep(staff),
			append(append(steps, attendanceRetentionSteps(attendance, settings)...), guardianRetentionSteps(guardians, settings)...)),
		log: func(ctx context.Context, kind string, outcome schedulerPorts.RetentionOutcome) error {
			return deletions.LogRetention(ctx, kind, outcome.Deleted, outcome.RetentionDays)
		},
		runs: schedulerRuntimeCompose.NewCleanupRunLedger(),
	}, nil
}

// dataRetentionSteps lists the owner data deletions of the nightly cleanup of
// one school; each owner resolves its own window, the status days theirs
// from settings. The settled enrollments are the withdrawn and lapsed ones no
// child was created from (#3960), the sick notes (AU-Bescheinigungen) queue
// their files (#3961), and the deliveries are the delivered mail and push
// intents and the mail delivery logs (#3955). A missing owner leaves its step
// out.
func dataRetentionSteps(
	settings retentionSettings, unregisteredTagScans devicefleet.UnregisteredTagScanRetention,
	statusDays carePlanModule.StudentStatusDayRetention, parentMessages communicationModule.ParentMessageCleanup,
	settledEnrollments enrollmentModule.SettledEnrollmentRetention, staffSickNotes workforceModule.SickNoteRetention,
	auditLedgers retentionAuditLedgers, deliveries deliveryContract.Retention,
) []schedulerPorts.RetentionStep {
	var steps []schedulerPorts.RetentionStep
	if unregisteredTagScans != nil {
		steps = append(steps, ownerRetentionStep(auditModels.DeletionTypeUnregisteredTagScanRetention, unregisteredTagScans.DeleteUnregisteredTagScansAfterRetention))
	}
	if statusDays != nil {
		steps = append(steps, schedulerPorts.RetentionStep{
			Kind: auditModels.DeletionTypeStatusDayRetention,
			Run: func(ctx context.Context) (schedulerPorts.RetentionOutcome, error) {
				window := settingsModule.NewStatusDayRetention(settings)
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
	if parentMessages != nil {
		steps = append(steps, schedulerPorts.RetentionStep{
			Kind: auditModels.DeletionTypeParentMessageRetention,
			Run: func(ctx context.Context) (schedulerPorts.RetentionOutcome, error) {
				result, err := parentMessages.CleanupExpiredParentMessages(ctx)
				return schedulerPorts.RetentionOutcome{Deleted: result.Deleted(), RetentionDays: result.RetentionDays}, err
			},
		})
	}
	if settledEnrollments != nil {
		steps = append(steps, schedulerPorts.RetentionStep{
			Kind: auditModels.DeletionTypeSettledEnrollmentRetention,
			Run: func(ctx context.Context) (schedulerPorts.RetentionOutcome, error) {
				result, err := settledEnrollments.CleanupSettledEnrollments(ctx)
				return schedulerPorts.RetentionOutcome{Deleted: result.Deleted(), RetentionDays: result.RetentionDays}, err
			},
		})
	}
	if staffSickNotes != nil {
		steps = append(steps, ownerRetentionStep(auditModels.DeletionTypeStaffSickNoteRetention, staffSickNotes.DeleteExpiredStaffSickNotes))
	}
	if auditLedgers != nil {
		steps = append(steps,
			ownerRetentionStep(auditModels.DeletionTypeAuthEventRetention, auditLedgers.DeleteExpiredAuthEvents),
			ownerRetentionStep(auditModels.DeletionTypeDataAccessLogRetention, auditLedgers.DeleteExpiredDataAccessLog),
			ownerRetentionStep(auditModels.DeletionTypeDeletionLogRetention, auditLedgers.DeleteExpiredDeletionLog),
		)
	}
	if deliveries != nil {
		steps = append(steps, ownerRetentionStep(auditModels.DeletionTypeDeliveryRetention, deliveries.DeleteExpiredDeliveries))
	}
	return steps
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
func protocolRetentionSteps(auditLedgers retentionAuditLedgers, settingAudit settingsModule.AuditRetention, exportTransfers exportTransferModule.JournalRetention) []schedulerPorts.RetentionStep {
	var steps []schedulerPorts.RetentionStep
	if auditLedgers != nil {
		for _, kind := range auditModels.ProtocolRetentionTypes() {
			steps = append(steps, ownerRetentionStep(kind, func(ctx context.Context) (int, int, error) {
				return auditLedgers.DeleteExpiredProtocol(ctx, kind)
			}))
		}
	}
	if settingAudit != nil {
		steps = append(steps, ownerRetentionStep(auditModels.DeletionTypeSettingAuditRetention, settingAudit.DeleteExpiredSettingChanges))
	}
	if exportTransfers != nil {
		steps = append(steps, ownerRetentionStep(auditModels.DeletionTypeExportTransferRetention, exportTransfers.DeleteExpiredTransfers))
	}
	return steps
}

// cleanupJobRetentionSteps are the fixed-window deletions of the technical
// remains (#4046): the finished cleanup jobs, care-withdrawal completions and
// enrollment rate-limit buckets of their owners.
func cleanupJobRetentionSteps(
	fileCleanups fileStorageModule.CleanupRetention, staffCleanups workforceModule.CleanupRetention,
	carePlanCleanups carePlanModule.CleanupRetention, submissionLimits enrollmentModule.SubmissionRateLimitRetention,
) []schedulerPorts.RetentionStep {
	var steps []schedulerPorts.RetentionStep
	if fileCleanups != nil {
		steps = append(steps, ownerRetentionStep(auditModels.DeletionTypeFileCleanupRetention, fileCleanups.DeleteFinishedCleanups))
	}
	if staffCleanups != nil {
		steps = append(steps, ownerRetentionStep(auditModels.DeletionTypeStaffCleanupRetention, staffCleanups.DeleteFinishedStaffCleanups))
	}
	if carePlanCleanups != nil {
		steps = append(steps,
			ownerRetentionStep(auditModels.DeletionTypeStudentDocumentCleanupRetention, carePlanCleanups.DeleteFinishedStudentDocumentCleanups),
			ownerRetentionStep(auditModels.DeletionTypeCareWithdrawalRetention, carePlanCleanups.DeleteFinishedWithdrawalCompletions),
		)
	}
	if submissionLimits != nil {
		steps = append(steps, ownerRetentionStep(auditModels.DeletionTypeSubmissionRateLimitRetention, submissionLimits.DeleteExpiredSubmissionRateLimits))
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
