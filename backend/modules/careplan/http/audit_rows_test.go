package careplanhttp_test

// The Audit Platform rows these suites read back or seed (#3356). Each row
// maps only the columns the assertions use; the stored vocabulary below is
// what the trail keeps, spelled out as the values the readers key on.
const (
	auditResourceStudentStatusDayOverview = "student_status_day_overview"
	auditDeletionTypeManual               = "manual"
	auditOfferingAdjustmentSourceDirect   = "direct"
	auditOfferingAdjustmentSourceRequest  = "request"
	auditConsentPhoto                     = "photo"
	auditConsentWithdrawn                 = "withdrawn"
	auditConsentSourceParentPortal        = "parent_portal"
	auditFieldPickupSchedule              = "pickup_schedule"
)
