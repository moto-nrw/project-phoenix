package audit

import (
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// ResourceType constants for DataAccessLog.ResourceType.
const (
	ResourceTypeAttendanceHistory = "attendance_history"
	// ResourceTypeAttendanceDayLog records a group day-log view/export
	// (issue #1456): per-group daily statuses of many children at once.
	// StudentID stays NULL; metadata carries the group scope.
	ResourceTypeAttendanceDayLog = "attendance_day_log"
	// ResourceTypeStudentStatusDayOverview records a group-scoped bulk view of
	// registered student absences over a requested date range.
	ResourceTypeStudentStatusDayOverview = "student_status_day_overview"
	// ResourceTypeEnrollmentPhaseExport records a bulk export of every
	// registration (guardian + child PII) in one enrollment phase. The
	// row's range_start/range_end carry the phase's service window —
	// the temporal span of the disclosed data — and metadata carries
	// phase_id, format, request_count and child_count.
	ResourceTypeEnrollmentPhaseExport = "enrollment_phase_export"
	// ResourceTypeEnrollmentStudentExport records an export of enrollment
	// answers attached to one student profile.
	ResourceTypeEnrollmentStudentExport = "enrollment_student_export"
	// ResourceTypeStaffFinancialView records serving the MASKED bank & tax
	// data of one staff member (#1423). StudentID stays NULL; metadata
	// carries staff_id.
	ResourceTypeStaffFinancialView = "staff_financial_view"
	// ResourceTypeStaffFinancialReveal records serving the FULL (unmasked)
	// bank & tax data of one staff member after the explicit
	// "Anzeigen"-toggle (#1423). StudentID stays NULL; metadata carries
	// staff_id.
	ResourceTypeStaffFinancialReveal = "staff_financial_reveal"
	// ResourceTypeGuardianFinancialView records serving the MASKED bank
	// details of one guardian (#2608). StudentID stays NULL; metadata carries
	// guardian_profile_id.
	ResourceTypeGuardianFinancialView = "guardian_financial_view"
	// ResourceTypeGuardianFinancialReveal records serving the FULL (unmasked)
	// IBAN of one guardian after the explicit "Anzeigen"-toggle (#2608).
	// StudentID stays NULL; metadata carries guardian_profile_id.
	ResourceTypeGuardianFinancialReveal = "guardian_financial_reveal"
	// ResourceTypeGuardianPaymentOverview records serving the school-wide
	// Bankverbindungen list (#2608) — one row per child with a payer, masked
	// IBANs. StudentID stays NULL; metadata carries the row count.
	ResourceTypeGuardianPaymentOverview = "guardian_payment_overview"
	// ResourceTypeGuardianPaymentExport records a bulk export of the
	// Bankverbindungen list (#2608) with UNMASKED IBANs — the most sensitive
	// read the tenant offers. StudentID stays NULL; metadata carries the row
	// count and the export format.
	ResourceTypeGuardianPaymentExport = "guardian_payment_export"
	// ResourceTypeStaffDocumentDownload records serving a sensitive staff
	// document (#1424): AU-Bescheinigungen (Art. 9 health data) and
	// Lohnabrechnungen. StudentID stays NULL; metadata carries staff_id,
	// document_id and category.
	ResourceTypeStaffDocumentDownload = "staff_document_download"
	// ResourceTypeStudentDocumentDownload records serving a sensitive child
	// document (#777): Attest, Impfnachweis and Medikamentenplan (Art. 9
	// health data) plus the Sorgerechtsnachweis. Unlike the staff rows above
	// this one names a child, so StudentID is SET and metadata carries only
	// document_id and category.
	ResourceTypeStudentDocumentDownload = "student_document_download"
	// ResourceTypeClassDayView records serving the read-only per-class day
	// view (#1772): care and departure data of one class on one date, shown
	// to a Lehrkraft (or any other class_day:read holder). StudentID stays
	// NULL; metadata carries school_class, date and student_count.
	ResourceTypeClassDayView = "class_day_view"
	// ResourceTypeSupervisionStudentSheet records serving ONE child's pickup
	// and emergency contacts to a supervisor running the block that child is
	// in (#2527). The class day view (ResourceTypeClassDayView) deliberately
	// carries no contact details at all, so this is the wider disclosure and
	// gets its own resource type — and, unlike the class day view, no
	// deduplication: the sheet opens only on a deliberate tap.
	ResourceTypeSupervisionStudentSheet = "supervision_student_sheet"
)

// DataAccessLog is an append-only record of a staff member viewing sensitive
// tenant data (currently: per-student attendance history). Written for GDPR
// auditability. No retention/cleanup policy exists yet — this matches the
// existing convention for other tables in the audit schema.
type DataAccessLog struct {
	ID int64 `bun:"id,pk,autoincrement" json:"id"`
	base.TenantModel
	ActorAccountID int64     `bun:"actor_account_id,notnull" json:"actor_account_id"`
	ActorRole      string    `bun:"actor_role,notnull" json:"actor_role"`
	ResourceType   string    `bun:"resource_type,notnull" json:"resource_type"`
	StudentID      *int64    `bun:"student_id" json:"student_id,omitempty"`
	RangeStart     time.Time `bun:"range_start,notnull" json:"range_start"`
	RangeEnd       time.Time `bun:"range_end,notnull" json:"range_end"`
	AccessedAt     time.Time `bun:"accessed_at,notnull,default:now()" json:"accessed_at"`
	// Metadata holds event-specific context (migration 1.15.101).
	// Mirrors the metadata column on the sibling audit tables.
	Metadata map[string]interface{} `bun:"metadata,type:jsonb" json:"metadata,omitempty"`
}

// GetID implements the base.Entity interface.
func (d *DataAccessLog) GetID() interface{} {
	return d.ID
}

// GetCreatedAt implements the base.Entity interface.
func (d *DataAccessLog) GetCreatedAt() time.Time {
	return d.AccessedAt
}

// GetUpdatedAt implements the base.Entity interface. Access log rows are
// append-only, so updated_at mirrors accessed_at.
func (d *DataAccessLog) GetUpdatedAt() time.Time {
	return d.AccessedAt
}

// GetMetadata returns the metadata map, lazily initialising it.
func (d *DataAccessLog) GetMetadata() map[string]interface{} {
	if d.Metadata == nil {
		d.Metadata = make(map[string]interface{})
	}
	return d.Metadata
}

// SetMetadata sets a single metadata key.
func (d *DataAccessLog) SetMetadata(key string, value interface{}) {
	if d.Metadata == nil {
		d.Metadata = make(map[string]interface{})
	}
	d.Metadata[key] = value
}
