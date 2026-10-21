package ports

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// AuthCleanup exposes the identity maintenance the composition root binds:
// the expired session sweep of the retained auth service plus the spent
// reset links and stale rate-limit windows Identity & Access owns (#3332).
type AuthCleanup interface {
	CleanupExpiredTokens(ctx context.Context) (int, error)
	DeleteSpentPasswordResetTokens(ctx context.Context) (int, error)
	DeleteStalePasswordResetWindows(ctx context.Context) (int, error)
}

// InvitationCleaner exposes the school invitation maintenance Identity &
// Access owns (#2722): the links whose expiry has passed.
type InvitationCleaner interface {
	DeleteExpiredSchoolInvitations(ctx context.Context) (int, error)
}

// StaffMessageCleanupResult reports the records removed by one tenant's
// staff-message retention sweep.
type StaffMessageCleanupResult struct {
	MessagesDeleted int
	ThreadsDeleted  int
	RetentionDays   int
}

// StaffMessageCleanup is supplied by the composition root. The scheduler
// owns when cleanup runs but remains independent of the Communication module.
type StaffMessageCleanup func(context.Context) (StaffMessageCleanupResult, error)

// RejectedEnrollmentCleaner removes the rejected enrollments of the tenant in
// context after their retention window and reports how many requests, late
// invites and outbox rows it removed. The composition root supplies it; the
// scheduler owns when it runs but stays independent of Enrollment.
type RejectedEnrollmentCleaner interface {
	CleanupRejectedEnrollments(ctx context.Context) (requests int, lateInvites, outboxRows int64, err error)
}

// WorkerTracer is injected by the Worker composition root. The scheduler owns
// job names and outcomes; the adapter owns correlation, logs, and metrics.
type WorkerTracer struct {
	StartJob func(context.Context, string) (context.Context, error)
	Logger   func(context.Context) *slog.Logger
	Failure  func(context.Context, string, string, error)
	Run      func(JobID, string, time.Duration)
	Batch    func(TenantBatchEvidence)
	Backlog  func(JobID, int)
}

// TenantBatchEvidence is the bounded, low-cardinality runtime record emitted
// after each tenant batch.
type TenantBatchEvidence struct {
	JobID     JobID
	Duration  time.Duration
	Processed int
	Failed    int
	Retries   int
	Backlog   int
	PoolWait  time.Duration
}

// WorkSessionCleaner exposes the cleanup routine for stale work sessions.
type WorkSessionCleaner interface {
	CleanupOpenSessions(ctx context.Context) (int, error)
}

// BreakAutoEnder exposes the method to auto-end expired breaks.
type BreakAutoEnder interface {
	AutoEndExpiredBreaks(ctx context.Context) (int, error)
}

// AutoCheckouter exposes the method to close open work sessions at their
// planned shift end (#1798).
type AutoCheckouter interface {
	AutoCheckoutDueSessions(ctx context.Context, grace time.Duration) (int, error)
}

// EmailChangeTokenCleaner exposes the cleanup routine for operator e-mail
// change links: expired links are spent so their operator can request a new
// one, then the rows nobody counts any more are deleted.
type EmailChangeTokenCleaner interface {
	CleanupOperatorEmailChanges(ctx context.Context) (int, error)
}

// OperatorInvitationCleaner exposes the Identity & Access token maintenance
// of the hourly cleanup: the operator invitation links and the fixed
// retention of the short-lived sign-in data (#3954).
type OperatorInvitationCleaner interface {
	DeleteExpiredOperatorInvitations(ctx context.Context) (int, error)
	SignInDataCleaner
}

// OperatorRetention deletes the moto team's own data after its fixed window
// (#4047): the operator ledger entries before 1 January of last year, then
// the operator accounts deactivated before that date. The ledger goes first,
// because an entry still pointing at an account refuses its deletion.
type OperatorRetention struct {
	Ledger interface {
		DeleteExpiredOperatorAuditLog(ctx context.Context) (int, error)
	}
	Accounts interface {
		DeleteDeactivatedOperators(ctx context.Context) (int, error)
	}
}

// AccountRetention tombstones the accounts no school needs any more (#4043)
// in the hourly cleanup.
type AccountRetention interface {
	EraseUnusedAccounts(ctx context.Context) (int, error)
}

// SignInDataCleaner exposes the fixed retention of the short-lived sign-in
// data Identity & Access owns (#3954): invitations for parents, e-mail codes
// and passkey ceremonies of accounts and operators, and expired operator
// sessions.
type SignInDataCleaner interface {
	DeleteExpiredGuardianInvitations(ctx context.Context) (int, error)
	DeleteExpiredMFAEmailChallenges(ctx context.Context) (int, error)
	DeleteExpiredPasskeySessions(ctx context.Context) (int, error)
	DeleteExpiredOperatorSessions(ctx context.Context, now time.Time) (int, error)
}

type CalendarFeedCleaner interface {
	CleanupExpiredFeedTombstones(ctx context.Context) (int, error)
}

// FeedbackCleaner exposes the cleanup routine for old feedback entries.
type FeedbackCleaner interface {
	DeleteExpired(ctx context.Context) (int, error)
}

// StaffDocumentFileCleaner removes files whose staff-document metadata either
// never committed or belongs to an offboarded staff member.
type StaffDocumentFileCleaner interface {
	CleanupOrphanedStaffDocumentFiles(ctx context.Context) (int, error)
}

// StudentDocumentFileCleaner removes objects whose child-document metadata
// either never committed or was cascaded away by a child deletion.
type StudentDocumentFileCleaner interface {
	CleanupOrphanedStudentDocumentFiles(ctx context.Context) (int, error)
}

// FileStoreCleaner removes objects of the school file storage whose metadata
// either never committed or was cascaded away by a folder deletion (#2596).
type FileStoreCleaner interface {
	CleanupOrphanedFiles(ctx context.Context) (int, error)
}

// SettingsResolver resolves setting values per tenant: the school's override,
// else the registry default. The root binds the Settings Platform service.
type SettingsResolver interface {
	ResolveString(ctx context.Context, key string) (string, error)
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveInt(ctx context.Context, key string) (int, error)
}

// OutboxWorkerRunner is the narrow contract the scheduler needs from the
// platform outbox worker. Defined here so the scheduler doesn't import
// services/platform.
type OutboxWorkerRunner interface {
	RunOnce(ctx context.Context, batchSize, maxAttempts int) (int, error)
	Backlog(ctx context.Context) (int, error)
}

// TimetableBridgeCompleter finalizes attendance and completes the schedule-side
// instances of ended active.groups in one step. Implemented by the Timetable
// owner's timetable.EndedSessionCompletion, the same completion the
// force-start path uses, so both paths leave identical rows behind (#1747).
type TimetableBridgeCompleter interface {
	CompleteActiveByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, completedAt time.Time) (int64, error)
}

// DayInstance is one timetable block of a day as the overdue tick reads it.
// Planned reports the block's composed status: planned until a session starts
// it. StartTime carries the block's wall clock.
type DayInstance struct {
	ID        int64
	Date      calendar.Date
	StartTime time.Time
	RoomID    int64
	Planned   bool
}

// DayInstanceReader returns the timetable blocks of the school in ctx on day.
// The root binds the retained activity-instance reads.
type DayInstanceReader func(ctx context.Context, day calendar.Date) ([]DayInstance, error)

// ExistingRoomReader returns which of roomIDs name a room of the school in
// ctx. The root binds the Facilities room reads.
type ExistingRoomReader func(ctx context.Context, roomIDs []int64) ([]int64, error)

// InstanceCheckoutCloser stamps the checkout on the open slot attendance of
// the ended active groups. The root binds the retained instance-student rows.
type InstanceCheckoutCloser interface {
	CloseOpenCheckoutsByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, checkedOutAt time.Time) (int, error)
}

// StudentChangeLogCleanupResult reports one tenant's change-history sweep.
type StudentChangeLogCleanupResult struct {
	EditsDeleted     int
	StudentsAffected int
	RetentionDays    int
	DurationMS       int64
}

// StudentChangeLogCleanup removes the tenant's change history past its
// retention. The root binds the People Directory sweep; the scheduler owns
// when it runs.
type StudentChangeLogCleanup func(context.Context) (StudentChangeLogCleanupResult, error)
