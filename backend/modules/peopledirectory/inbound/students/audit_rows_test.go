package students_test

import (
	"time"

	"github.com/uptrace/bun"
)

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

type dataAccessLogRow struct {
	bun.BaseModel `bun:"table:audit.data_access_log,alias:data_access_log"`
	ID            int64          `bun:"id,pk,autoincrement"`
	RangeStart    time.Time      `bun:"range_start"`
	RangeEnd      time.Time      `bun:"range_end"`
	Metadata      map[string]any `bun:"metadata,type:jsonb"`
}

type studentDeletionRow struct {
	bun.BaseModel  `bun:"table:audit.student_deletions,alias:student_deletion"`
	ID             int64                    `bun:"id,pk,autoincrement"`
	ActorAccountID int64                    `bun:"actor_account_id"`
	Reason         string                   `bun:"reason"`
	Counts         studentDeletionCountsRow `bun:"counts,type:jsonb"`
}

// studentDeletionCountsRow is the whole stored count object, so an equality
// assertion still pins every count, not only the one a test names.
type studentDeletionCountsRow struct {
	TimetableAssignments int `json:"timetable_assignments"`
	ActivityEnrollments  int `json:"activity_enrollments"`
	AttendanceRecords    int `json:"attendance_records"`
	CareSchedules        int `json:"care_schedules"`
	GuardianLinks        int `json:"guardian_links"`
	CompanionLinks       int `json:"companion_links"`
	Communications       int `json:"communications"`
	Consents             int `json:"consents"`
	EnrollmentReferences int `json:"enrollment_references"`
	OtherRecords         int `json:"other_records"`
}

type dataDeletionRow struct {
	bun.BaseModel  `bun:"table:audit.data_deletions,alias:data_deletion"`
	ID             int64          `bun:"id,pk,autoincrement"`
	RecordsDeleted int            `bun:"records_deleted"`
	DeletionReason string         `bun:"deletion_reason"`
	Metadata       map[string]any `bun:"metadata,type:jsonb"`
}

type studentConsentChangeRow struct {
	bun.BaseModel `bun:"table:audit.student_consent_changes,alias:student_consent_change"`
	ID            int64     `bun:"id,pk,autoincrement"`
	TenantID      int64     `bun:"tenant_id"`
	StudentID     int64     `bun:"student_id"`
	ConsentKey    string    `bun:"consent_key"`
	Action        string    `bun:"action"`
	Source        string    `bun:"source"`
	CreatedAt     time.Time `bun:"created_at"`
	UpdatedAt     time.Time `bun:"updated_at"`
}
