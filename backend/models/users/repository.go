package users

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// RFIDCardRepository defines operations for managing RFID cards
type RFIDCardRepository interface {
	// Create inserts a new RFID card into the database
	Create(ctx context.Context, card *RFIDCard) error

	// FindByID retrieves an RFID card by its ID
	FindByID(ctx context.Context, id string) (*RFIDCard, error)

	// Update updates an existing RFID card
	Update(ctx context.Context, card *RFIDCard) error

	// Delete removes an RFID card
	Delete(ctx context.Context, id string) error

	// List retrieves RFID cards matching the filters
	List(ctx context.Context, filters map[string]interface{}) ([]*RFIDCard, error)

	// Activate sets an RFID card as active
	Activate(ctx context.Context, id string) error

	// Deactivate sets an RFID card as inactive
	Deactivate(ctx context.Context, id string) error
}

// PersonRepository defines operations for managing persons
type PersonRepository interface {
	// Create inserts a new person into the database
	Create(ctx context.Context, person *Person) error

	// FindByID retrieves a person by their ID
	FindByID(ctx context.Context, id interface{}) (*Person, error)

	// FindByTagID retrieves a person by their RFID tag ID
	FindByTagID(ctx context.Context, tagID string) (*Person, error)

	// FindByAccountID retrieves a person by their account ID
	FindByAccountID(ctx context.Context, accountID int64) (*Person, error)

	// Update updates an existing person
	Update(ctx context.Context, person *Person) error

	// Delete removes a person
	Delete(ctx context.Context, id interface{}) error

	// List retrieves persons matching the filters
	List(ctx context.Context, filters map[string]interface{}) ([]*Person, error)

	// LinkToAccount associates a person with an account
	LinkToAccount(ctx context.Context, personID int64, accountID int64) error

	// UnlinkFromAccount removes account association from a person
	UnlinkFromAccount(ctx context.Context, personID int64) error

	// LinkToRFIDCard associates a person with an RFID card
	LinkToRFIDCard(ctx context.Context, personID int64, tagID string) error

	// UnlinkFromRFIDCard removes RFID card association from a person
	UnlinkFromRFIDCard(ctx context.Context, personID int64) error

	// FindWithAccount retrieves a person with their associated account
	FindWithAccount(ctx context.Context, id int64) (*Person, error)
}

// StudentRepository defines operations for managing students
type StudentRepository interface {
	// Create inserts a new student into the database
	Create(ctx context.Context, student *Student) error

	// FindByID retrieves a student by their ID
	FindByID(ctx context.Context, id interface{}) (*Student, error)

	// FindByPersonID retrieves a student by their person ID
	FindByPersonID(ctx context.Context, personID int64) (*Student, error)

	// FindByGroupID retrieves students by their group ID
	FindByGroupID(ctx context.Context, groupID int64) ([]*Student, error)

	// FindByGroupIDs retrieves students by multiple group IDs
	FindByGroupIDs(ctx context.Context, groupIDs []int64) ([]*Student, error)

	// FindBySchoolClass retrieves students by their school class
	FindBySchoolClass(ctx context.Context, schoolClass string) ([]*Student, error)

	// Update updates an existing student
	Update(ctx context.Context, student *Student) error

	// Delete removes a student
	Delete(ctx context.Context, id interface{}) error

	// List retrieves students matching the filters
	List(ctx context.Context, filters map[string]interface{}) ([]*Student, error)

	// ListWithOptions retrieves students with query options
	ListWithOptions(ctx context.Context, options *base.QueryOptions) ([]*Student, error)

	// CountWithOptions counts students matching the query options
	CountWithOptions(ctx context.Context, options *base.QueryOptions) (int, error)

	// UpdateLocation updates a student's location status
	UpdateLocation(ctx context.Context, id int64, location string) error

	// AssignToGroup assigns a student to a group
	AssignToGroup(ctx context.Context, studentID int64, groupID int64) error

	// RemoveFromGroup removes a student from their group
	RemoveFromGroup(ctx context.Context, studentID int64) error

	// FindByTeacherID retrieves students supervised by a teacher (through group assignments)
	FindByTeacherID(ctx context.Context, teacherID int64) ([]*Student, error)

	// FindByTeacherIDWithGroups retrieves students with group names supervised by a teacher
	FindByTeacherIDWithGroups(ctx context.Context, teacherID int64) ([]*StudentWithGroupInfo, error)
}

// StaffRepository defines operations for managing staff members
type StaffRepository interface {
	// Create inserts a new staff member into the database
	Create(ctx context.Context, staff *Staff) error

	// FindByID retrieves a staff member by their ID
	FindByID(ctx context.Context, id interface{}) (*Staff, error)

	// FindByPersonID retrieves a staff member by their person ID
	FindByPersonID(ctx context.Context, personID int64) (*Staff, error)

	// Update updates an existing staff member
	Update(ctx context.Context, staff *Staff) error

	// Delete removes a staff member
	Delete(ctx context.Context, id interface{}) error

	// List retrieves staff members matching the filters
	List(ctx context.Context, filters map[string]interface{}) ([]*Staff, error)

	// UpdateNotes updates staff notes
	UpdateNotes(ctx context.Context, id int64, notes string) error

	// FindWithPerson retrieves a staff member with their associated person data
	FindWithPerson(ctx context.Context, id int64) (*Staff, error)
}

// TeacherRepository defines operations for managing teachers
type TeacherRepository interface {
	// Create inserts a new teacher into the database
	Create(ctx context.Context, teacher *Teacher) error

	// FindByID retrieves a teacher by their ID
	FindByID(ctx context.Context, id interface{}) (*Teacher, error)

	// FindByStaffID retrieves a teacher by their staff ID
	FindByStaffID(ctx context.Context, staffID int64) (*Teacher, error)

	// FindBySpecialization retrieves teachers by their specialization
	FindBySpecialization(ctx context.Context, specialization string) ([]*Teacher, error)

	// Update updates an existing teacher
	Update(ctx context.Context, teacher *Teacher) error

	// Delete removes a teacher
	Delete(ctx context.Context, id interface{}) error

	// List retrieves teachers matching the filters
	List(ctx context.Context, filters map[string]interface{}) ([]*Teacher, error)

	// ListWithOptions retrieves teachers matching the query options
	ListWithOptions(ctx context.Context, options *base.QueryOptions) ([]*Teacher, error)

	// FindByGroupID retrieves teachers assigned to a group
	FindByGroupID(ctx context.Context, groupID int64) ([]*Teacher, error)

	// UpdateQualifications updates a teacher's qualifications
	UpdateQualifications(ctx context.Context, id int64, qualifications string) error

	// FindWithStaffAndPerson retrieves a teacher with their associated staff and person data
	FindWithStaffAndPerson(ctx context.Context, id int64) (*Teacher, error)
}

// GuestRepository defines operations for managing guests
type GuestRepository interface {
	// Create inserts a new guest into the database
	Create(ctx context.Context, guest *Guest) error

	// FindByID retrieves a guest by their ID
	FindByID(ctx context.Context, id interface{}) (*Guest, error)

	// FindByStaffID retrieves a guest by their staff ID
	FindByStaffID(ctx context.Context, staffID int64) (*Guest, error)

	// FindByOrganization retrieves guests by their organization
	FindByOrganization(ctx context.Context, organization string) ([]*Guest, error)

	// FindByExpertise retrieves guests by their activity expertise
	FindByExpertise(ctx context.Context, expertise string) ([]*Guest, error)

	// Update updates an existing guest
	Update(ctx context.Context, guest *Guest) error

	// Delete removes a guest
	Delete(ctx context.Context, id interface{}) error

	// List retrieves guests matching the filters
	List(ctx context.Context, filters map[string]interface{}) ([]*Guest, error)

	// FindActive retrieves currently active guests
	FindActive(ctx context.Context) ([]*Guest, error)

	// SetDateRange sets a guest's start and end dates
	SetDateRange(ctx context.Context, id int64, startDate, endDate time.Time) error
}

// ProfileRepository defines operations for managing profiles
type ProfileRepository interface {
	// Create inserts a new profile into the database
	Create(ctx context.Context, profile *Profile) error

	// FindByID retrieves a profile by its ID
	FindByID(ctx context.Context, id interface{}) (*Profile, error)

	// FindByAccountID retrieves a profile by account ID
	FindByAccountID(ctx context.Context, accountID int64) (*Profile, error)

	// Update updates an existing profile
	Update(ctx context.Context, profile *Profile) error

	// Delete removes a profile
	Delete(ctx context.Context, id interface{}) error

	// List retrieves profiles matching the filters
	List(ctx context.Context, filters map[string]interface{}) ([]*Profile, error)

	// UpdateAvatar updates a profile's avatar
	UpdateAvatar(ctx context.Context, id int64, avatar string) error

	// UpdateBio updates a profile's bio
	UpdateBio(ctx context.Context, id int64, bio string) error

	// UpdateSettings updates a profile's settings
	UpdateSettings(ctx context.Context, id int64, settings string) error
}

// PersonGuardianRepository defines operations for managing person-guardian relationships
type PersonGuardianRepository interface {
	// Create inserts a new person-guardian relationship into the database
	Create(ctx context.Context, relationship *PersonGuardian) error

	// FindByID retrieves a relationship by its ID
	FindByID(ctx context.Context, id interface{}) (*PersonGuardian, error)

	// FindByPersonID retrieves relationships by person ID
	FindByPersonID(ctx context.Context, personID int64) ([]*PersonGuardian, error)

	// FindByGuardianID retrieves relationships by guardian account ID
	FindByGuardianID(ctx context.Context, guardianID int64) ([]*PersonGuardian, error)

	// FindPrimaryByPersonID retrieves the primary guardian for a person
	FindPrimaryByPersonID(ctx context.Context, personID int64) (*PersonGuardian, error)

	// FindByRelationshipType retrieves relationships by relationship type
	FindByRelationshipType(ctx context.Context, personID int64, relationshipType RelationshipType) ([]*PersonGuardian, error)

	// Update updates an existing relationship
	Update(ctx context.Context, relationship *PersonGuardian) error

	// Delete removes a relationship
	Delete(ctx context.Context, id interface{}) error

	// List retrieves relationships matching the filters
	List(ctx context.Context, filters map[string]interface{}) ([]*PersonGuardian, error)

	// SetPrimary sets a guardian as the primary guardian for a person
	SetPrimary(ctx context.Context, id int64, isPrimary bool) error

	// UpdatePermissions updates a guardian's permissions
	UpdatePermissions(ctx context.Context, id int64, permissions string) error
}

// GuardianRepository defines operations for managing guardians
type GuardianRepository interface {
	// Create inserts a new guardian into the database
	Create(ctx context.Context, guardian *Guardian) error

	// FindByID retrieves a guardian by their ID
	FindByID(ctx context.Context, id interface{}) (*Guardian, error)

	// FindByEmail retrieves a guardian by their email
	FindByEmail(ctx context.Context, email string) (*Guardian, error)

	// FindByPhone retrieves a guardian by their phone number
	FindByPhone(ctx context.Context, phone string) (*Guardian, error)

	// FindByAccountID retrieves a guardian by their account ID
	FindByAccountID(ctx context.Context, accountID int64) (*Guardian, error)

	// FindByStudentID retrieves all guardians for a student
	FindByStudentID(ctx context.Context, studentID int64) ([]*Guardian, error)

	// Update updates an existing guardian
	Update(ctx context.Context, guardian *Guardian) error

	// Delete removes a guardian
	Delete(ctx context.Context, id interface{}) error

	// List retrieves guardians matching the filters
	List(ctx context.Context, filters map[string]interface{}) ([]*Guardian, error)

	// ListWithOptions retrieves guardians with query options
	ListWithOptions(ctx context.Context, options *base.QueryOptions) ([]*Guardian, error)

	// CountWithOptions counts guardians matching the query options
	CountWithOptions(ctx context.Context, options *base.QueryOptions) (int, error)

	// Search searches for guardians by name, email, or phone
	Search(ctx context.Context, query string, limit int) ([]*Guardian, error)

	// LinkToAccount associates a guardian with an account
	LinkToAccount(ctx context.Context, guardianID int64, accountID int64) error

	// UnlinkFromAccount removes account association from a guardian
	UnlinkFromAccount(ctx context.Context, guardianID int64) error

	// FindActive retrieves all active guardians
	FindActive(ctx context.Context) ([]*Guardian, error)
}

// StudentGuardianRepository defines operations for managing student-guardian relationships
type StudentGuardianRepository interface {
	// Create inserts a new student-guardian relationship into the database
	Create(ctx context.Context, relationship *StudentGuardian) error

	// FindByID retrieves a relationship by its ID
	FindByID(ctx context.Context, id interface{}) (*StudentGuardian, error)

	// FindByStudentID retrieves relationships by student ID
	FindByStudentID(ctx context.Context, studentID int64) ([]*StudentGuardian, error)

	// FindByGuardianID retrieves relationships by guardian account ID
	FindByGuardianID(ctx context.Context, guardianID int64) ([]*StudentGuardian, error)

	// FindPrimaryByStudentID retrieves the primary guardian for a student
	FindPrimaryByStudentID(ctx context.Context, studentID int64) (*StudentGuardian, error)

	// FindEmergencyContactsByStudentID retrieves all emergency contacts for a student
	FindEmergencyContactsByStudentID(ctx context.Context, studentID int64) ([]*StudentGuardian, error)

	// FindPickupAuthoritiesByStudentID retrieves all guardians who can pickup a student
	FindPickupAuthoritiesByStudentID(ctx context.Context, studentID int64) ([]*StudentGuardian, error)

	// FindByRelationshipType retrieves relationships by relationship type
	FindByRelationshipType(ctx context.Context, studentID int64, relationshipType string) ([]*StudentGuardian, error)

	// Update updates an existing relationship
	Update(ctx context.Context, relationship *StudentGuardian) error

	// Delete removes a relationship
	Delete(ctx context.Context, id interface{}) error

	// List retrieves relationships matching the filters
	List(ctx context.Context, filters map[string]interface{}) ([]*StudentGuardian, error)

	// SetPrimary sets a guardian as the primary guardian for a student
	SetPrimary(ctx context.Context, id int64, isPrimary bool) error

	// SetEmergencyContact sets whether a guardian is an emergency contact
	SetEmergencyContact(ctx context.Context, id int64, isEmergencyContact bool) error

	// SetCanPickup sets whether a guardian can pickup a student
	SetCanPickup(ctx context.Context, id int64, canPickup bool) error

	// UpdatePermissions updates a guardian's permissions
	UpdatePermissions(ctx context.Context, id int64, permissions string) error
}

// PrivacyConsentRepository defines operations for managing privacy consents
type PrivacyConsentRepository interface {
	// Create inserts a new privacy consent into the database
	Create(ctx context.Context, consent *PrivacyConsent) error

	// FindByID retrieves a privacy consent by its ID
	FindByID(ctx context.Context, id interface{}) (*PrivacyConsent, error)

	// FindByStudentID retrieves privacy consents for a student
	FindByStudentID(ctx context.Context, studentID int64) ([]*PrivacyConsent, error)

	// FindByStudentIDAndPolicyVersion retrieves a privacy consent for a student and policy version
	FindByStudentIDAndPolicyVersion(ctx context.Context, studentID int64, policyVersion string) (*PrivacyConsent, error)

	// FindActiveByStudentID retrieves active privacy consents for a student
	FindActiveByStudentID(ctx context.Context, studentID int64) ([]*PrivacyConsent, error)

	// FindExpired retrieves all expired privacy consents
	FindExpired(ctx context.Context) ([]*PrivacyConsent, error)

	// FindNeedingRenewal retrieves all privacy consents that need renewal
	FindNeedingRenewal(ctx context.Context) ([]*PrivacyConsent, error)

	// Update updates an existing privacy consent
	Update(ctx context.Context, consent *PrivacyConsent) error

	// Delete removes a privacy consent
	Delete(ctx context.Context, id interface{}) error

	// List retrieves privacy consents matching the filters
	List(ctx context.Context, filters map[string]interface{}) ([]*PrivacyConsent, error)

	// Accept marks a privacy consent as accepted
	Accept(ctx context.Context, id int64, acceptedAt time.Time) error

	// Revoke revokes a privacy consent
	Revoke(ctx context.Context, id int64) error

	// SetExpiryDate sets the expiry date for a privacy consent
	SetExpiryDate(ctx context.Context, id int64, expiresAt time.Time) error

	// SetRenewalRequired sets whether renewal is required for a privacy consent
	SetRenewalRequired(ctx context.Context, id int64, renewalRequired bool) error

	// UpdateDetails updates the details for a privacy consent
	UpdateDetails(ctx context.Context, id int64, details string) error
}
