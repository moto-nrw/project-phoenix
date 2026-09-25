// Package application holds Enrollment's phase administration, form-schema
// publishing, phase-expiry warnings, captcha verification and parent mails.
// Every dependency outside the owner's own records arrives through the
// consumer-owned ports below; the composition binds them.
package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
)

// Transactions opens a tenant transaction for a standalone caller or joins
// the caller's ambient one.
type Transactions interface {
	RunInTx(ctx context.Context, fn func(context.Context) error) error
}

// Runtime is the tenant runtime the services read: the ambient tenant, the
// rollback mark of a committed 4xx, and the owner's not-found signal.
type Runtime struct {
	Transactions Transactions
	// TenantID returns the tenant in context, or 0 outside one.
	TenantID func(context.Context) int64
	// MarkRollback discards the ambient transaction although the request
	// answers with a client error.
	MarkRollback func(context.Context)
	// NotFound reports whether an owner read found no row.
	NotFound func(error) bool
	// NoRows is the error an owner read that found no row wraps; the
	// decision flow wraps a missing student the same way.
	NoRows error
	// InTransaction reports whether the context carries a tenant
	// transaction.
	InTransaction func(context.Context) bool
	// Savepoint runs fn inside a savepoint of the ambient transaction. A
	// savepoint-control failure is fatal to that transaction.
	Savepoint func(ctx context.Context, fn func(context.Context) error) error
	// IsSavepointControl reports a savepoint-control failure.
	IsSavepointControl func(error) bool
	// WithinCurrentTenant runs fn in a new transaction of the tenant in
	// context.
	WithinCurrentTenant func(ctx context.Context, fn func(context.Context) error) error
	// AfterCommit queues fn to run once the ambient transaction commits, or
	// runs it at once outside a transaction.
	AfterCommit func(ctx context.Context, fn func())
}

// CollectionSettings resolves the tenant settings that decide which grades
// and classes the enrollment form collects. LockClassCollectionPair takes the
// per-tenant lock the settings side takes before turning collection off, so
// the two sides of the invariant cannot both pass on a stale read (#1663).
type CollectionSettings interface {
	CollectGradeLevel(ctx context.Context) (bool, error)
	CollectSchoolClass(ctx context.Context) (bool, error)
	GradeLevelMax(ctx context.Context) (int, error)
	LockClassCollectionPair(ctx context.Context) error
}

// PhaseRecords is the owner capability the phase administration writes.
type PhaseRecords interface {
	Phase(context.Context, int64) (*enrollment.Phase, error)
	Phases(context.Context) ([]*enrollment.Phase, error)
	PublicOpenPhases(context.Context, time.Time) ([]*enrollment.Phase, error)
	InsertPhase(context.Context, *enrollment.Phase) error
	UpdatePhase(context.Context, *enrollment.Phase) error
	Schema(context.Context, int64) (*enrollment.FormSchema, error)
	CountPhaseRequests(context.Context, int64) (int, error)
	CountCreatedStudentsByPhase(context.Context, int64) (int, error)
	RemovePhase(context.Context, int64) (int, error)
	PhaseResponseChildren(ctx context.Context, phaseID int64, statuses []string) ([]enrollment.PhaseResponseChild, error)
}

// PhaseOfferings reads the Care Plan offerings linked to a phase.
type PhaseOfferings interface {
	OfferingIDsForPhase(ctx context.Context, phaseID int64) ([]int64, error)
	CountOfferingsForPhase(ctx context.Context, phaseID int64) (int, error)
}

// CalendarPeriods is the slice of the School Calendar the phase link
// validation reads.
type CalendarPeriods interface {
	FindCalendarPeriod(ctx context.Context, id int64) (schoolcalendar.CalendarPeriod, error)
}

// FormSchemaRecords is the owner capability schema publishing uses.
type FormSchemaRecords interface {
	ActiveSchema(context.Context) (*enrollment.FormSchema, error)
	Schema(context.Context, int64) (*enrollment.FormSchema, error)
	SchemaVersions(context.Context) ([]*enrollment.FormSchema, error)
	LockSchemaLineages(context.Context) error
	NextSchemaVersionForName(context.Context, string) (int, error)
	RenameSchema(context.Context, int64, string) (*enrollment.FormSchema, error)
	DeleteUnusedSchema(context.Context, int64) (string, error)
	PublishSchema(context.Context, enrollment.FormSchema) (*enrollment.FormSchema, error)
}

// CaptchaSettings resolves the captcha settings of the tenant in context.
type CaptchaSettings interface {
	CaptchaRequired(ctx context.Context) (bool, error)
	CaptchaSecretKey(ctx context.Context) (string, error)
	CaptchaSiteKey(ctx context.Context) (string, error)
}

// CaptchaSiteVerify asks the captcha provider whether a token is valid.
type CaptchaSiteVerify interface {
	SiteVerify(ctx context.Context, secret, token, remoteIP string) (success bool, errorCodes []string, err error)
}

// NotificationSettings resolves the tenant's decision notification mode.
type NotificationSettings interface {
	NotifyPerDecision(ctx context.Context) (string, error)
}

// NotificationModePin atomically pins a request's decision notification
// mode and returns the mode that holds.
type NotificationModePin interface {
	PinDecisionNotificationMode(ctx context.Context, requestID int64, proposedMode string) (string, error)
}

// Fingerprint returns a stable content identity for idempotency keys.
type Fingerprint func(content []byte) string

// ExpirySnapshots is the owner report the phase-expiry projection feeds.
type ExpirySnapshots interface {
	PhaseExpirySnapshots(context.Context, enrollment.PhaseExpiryInput) ([]*enrollment.PhaseExpirySnapshot, error)
}

// ApprovedSelections reads Enrollment's approved offering selections.
type ApprovedSelections interface {
	ApprovedSelectionsForStudents(context.Context, []int64, enrollment.Date, enrollment.Date) ([]*enrollment.ApprovedOfferingSelection, error)
	ApprovedSelectionsForOfferings(context.Context, []int64, enrollment.Date) ([]*enrollment.ApprovedOfferingSelection, error)
}

// OfferingStudent is one child as the approved-offering projection resolves
// it through People Directory.
type OfferingStudent struct {
	ID          int64
	SchoolClass string
	Alumnus     bool
}

// OfferingStudentDirectory reads the current class and alumnus flag of the
// given children.
type OfferingStudentDirectory interface {
	ListOfferingStudents(context.Context, []int64) ([]OfferingStudent, error)
}
