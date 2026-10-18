package test

import (
	"encoding/json"
	"time"

	"github.com/uptrace/bun"
)

// DataDeletion is one audit.data_deletions row the fixtures write. The Audit
// Platform keeps its row mapping in its records port, which the shared
// fixtures may not import (#2748), so test support maps the table itself.
type DataDeletion struct {
	bun.BaseModel  `bun:"table:audit.data_deletions,alias:data_deletion"`
	ID             int64          `bun:"id,pk,autoincrement"`
	TenantID       int64          `bun:"tenant_id,notnull"`
	StudentID      *int64         `bun:"student_id"`
	StaffID        *int64         `bun:"staff_id"`
	DeletionType   string         `bun:"deletion_type,notnull"`
	RecordsDeleted int            `bun:"records_deleted,notnull"`
	DeletionReason string         `bun:"deletion_reason"`
	DeletedBy      string         `bun:"deleted_by,notnull"`
	DeletedAt      time.Time      `bun:"deleted_at,notnull,default:now()"`
	Metadata       map[string]any `bun:"metadata,type:jsonb"`
}

// EnrollmentOfferingAdjustment is one audit.enrollment_offering_adjustments
// row the fixtures write.
type EnrollmentOfferingAdjustment struct {
	bun.BaseModel               `bun:"table:audit.enrollment_offering_adjustments,alias:enrollment_offering_adjustment"`
	ID                          int64           `bun:"id,pk,autoincrement"`
	TenantID                    int64           `bun:"tenant_id,notnull"`
	RequestID                   int64           `bun:"request_id,notnull"`
	RequestChildID              int64           `bun:"request_child_id,notnull"`
	StudentID                   int64           `bun:"student_id,notnull"`
	ActorAccountID              int64           `bun:"actor_account_id,notnull"`
	ActorRole                   string          `bun:"actor_role,notnull"`
	ActorNameSnapshot           *string         `bun:"actor_name_snapshot"`
	ActorEmailSnapshot          *string         `bun:"actor_email_snapshot"`
	Reason                      string          `bun:"reason,notnull"`
	Source                      string          `bun:"source,notnull"`
	Before                      json.RawMessage `bun:"before_json,type:jsonb,notnull"`
	After                       json.RawMessage `bun:"after_json,type:jsonb,notnull"`
	CompleteWithdrawalConfirmed bool            `bun:"complete_withdrawal_confirmed,notnull"`
	ChangedAt                   time.Time       `bun:"changed_at,notnull,default:now()"`
}

// AuditStammdatenSectionSchoolClasses is the stored audit.staff_master_data_changes
// section of a staff member's class assignments (#1772).
const AuditStammdatenSectionSchoolClasses = "school-classes"

// The audit rows and stored values the People Directory and Care Plan
// contract suites assert. Those suites may not import the Audit records either,
// so the rows map the columns the suites read.

// Stored audit.staff_master_data_changes sections (#1772).
const (
	AuditStammdatenSectionPerson     = "person"
	AuditStammdatenSectionBankSteuer = "bank-steuer"
	AuditStammdatenSectionDokumente  = "dokumente"
)

// Stored audit.data_access_log resource types.
const (
	AuditResourceTypeStaffFinancialView    = "staff_financial_view"
	AuditResourceTypeStaffFinancialReveal  = "staff_financial_reveal"
	AuditResourceTypeStaffDocumentDownload = "staff_document_download"
)

// Stored audit.data_deletions deletion types.
const (
	AuditDeletionTypeManual                    = "manual"
	AuditDeletionTypeVisitRetention            = "visit_retention"
	AuditDeletionTypeStudentChangeLogRetention = "student_change_log_retention"
)

// Stored audit.auth_events event types of the caregiver capability.
const (
	AuditEventTypeCaregiverCapabilityEnabled  = "caregiver_capability_enabled"
	AuditEventTypeCaregiverCapabilityDisabled = "caregiver_capability_disabled"
)

// Stored audit.guardian_financial_changes field names.
const (
	AuditGuardianPaymentFieldIBAN          = "iban"
	AuditGuardianPaymentFieldAccountHolder = "account_holder"
	AuditGuardianPaymentFieldIsPayer       = "is_payer"
)

// AuditTimeTrackingDeletionSourceAbsence is the stored
// audit.time_tracking_deletions source of a deleted staff absence.
const AuditTimeTrackingDeletionSourceAbsence = "absence"

// AuditLogSourcePersonnelNumber is the time-tracking audit feed source of a
// personnel number change.
const AuditLogSourcePersonnelNumber = "personnel_number"

// StaffMasterDataChange is one audit.staff_master_data_changes row.
type StaffMasterDataChange struct {
	bun.BaseModel `bun:"table:audit.staff_master_data_changes,alias:staff_master_data_change"`
	ID            int64     `bun:"id,pk,autoincrement"`
	TenantID      int64     `bun:"tenant_id,notnull"`
	StaffID       int64     `bun:"staff_id,notnull"`
	ChangedBy     int64     `bun:"changed_by,notnull"`
	Section       string    `bun:"section,notnull"`
	FieldName     string    `bun:"field_name,notnull"`
	OldValue      string    `bun:"old_value,notnull"`
	NewValue      string    `bun:"new_value,notnull"`
	Note          string    `bun:"note,notnull"`
	OccurredAt    time.Time `bun:"occurred_at,nullzero,notnull,default:now()"`
}

// DataAccessLog is one audit.data_access_log row.
type DataAccessLog struct {
	bun.BaseModel  `bun:"table:audit.data_access_log,alias:data_access_log"`
	ID             int64          `bun:"id,pk,autoincrement"`
	TenantID       int64          `bun:"tenant_id,notnull"`
	ActorAccountID int64          `bun:"actor_account_id,notnull"`
	ActorRole      string         `bun:"actor_role,notnull"`
	ResourceType   string         `bun:"resource_type,notnull"`
	StudentID      *int64         `bun:"student_id"`
	RangeStart     time.Time      `bun:"range_start,notnull"`
	RangeEnd       time.Time      `bun:"range_end,notnull"`
	AccessedAt     time.Time      `bun:"accessed_at,notnull,default:now()"`
	Metadata       map[string]any `bun:"metadata,type:jsonb"`
}

// PersonnelNumberChange is one audit.personnel_number_changes row.
type PersonnelNumberChange struct {
	bun.BaseModel `bun:"table:audit.personnel_number_changes,alias:personnel_number_change"`
	ID            int64     `bun:"id,pk,autoincrement"`
	TenantID      int64     `bun:"tenant_id,notnull"`
	StaffID       int64     `bun:"staff_id,notnull"`
	ChangedBy     int64     `bun:"changed_by,notnull"`
	OldValue      string    `bun:"old_value,notnull"`
	NewValue      string    `bun:"new_value,notnull"`
	Note          string    `bun:"note,notnull"`
	OccurredAt    time.Time `bun:"occurred_at,nullzero,notnull,default:now()"`
}

// TimeTrackingDeletion is one audit.time_tracking_deletions row.
type TimeTrackingDeletion struct {
	bun.BaseModel `bun:"table:audit.time_tracking_deletions,alias:time_tracking_deletion"`
	ID            int64           `bun:"id,pk,autoincrement"`
	TenantID      int64           `bun:"tenant_id,notnull"`
	StaffID       int64           `bun:"staff_id,notnull"`
	Source        string          `bun:"source,notnull"`
	SourceID      int64           `bun:"source_id,notnull"`
	DeletedBy     int64           `bun:"deleted_by,notnull"`
	Payload       json.RawMessage `bun:"payload,type:jsonb,notnull"`
	Note          string          `bun:"note,notnull"`
	OccurredAt    time.Time       `bun:"occurred_at,nullzero,notnull,default:now()"`
}

// AuthEvent is one audit.auth_events row.
type AuthEvent struct {
	bun.BaseModel `bun:"table:audit.auth_events,alias:auth_event"`
	ID            int64          `bun:"id,pk,autoincrement"`
	TenantID      int64          `bun:"tenant_id,notnull"`
	AccountID     int64          `bun:"account_id,notnull"`
	EventType     string         `bun:"event_type,notnull"`
	Success       bool           `bun:"success,notnull"`
	IPAddress     string         `bun:"ip_address,notnull"`
	UserAgent     string         `bun:"user_agent"`
	ErrorMessage  string         `bun:"error_message"`
	Metadata      map[string]any `bun:"metadata,type:jsonb"`
	CreatedAt     time.Time      `bun:"created_at,notnull,default:now()"`
}

// GuardianFinancialChange is one audit.guardian_financial_changes row.
type GuardianFinancialChange struct {
	bun.BaseModel     `bun:"table:audit.guardian_financial_changes,alias:guardian_financial_change"`
	ID                int64     `bun:"id,pk,autoincrement"`
	TenantID          int64     `bun:"tenant_id,notnull"`
	GuardianProfileID int64     `bun:"guardian_profile_id,notnull"`
	StudentID         *int64    `bun:"student_id"`
	ChangedBy         int64     `bun:"changed_by,notnull"`
	FieldName         string    `bun:"field_name,notnull"`
	OldValue          string    `bun:"old_value,notnull"`
	NewValue          string    `bun:"new_value,notnull"`
	Note              string    `bun:"note,notnull"`
	OccurredAt        time.Time `bun:"occurred_at,nullzero,notnull,default:now()"`
}

// AuditStudentFieldDocumentPrefix starts the audit.student_field_edits field
// name of a child-document upload or deletion; the category follows (#777).
const AuditStudentFieldDocumentPrefix = "document_"

// AuditResourceTypeStudentDocumentDownload is the stored
// audit.data_access_log resource type of a sensitive child-document download.
const AuditResourceTypeStudentDocumentDownload = "student_document_download"

// StudentFieldEdit is one audit.student_field_edits row.
type StudentFieldEdit struct {
	bun.BaseModel `bun:"table:audit.student_field_edits,alias:student_field_edit"`
	ID            int64     `bun:"id,pk,autoincrement"`
	TenantID      int64     `bun:"tenant_id,notnull"`
	StudentID     int64     `bun:"student_id,notnull"`
	EditedBy      int64     `bun:"edited_by,notnull"`
	EditedByName  string    `bun:"edited_by_name,notnull"`
	FieldName     string    `bun:"field_name,notnull"`
	OldValue      *string   `bun:"old_value"`
	NewValue      *string   `bun:"new_value"`
	CreatedAt     time.Time `bun:"created_at,notnull,default:now()"`
}
