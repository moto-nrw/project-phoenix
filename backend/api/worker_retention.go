package api

import (
	"context"
	"fmt"

	carePlanModule "github.com/moto-nrw/project-phoenix/modules/careplan"
	communicationModule "github.com/moto-nrw/project-phoenix/modules/communication"
	deliveryModule "github.com/moto-nrw/project-phoenix/modules/delivery"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	enrollmentModule "github.com/moto-nrw/project-phoenix/modules/enrollment"
	exportTransferModule "github.com/moto-nrw/project-phoenix/modules/exporttransfer"
	fileStorageModule "github.com/moto-nrw/project-phoenix/modules/filestorage"
	schedulerRuntimeCompose "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/compose"
	schedulerPorts "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"
	settingsModule "github.com/moto-nrw/project-phoenix/modules/settings"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// workerRetention is the retention capability of the nightly cleanup (#3950):
// the owners' deletion steps and their deletion log, which the Worker runs,
// and the ledger of each school's last successful run.
type workerRetention struct {
	steps []schedulerPorts.RetentionStep
	log   schedulerPorts.RetentionLog
	runs  schedulerPorts.CleanupRunLedger
}

// Data kinds of the nightly retention steps. Each one is the deletion type
// the Audit Platform records for its step in audit.data_deletions.
const (
	retentionUnregisteredTagScans = "unregistered_tag_scan_retention"
	retentionStatusDays           = "status_day_retention"
	retentionParentMessages       = "parent_message_retention"
	retentionSettledEnrollments   = "settled_enrollment_retention"
	retentionStaffSickNotes       = "staff_sick_note_retention"
	retentionAuthEvents           = "auth_event_retention"
	retentionDataAccessLog        = "data_access_log_retention"
	retentionDeletionLog          = "deletion_log_retention"
	retentionDeliveries           = "delivery_retention"
	// The technical remains of #4046.
	retentionFileCleanups           = "file_cleanup_retention"
	retentionStaffCleanups          = "staff_cleanup_retention"
	retentionStudentDocumentCleanup = "student_document_cleanup_retention"
	retentionCareWithdrawals        = "care_withdrawal_retention"
	retentionSubmissionRateLimits   = "submission_rate_limit_retention"
	// The protocols of #4044: the setting protocol and the export journal
	// of their owners; the rest are the Audit Platform's (auditProtocols).
	retentionSettingAudit    = "setting_audit_retention"
	retentionExportTransfers = "export_transfer_retention"
)

// auditProtocols are the protocols of #4044 the Audit Platform deletes, each
// after its fixed window and logged as its own data kind.
var auditProtocols = []string{
	"student_deletion_log_retention",
	"enrollment_deletion_log_retention",
	"enrollment_restoration_log_retention",
	"time_tracking_deletion_log_retention",
	"staff_master_data_change_retention",
	"personnel_number_change_retention",
	"guardian_financial_change_retention",
	"class_list_change_retention",
	"substitution_change_retention",
	"file_event_retention",
}

var retentionSettingKeys = []string{
	settingsModule.KeyGDPRStatusDayRetentionDays,
	settingsModule.KeyGDPRParentMessageRetentionDays,
	settingsModule.KeyGDPRWithdrawnEnrollmentRetentionDays,
	settingsModule.KeyGDPRStaffSickNoteRetentionDays,
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
		// auditProtocols after its fixed window (#4044).
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

func newWorkerRetention(owners retentionOwners) (workerRetention, error) {
	if owners.deletionLog == nil || owners.settings == nil {
		return workerRetention{}, fmt.Errorf("compose worker retention: deletion log and settings are required")
	}
	if err := settingsCompose.VerifyPreloadKeys(retentionSettingKeys); err != nil {
		return workerRetention{}, fmt.Errorf("compose worker retention: %w", err)
	}
	deletions := owners.deletionLog
	return workerRetention{
		steps: retentionSteps(owners),
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
		steps = append(steps, ownerRetentionStep(retentionUnregisteredTagScans, owners.unregisteredTagScans.DeleteUnregisteredTagScansAfterRetention))
	}
	if owners.statusDays != nil {
		statusDays := owners.statusDays
		steps = append(steps, schedulerPorts.RetentionStep{
			Kind: retentionStatusDays,
			Run: func(ctx context.Context) (schedulerPorts.RetentionOutcome, error) {
				days, err := retentionDays(ctx, owners.settings, settingsModule.KeyGDPRStatusDayRetentionDays)
				if err != nil {
					return schedulerPorts.RetentionOutcome{}, err
				}
				deleted, err := statusDays.DeleteStudentStatusDaysBefore(ctx, carePlanModule.Date(retentionCutoff(days)))
				return schedulerPorts.RetentionOutcome{Deleted: deleted, RetentionDays: days}, err
			},
		})
	}
	if owners.parentMessages != nil {
		messages := owners.parentMessages
		steps = append(steps, schedulerPorts.RetentionStep{
			Kind: retentionParentMessages,
			Run: func(ctx context.Context) (schedulerPorts.RetentionOutcome, error) {
				result, err := messages.CleanupExpiredParentMessages(ctx)
				return schedulerPorts.RetentionOutcome{Deleted: result.MessagesDeleted + result.ThreadsDeleted, RetentionDays: result.RetentionDays}, err
			},
		})
	}
	if owners.settledEnrollments != nil {
		enrollments := owners.settledEnrollments
		steps = append(steps, schedulerPorts.RetentionStep{
			Kind: retentionSettledEnrollments,
			Run: func(ctx context.Context) (schedulerPorts.RetentionOutcome, error) {
				result, err := enrollments.CleanupSettledEnrollments(ctx)
				deleted := result.DeletedRequests + int(result.DeletedLateInvites)
				return schedulerPorts.RetentionOutcome{Deleted: deleted, RetentionDays: result.RetentionDays}, err
			},
		})
	}
	if owners.staffSickNotes != nil {
		steps = append(steps, ownerRetentionStep(retentionStaffSickNotes, owners.staffSickNotes.DeleteExpiredStaffSickNotes))
	}
	if owners.auditLedgers != nil {
		ledgers := owners.auditLedgers
		steps = append(steps,
			ownerRetentionStep(retentionAuthEvents, ledgers.DeleteExpiredAuthEvents),
			ownerRetentionStep(retentionDataAccessLog, ledgers.DeleteExpiredDataAccessLog),
			ownerRetentionStep(retentionDeletionLog, ledgers.DeleteExpiredDeletionLog),
		)
	}
	if owners.deliveries != nil {
		steps = append(steps, ownerRetentionStep(retentionDeliveries, owners.deliveries.DeleteExpiredDeliveries))
	}
	steps = append(steps, protocolRetentionSteps(owners)...)
	return append(steps, cleanupJobRetentionSteps(owners)...)
}

// protocolRetentionSteps are the fixed-window deletions of the protocols of
// #4044: one step per protocol, so each logs its own data kind.
func protocolRetentionSteps(owners retentionOwners) []schedulerPorts.RetentionStep {
	var steps []schedulerPorts.RetentionStep
	if owners.auditLedgers != nil {
		ledgers := owners.auditLedgers
		for _, kind := range auditProtocols {
			steps = append(steps, ownerRetentionStep(kind, func(ctx context.Context) (int, int, error) {
				return ledgers.DeleteExpiredProtocol(ctx, kind)
			}))
		}
	}
	if owners.settingAudit != nil {
		steps = append(steps, ownerRetentionStep(retentionSettingAudit, owners.settingAudit.DeleteExpiredSettingChanges))
	}
	if owners.exportTransfers != nil {
		steps = append(steps, ownerRetentionStep(retentionExportTransfers, owners.exportTransfers.DeleteExpiredTransfers))
	}
	return steps
}

// cleanupJobRetentionSteps are the fixed-window deletions of the technical
// remains (#4046).
func cleanupJobRetentionSteps(owners retentionOwners) []schedulerPorts.RetentionStep {
	var steps []schedulerPorts.RetentionStep
	if owners.fileCleanups != nil {
		steps = append(steps, ownerRetentionStep(retentionFileCleanups, owners.fileCleanups.DeleteFinishedCleanups))
	}
	if owners.staffCleanups != nil {
		steps = append(steps, ownerRetentionStep(retentionStaffCleanups, owners.staffCleanups.DeleteFinishedStaffCleanups))
	}
	if owners.carePlanCleanups != nil {
		steps = append(steps,
			ownerRetentionStep(retentionStudentDocumentCleanup, owners.carePlanCleanups.DeleteFinishedStudentDocumentCleanups),
			ownerRetentionStep(retentionCareWithdrawals, owners.carePlanCleanups.DeleteFinishedWithdrawalCompletions),
		)
	}
	if owners.submissionLimits != nil {
		steps = append(steps, ownerRetentionStep(retentionSubmissionRateLimits, owners.submissionLimits.DeleteExpiredSubmissionRateLimits))
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

// retentionDays resolves a school's retention window. It never guesses: an
// unreadable or non-positive window fails the step, so nothing is deleted
// on a wrong period and the run is caught up once the setting is fixed.
func retentionDays(ctx context.Context, settings interface {
	ResolveInt(ctx context.Context, key string) (int, error)
}, key string) (int, error) {
	days, err := settings.ResolveInt(ctx, key)
	if err != nil {
		return 0, fmt.Errorf("resolve %s: %w", key, err)
	}
	if days <= 0 {
		return 0, fmt.Errorf("resolve %s: retention window must be positive, got %d", key, days)
	}
	return days, nil
}

// retentionCutoff is the first Berlin calendar day a window of days keeps:
// data dated before it is past its retention.
func retentionCutoff(days int) calendar.Date {
	return calendar.TodayDate().AddDays(-days)
}
