package test

import (
	"encoding/json"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The People Directory rows the fixtures write. People Directory keeps its
// row mapping in models/users, which the shared fixtures may not import
// (#2748, #3966), so test support maps the columns it writes and its callers
// read. A caller that hands a row to People Directory code reads the owner's
// model back through the owner instead.

// Student lifecycle states as users.student_school_memberships.status stores
// them.
const (
	StudentStatusPending  = "pending"
	StudentStatusActive   = "active"
	StudentStatusInactive = "inactive"
	StudentStatusAlumnus  = "alumnus"
)

// Person is one users.persons row.
type Person struct {
	bun.BaseModel `bun:"table:users.persons,alias:person"`
	ID            int64      `bun:"id,pk,autoincrement"`
	TenantID      int64      `bun:"tenant_id,notnull"`
	CreatedAt     time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	FirstName     string     `bun:"first_name,notnull"`
	LastName      string     `bun:"last_name,notnull"`
	TagID         *string    `bun:"tag_id"`
	AccountID     *int64     `bun:"account_id"`
	DeletedAt     *time.Time `bun:"deleted_at,soft_delete,nullzero"`
}

// Staff is one staff member: the School Membership row of
// users.staff_school_memberships, whose id every dependent table references,
// with the employment-profile columns the fixtures set (#2754).
type Staff struct {
	bun.BaseModel         `bun:"alias:staff"`
	ID                    int64      `bun:"id,pk,autoincrement"`
	TenantID              int64      `bun:"tenant_id,notnull"`
	CreatedAt             time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt             time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	PersonID              int64      `bun:"person_id,notnull"`
	StaffNotes            string     `bun:"staff_notes"`
	PersonnelNumber       *string    `bun:"personnel_number"`
	BirthdayDisplayOptOut bool       `bun:"birthday_display_opt_out,notnull"`
	DeletedAt             *time.Time `bun:"deleted_at,soft_delete,nullzero"`

	// Person is the fixture's person row; it is not stored with the staff row.
	Person *Person `bun:"-"`
}

// Teacher is one users.teachers row.
type Teacher struct {
	bun.BaseModel  `bun:"table:users.teachers,alias:teacher"`
	ID             int64      `bun:"id,pk,autoincrement"`
	TenantID       int64      `bun:"tenant_id,notnull"`
	CreatedAt      time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt      time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	StaffID        int64      `bun:"staff_id,notnull"`
	Specialization string     `bun:"specialization,nullzero"`
	Role           string     `bun:"role"`
	Qualifications string     `bun:"qualifications"`
	DeletedAt      *time.Time `bun:"deleted_at,soft_delete,nullzero"`

	// Staff is the fixture's staff row; it is not stored with the teacher row.
	Staff *Staff `bun:"-"`
}

// Student is one child: the student profile id with the School Membership
// and care-profile columns the fixtures set and their callers update.
type Student struct {
	bun.BaseModel `bun:"alias:student"`
	ID            int64      `bun:"id,pk,autoincrement"`
	TenantID      int64      `bun:"tenant_id,notnull"`
	CreatedAt     time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	PersonID      int64      `bun:"person_id,notnull"`
	SchoolClass   string     `bun:"school_class,notnull"`
	GroupID       *int64     `bun:"group_id"`
	Status        string     `bun:"status,notnull,default:'active'"`
	Sick          *bool      `bun:"sick"`
	SickSince     *time.Time `bun:"sick_since"`
	Excused       *bool      `bun:"excused"`
	ExcusedSince  *time.Time `bun:"excused_since"`
}

// GuardianProfile is one users.guardian_profiles row.
type GuardianProfile struct {
	bun.BaseModel          `bun:"table:users.guardian_profiles,alias:guardian_profile"`
	ID                     int64     `bun:"id,pk,autoincrement"`
	TenantID               int64     `bun:"tenant_id,notnull"`
	CreatedAt              time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt              time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	FirstName              string    `bun:"first_name"`
	LastName               string    `bun:"last_name"`
	Email                  *string   `bun:"email"`
	AccountID              *int64    `bun:"account_id"`
	HasAccount             bool      `bun:"has_account,notnull,default:false"`
	PreferredContactMethod string    `bun:"preferred_contact_method,default:'phone'"`
	LanguagePreference     string    `bun:"language_preference,default:'de'"`
}

// StudentGuardian is one student-guardian link in the row shape the
// production guardian-link projection joins from its three owners: People
// Directory's relationship, Care Plan's pickup permission and Identity &
// Access's portal access.
type StudentGuardian struct {
	bun.BaseModel      `bun:"alias:student_guardian"`
	ID                 int64          `bun:"id,pk,autoincrement"`
	TenantID           int64          `bun:"tenant_id,notnull"`
	CreatedAt          time.Time      `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt          time.Time      `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	StudentID          int64          `bun:"student_id,notnull"`
	GuardianProfileID  int64          `bun:"guardian_profile_id,notnull"`
	RelationshipType   string         `bun:"relationship_type,notnull"`
	GuardianRole       string         `bun:"guardian_role,notnull,default:'custom'"`
	IsPrimary          bool           `bun:"is_primary,notnull"`
	IsEmergencyContact bool           `bun:"is_emergency_contact,notnull"`
	CanPickup          bool           `bun:"can_pickup,notnull"`
	PickupNotes        *string        `bun:"pickup_notes"`
	EmergencyPriority  int            `bun:"emergency_priority,default:1"`
	IsPayer            bool           `bun:"is_payer,notnull"`
	Permissions        map[string]any `bun:"permissions,type:jsonb,nullzero"`
	AccessAccountID    *int64         `bun:"access_account_id,scanonly"`
}

// ExternalCaregiverRecord is one users.external_caregivers row.
type ExternalCaregiverRecord struct {
	bun.BaseModel     `bun:"table:users.external_caregivers,alias:external_caregiver"`
	ID                int64          `bun:"id,pk,autoincrement"`
	TenantID          int64          `bun:"tenant_id,notnull"`
	CreatedAt         time.Time      `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt         time.Time      `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	StaffID           int64          `bun:"staff_id,notnull"`
	Organization      string         `bun:"organization"`
	ContactEmail      string         `bun:"contact_email"`
	ContactPhone      string         `bun:"contact_phone"`
	ActivityExpertise string         `bun:"activity_expertise,notnull"`
	StartDate         *calendar.Date `bun:"start_date"`
	EndDate           *calendar.Date `bun:"end_date"`
	Notes             string         `bun:"notes"`

	// Staff is the fixture's staff row; it is not stored with the record.
	Staff *Staff `bun:"-"`
}

// PrivacyConsent is one users.privacy_consents row.
type PrivacyConsent struct {
	bun.BaseModel     `bun:"table:users.privacy_consents,alias:privacy_consent"`
	ID                int64          `bun:"id,pk,autoincrement"`
	TenantID          int64          `bun:"tenant_id,notnull"`
	CreatedAt         time.Time      `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt         time.Time      `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	StudentID         int64          `bun:"student_id,notnull"`
	PolicyVersion     string         `bun:"policy_version,notnull"`
	Accepted          bool           `bun:"accepted,notnull"`
	AcceptedAt        *time.Time     `bun:"accepted_at"`
	ExpiresAt         *time.Time     `bun:"expires_at"`
	DurationDays      *int           `bun:"duration_days"`
	RenewalRequired   bool           `bun:"renewal_required,notnull"`
	DataRetentionDays int            `bun:"data_retention_days,notnull"`
	Details           map[string]any `bun:"details,type:jsonb"`
}

// ClassListEntry is one users.class_list_entries row (#2382).
type ClassListEntry struct {
	bun.BaseModel `bun:"table:users.class_list_entries,alias:class_list_entry"`
	ID            int64     `bun:"id,pk,autoincrement"`
	TenantID      int64     `bun:"tenant_id,notnull"`
	CreatedAt     time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	FirstName     string    `bun:"first_name,notnull"`
	LastName      string    `bun:"last_name,notnull"`
	SchoolClass   string    `bun:"school_class,notnull"`
	CreatedBy     *int64    `bun:"created_by"`
}

// ParentAnnouncement is one users.parent_announcements row: a plain
// Mitteilung, so every column the fixture leaves alone takes its default.
type ParentAnnouncement struct {
	bun.BaseModel           `bun:"table:users.parent_announcements,alias:parent_announcement"`
	ID                      int64     `bun:"id,pk,autoincrement"`
	TenantID                int64     `bun:"tenant_id,notnull"`
	CreatedAt               time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt               time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	Title                   string    `bun:"title,notnull"`
	Body                    string    `bun:"body,notnull"`
	Priority                string    `bun:"priority,notnull"`
	RequiresAcknowledgement bool      `bun:"requires_acknowledgement,notnull"`
	SendEmail               bool      `bun:"send_email,notnull"`
	Active                  bool      `bun:"active,notnull"`
	CreatedBy               int64     `bun:"created_by,notnull"`
}

// CareWithdrawalCompletion is one users.care_withdrawal_completions row.
type CareWithdrawalCompletion struct {
	bun.BaseModel           `bun:"table:users.care_withdrawal_completions,alias:care_withdrawal_completion"`
	ID                      int64           `bun:"id,pk,autoincrement"`
	TenantID                int64           `bun:"tenant_id,notnull"`
	CreatedAt               time.Time       `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt               time.Time       `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	StudentID               *int64          `bun:"student_id"`
	FirstBookinglessDay     calendar.Date   `bun:"first_bookingless_day,type:date,notnull"`
	Trigger                 string          `bun:"trigger,notnull"`
	WithdrawalConfirmedBy   *int64          `bun:"withdrawal_confirmed_by"`
	WithdrawalConfirmedRole string          `bun:"withdrawal_confirmed_role,notnull"`
	WithdrawalConfirmedAt   time.Time       `bun:"withdrawal_confirmed_at,notnull"`
	SourceOfferings         json.RawMessage `bun:"source_offerings,type:jsonb,notnull"`
	State                   string          `bun:"state,notnull"`
}
