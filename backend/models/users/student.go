package users

import (
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// StudentStatus represents the lifecycle status of a student.
// Set on creation by the parent-enrollment flow; transitions are driven by
// the activate-students scheduler tick (pending→active when enrolled_from
// arrives, active→inactive when enrolled_until passes). The "alumnus" value
// is reserved for future graduation/leaver flows — no scheduler logic
// transitions to it as of PR 2.
type StudentStatus string

const (
	StudentStatusPending  StudentStatus = "pending"
	StudentStatusActive   StudentStatus = "active"
	StudentStatusInactive StudentStatus = "inactive"
	StudentStatusAlumnus  StudentStatus = "alumnus"
)

// Student represents a student in the system
type Student struct {
	base.Model `bun:"schema:users,table:students"`
	base.TenantModel
	PersonID        int64   `bun:"person_id,notnull" json:"person_id"`
	SchoolClass     string  `bun:"school_class,notnull" json:"school_class"`
	GuardianName    *string `bun:"guardian_name" json:"guardian_name,omitempty"`       // Optional: Legacy field, use guardian_profiles instead
	GuardianContact *string `bun:"guardian_contact" json:"guardian_contact,omitempty"` // Optional: Legacy field, use guardian_profiles instead
	GuardianEmail   *string `bun:"guardian_email" json:"guardian_email,omitempty"`
	GuardianPhone   *string `bun:"guardian_phone" json:"guardian_phone,omitempty"`
	GroupID         *int64  `bun:"group_id" json:"group_id,omitempty"`
	ExtraInfo       *string `bun:"extra_info" json:"extra_info,omitempty"`
	SupervisorNotes *string `bun:"supervisor_notes" json:"supervisor_notes,omitempty"`
	HealthInfo      *string `bun:"health_info" json:"health_info,omitempty"`
	PickupStatus    *string `bun:"pickup_status" json:"pickup_status,omitempty"`
	// BusDays is the single source of truth for the Buskind flag (#1582): the
	// weekdays on which the child rides the bus. The legacy boolean bus column
	// was dropped in migration 1.15.116; the API derives a compatibility bus
	// flag from BusDays.HasAny() at the response boundary.
	BusDays       BusDays       `bun:"bus_days,type:jsonb,scanonly" json:"bus_days,omitempty"`
	Sick          *bool         `bun:"sick" json:"sick,omitempty"`                   // true = currently sick
	SickSince     *time.Time    `bun:"sick_since" json:"sick_since,omitempty"`       // When sickness was reported
	Excused       *bool         `bun:"excused" json:"excused,omitempty"`             // true = currently excused (not attending today)
	ExcusedSince  *time.Time    `bun:"excused_since" json:"excused_since,omitempty"` // When excused status was reported
	Status        StudentStatus `bun:"status,notnull,default:'active'" json:"status"`
	EnrolledFrom  *time.Time    `bun:"enrolled_from,type:date" json:"enrolled_from,omitempty"`
	EnrolledUntil *time.Time    `bun:"enrolled_until,type:date" json:"enrolled_until,omitempty"`

	// Photo (optional, gated by operations.student_photos_enabled setting +
	// per-student parental consent recorded in photo_consent_given_at).
	// PhotoPath stores the on-disk path; the JSON-facing photo_url is filled
	// by the service layer mapping the path to a tenant-scoped /uploads URL.
	PhotoPath           *string    `bun:"photo_path" json:"-"`
	PhotoConsentGivenAt *time.Time `bun:"photo_consent_given_at" json:"photo_consent_given_at,omitempty"`
	PhotoConsentGivenBy *int64     `bun:"photo_consent_given_by" json:"photo_consent_given_by,omitempty"`

	// Other consents the parent ticked at enrollment time. Populated
	// by the decision service on approval from request.consent_flags.
	// NULL = no consent recorded (parent either declined or the row
	// predates the consent capture). Timestamp = consent given on
	// that date. Mirrors the photo_consent_given_at shape so the
	// student detail page can render all four uniformly.
	AGBAcceptedAt            *time.Time `bun:"agb_accepted_at" json:"agb_accepted_at,omitempty"`
	DataProcessingAcceptedAt *time.Time `bun:"data_processing_accepted_at" json:"data_processing_accepted_at,omitempty"`
	EmailContactAcceptedAt   *time.Time `bun:"email_contact_accepted_at" json:"email_contact_accepted_at,omitempty"`

	// Relations
	Person *Person `bun:"rel:belongs-to,join:person_id=id" json:"person,omitempty"`
	// Group relation is loaded dynamically to avoid import cycle
}

// BeforeAppendModel sets the correct table expression
// Note: Table aliases (AS "student") are only applied for SELECT, UPDATE, and DELETE queries.
//
//	For INSERT queries, aliases should NOT be used, as they can cause issues with some database drivers.
func (s *Student) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(`users.students AS "student"`)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(`users.students AS "student"`)
	}
	return nil
}

// TableName returns the database table name
func (s *Student) TableName() string {
	return "users.students"
}

// Validate ensures student data is valid
func (s *Student) Validate() error {
	if s.PersonID <= 0 {
		return errors.New("person ID is required")
	}

	if s.SchoolClass == "" {
		return errors.New("school class is required")
	}

	s.SchoolClass = strings.TrimSpace(s.SchoolClass)

	// Normalize optional legacy guardian fields
	trimPtrString(s.GuardianName)
	trimPtrStringOrNil(&s.GuardianContact)

	// Validate optional contact fields
	if err := validatePtrEmail(s.GuardianEmail, "guardian email"); err != nil {
		return err
	}
	if err := validatePtrPhone(s.GuardianPhone, "guardian phone"); err != nil {
		return err
	}

	if err := s.BusDays.Validate(); err != nil {
		return err
	}

	return nil
}

// trimPtrString trims whitespace from a non-nil string pointer
func trimPtrString(s *string) {
	if s != nil && *s != "" {
		*s = strings.TrimSpace(*s)
	}
}

// trimPtrStringOrNil trims whitespace and sets to nil if empty
func trimPtrStringOrNil(sp **string) {
	if *sp == nil || **sp == "" {
		return
	}
	trimmed := strings.TrimSpace(**sp)
	if trimmed == "" {
		*sp = nil
	} else {
		**sp = trimmed
	}
}

// validatePtrEmail validates an optional email pointer
func validatePtrEmail(email *string, fieldName string) error {
	if email == nil || *email == "" {
		return nil
	}
	*email = strings.TrimSpace(*email)
	// Pinned to the shared canonical pattern (email_validation.go) so the
	// rule enforced here at student creation matches enrollment submit-time
	// validation exactly — a value accepted at submit can't be rejected here.
	if !optionalEmailPattern.MatchString(*email) {
		return errors.New("invalid " + fieldName + " format")
	}
	return nil
}

// validatePtrPhone validates an optional phone pointer
func validatePtrPhone(phone *string, fieldName string) error {
	if phone == nil || *phone == "" {
		return nil
	}
	*phone = strings.TrimSpace(*phone)
	// Pinned to the canonical optionalPhonePattern (phone_validation.go)
	// so this student-creation check never diverges from the submit/edit
	// validation in the enrollment service.
	if !optionalPhonePattern.MatchString(*phone) {
		return errors.New("invalid " + fieldName + " format")
	}
	return nil
}

// SetPerson links this student to a person
func (s *Student) SetPerson(person *Person) {
	s.Person = person
	if person != nil {
		s.PersonID = person.ID
	}
}

// GetID returns the entity's ID
func (m *Student) GetID() interface{} {
	return m.ID
}

// GetCreatedAt returns the creation timestamp
func (m *Student) GetCreatedAt() time.Time {
	return m.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (m *Student) GetUpdatedAt() time.Time {
	return m.UpdatedAt
}

// StudentWithGroupInfo represents a student with their group information
type StudentWithGroupInfo struct {
	*Student
	GroupName string `json:"group_name"`
}
