package audit

import (
	"context"
	"time"

	auditModel "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/uptrace/bun"
)

// Repositories is the Audit Platform persistence the Audit composition builds
// for the retained consumers (ADR 0049). Every field keeps the model-typed
// contract the consumers already depend on.
type Repositories struct {
	// AppendStore is the only adapter that inserts ledger rows; the audit
	// command appends through it.
	AppendStore auditModel.AppendStore

	FileEvent                    auditModel.FileEventRepository
	SubstitutionChange           auditModel.SubstitutionChangeCreator
	DataDeletion                 auditModel.DataDeletionRepository
	StudentDeletionAudit         auditModel.StudentDeletionRepository
	EnrollmentDeletionAudit      auditModel.EnrollmentDeletionRepository
	EnrollmentRestorationAudit   auditModel.EnrollmentRestorationRepository
	DataAccessLog                auditModel.DataAccessLogRepository
	EnrollmentOfferingAdjustment auditModel.EnrollmentOfferingAdjustmentRepository
	GuardianChange               auditModel.GuardianChangeRepository
	StudentConsentChange         auditModel.StudentConsentChangeRepository
	DeviationEvent               auditModel.DeviationEventRepository
	AuthEvent                    auditModel.AuthEventRepository
	DataImport                   auditModel.DataImportRepository
	WorkSessionEdit              auditModel.WorkSessionEditRepository
	StudentFieldEdit             auditModel.StudentFieldEditRepository
	TimeTrackingDeletion         auditModel.TimeTrackingDeletionRepository
	PersonnelNumberChange        auditModel.PersonnelNumberChangeCreator
	StaffMasterDataChange        auditModel.StaffMasterDataChangeCreator
	GuardianFinancialChange      auditModel.GuardianFinancialChangeCreator
	ClassListEntryChange         auditModel.ClassListEntryChangeRepository
	TimeTrackingAuditLog         auditModel.TimeTrackingAuditLogRepository
	AttendanceCorrection         auditModel.AttendanceCorrectionRepository
	// OperatorAuditLog is the Audit-owned operator ledger (#2720).
	OperatorAuditLog auditModel.OperatorAuditLogRepository
	// BookingConsistency is the cross-domain booking audit; nil when the
	// composition was built without the Enrollment projection.
	BookingConsistency auditModel.BookingConsistencyRepository
	// RetentionSummaries is the cleanup read over the deletion ledger, kept
	// apart so the producer-facing DataDeletionRepository stays narrow.
	RetentionSummaries RetentionSummaries
}

// RetentionSummaries lists the recent automated deletion runs.
type RetentionSummaries interface {
	ListRecentRetentionSummaries(ctx context.Context, since time.Time, limit int) ([]auditModel.RecentDeletionSummary, error)
}

// NewRepositories binds every retained Audit repository to reads and the
// append store to appends.
func NewRepositories(reads, appends Runtime) *Repositories {
	deletions := NewDataDeletionRepository(reads)
	return &Repositories{
		AppendStore:                  NewAppender(appends),
		FileEvent:                    NewFileEventRepository(reads),
		SubstitutionChange:           NewSubstitutionChangeRepository(reads),
		DataDeletion:                 deletions,
		StudentDeletionAudit:         NewStudentDeletionRepository(reads),
		EnrollmentDeletionAudit:      NewEnrollmentDeletionRepository(reads),
		EnrollmentRestorationAudit:   NewEnrollmentRestorationRepository(reads),
		DataAccessLog:                NewDataAccessLogRepository(reads),
		EnrollmentOfferingAdjustment: NewEnrollmentOfferingAdjustmentRepository(reads),
		GuardianChange:               NewGuardianChangeRepository(reads),
		StudentConsentChange:         NewStudentConsentChangeRepository(reads),
		DeviationEvent:               NewDeviationEventRepository(reads),
		AuthEvent:                    NewAuthEventRepository(reads),
		DataImport:                   NewDataImportRepository(reads),
		WorkSessionEdit:              NewWorkSessionEditRepository(reads),
		StudentFieldEdit:             NewStudentFieldEditRepository(reads),
		TimeTrackingDeletion:         NewTimeTrackingDeletionRepository(reads),
		PersonnelNumberChange:        NewPersonnelNumberChangeRepository(reads),
		StaffMasterDataChange:        NewStaffMasterDataChangeRepository(reads),
		GuardianFinancialChange:      NewGuardianFinancialChangeRepository(reads),
		ClassListEntryChange:         NewClassListEntryChangeRepository(reads),
		TimeTrackingAuditLog:         NewTimeTrackingAuditLogRepository(reads),
		AttendanceCorrection:         NewAttendanceCorrectionRepository(reads),
		OperatorAuditLog:             NewOperatorAuditLogRepository(reads),
		RetentionSummaries:           deletions,
	}
}

// NewAmbientRuntime resolves the Audit handoff the caller put on the context:
// its transaction, failing closed outside a unit of work. Command
// roots and test graphs read through it.
func NewAmbientRuntime(db *bun.DB) Runtime {
	if db == nil {
		panic("audit repository: database is required")
	}
	return func(ctx context.Context) (bun.IDB, int64) {
		tenantID := auditModel.TenantIDFromContext(ctx)
		if raw, ok := auditModel.TransactionFromContext(ctx); ok {
			switch tx := raw.(type) {
			case bun.Tx:
				return tx, tenantID
			case *bun.Tx:
				if tx != nil {
					return tx, tenantID
				}
			}
		}
		return nil, tenantID
	}
}
