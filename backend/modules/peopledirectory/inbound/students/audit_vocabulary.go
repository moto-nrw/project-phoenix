package students

import "strings"

// Resource types of the GDPR disclosure trail (audit.data_access_log) these
// routes append through Student Presence's RecordDataAccess. They are stored
// values the Audit Platform's readers key on, so they never change.
const (
	dataAccessAttendanceHistory        = "attendance_history"
	dataAccessAttendanceDayLog         = "attendance_day_log"
	dataAccessStudentStatusDayOverview = "student_status_day_overview"
	dataAccessStudentHealthListExport  = "student_health_list_export"
)

// consentSourceTenantPortal marks a consent change the school recorded, in
// the Audit Platform's consent trail (audit.student_consent_changes).
const consentSourceTenantPortal = "tenant_portal"

// The change history (audit.student_field_edits) records a document upload
// or deletion under "document_<category>"; the bare "document" is the
// categoryless form written before the category moved into the field name.
const (
	documentHistoryField       = "document"
	documentHistoryFieldPrefix = "document_"
)

// documentHistoryCategory reports whether a change-history field records a
// document event and, for the categorised form, its category.
func documentHistoryCategory(field string) (category string, isDocument bool) {
	if category, ok := strings.CutPrefix(field, documentHistoryFieldPrefix); ok {
		return category, true
	}
	return "", field == documentHistoryField
}
