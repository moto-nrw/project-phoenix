package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/auditlog/consents"
	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
)

// ImportConfig defines entity-specific import behavior
type ImportConfig[T any] interface {
	// PreloadReferenceData loads all reference data (groups, rooms) into memory cache
	PreloadReferenceData(ctx context.Context) error

	// Validate validates a single row of import data
	Validate(ctx context.Context, row *T) []importModels.ValidationError

	// FindExisting checks if entity already exists (for duplicate detection)
	FindExisting(ctx context.Context, row T) (*int64, error)

	// Create creates a new entity from import data
	Create(ctx context.Context, row T) (int64, error)

	// Update updates an existing entity
	Update(ctx context.Context, id int64, row T) error

	// EntityName returns the entity type name (for logging/errors)
	EntityName() string
}

// BatchValidator can be implemented by import configs that need validation
// across rows, such as duplicate checks within the uploaded file.
type BatchValidator[T any] interface {
	ValidateBatch(ctx context.Context, rows []T) map[int][]importModels.ValidationError
}

// ChildQuotaCounter can be implemented by imports whose rows can raise the
// Kontingentzahl (#3571). The dry run asks it for every row it would create
// (existingID nil) or update, so the preview counts what the owner counts
// instead of the rows it creates: a new child whose care already ended adds
// nobody, an update that resumes a child's care adds one.
type ChildQuotaCounter[T any] interface {
	AddsToChildQuota(ctx context.Context, row T, existingID *int64) (bool, error)
}

// ProcessingOrderer can be implemented by imports that must create rows in a
// deterministic order. The returned indexes always refer to the original
// upload order, so row numbers in validation errors remain accurate.
//
// It is used only for real imports; previews preserve upload order.
type ProcessingOrderer[T any] interface {
	ProcessingOrder(rows []T) []int
}

// StaffImportDeps contains the dependencies for StaffImportConfig.
type StaffImportDeps struct {
	Authorization       importModels.Authorization
	Transactions        importModels.Transactions
	Invitations         importModels.StaffInviter
	FindInvitedPeople   func(context.Context, string) ([]int64, error)
	FindSchoolAccount   func(context.Context, string) (int64, bool, error)
	Roles               importModels.SchoolRoleQuery
	RolePolicy          importModels.SchoolRolePolicy
	FindRolePermissions func(context.Context, int64) ([]string, error)
	SchoolName          func(context.Context) (string, error)

	// Stammdaten owners (#2600, #2708): People Directory files the person,
	// School Membership the staff and caregiver rows, Workforce the master
	// data and qualifications.
	Persons    PersonDirectory
	Membership StaffMembership
	Records    StaffRecords
}

// OpeningBalanceImportDeps contains the request-independent dependencies for
// the opening balance import (#2132). The service dependencies are declared as
// the narrow slices used here rather than the full service interfaces — the
// import consumes four ledger operations, not the absence workflow.
type OpeningBalanceImportDeps struct {
	Transactions         importModels.Transactions
	References           importModels.OpeningReferences
	BalanceAdjustService OpeningBalanceBookings
	StaffAbsenceService  VacationTakeovers
}

// ImportRuntime carries the platform collaborators of an import service:
// the Audit command the GDPR import record is appended through (issue #584:
// audit writes moved out of api/import; #2708: the owner command replaces
// the repository) and the runtime-evidence sink that receives one
// importModels.ImportObservation per run. A nil Observe disables observation; a nil
// Audit makes RecordAuditInTransaction fail. Transactions is required for
// ImportBatches, which owns the transaction lifecycle.
type ImportRuntime struct {
	Transactions importModels.Transactions
	Audit        AuditCommand
	Observe      func(importModels.ImportObservation)
	Fingerprint  func([]byte) string
}

// StudentImportDeps are the consumer-owned ports of the student import.
// Persons, Students and Guardians are the People Directory; Schedules is
// Care Plan; PrivacyConsents is Student Presence; ConsentHistory is the
// Audit platform recorder of the consent timestamps.
type StudentImportDeps struct {
	Transactions    importModels.Transactions
	Persons         PersonDirectory
	Students        StudentDirectory
	Guardians       GuardianDirectory
	Schedules       StudentSchedules
	PrivacyConsents PrivacyConsents
	// FindRFIDCard resolves the optional RFID column to a card of this school
	// (#2600). nil disables RFID import (the column is then rejected).
	FindRFIDCard   func(context.Context, string) (string, bool, error)
	Resolver       RelationshipResolver
	ConsentHistory consents.ConsentTransitions
	// CountsTowardChildQuota is School Membership's Kontingentzahl rule for
	// one membership (#3571); the preview counts the children a row adds
	// with it. Required for a dry run.
	CountsTowardChildQuota func(status, enrolledFrom, enrolledUntil, day string) bool
}

// ClassListImportDeps are the owner ports of the class-list entry import
// (#2382, #2708): School Membership writes the entry, People Directory
// answers whether a student already carries the name, and the Audit
// platform records the change so imported rows leave the same trail as
// manually created ones.
type ClassListImportDeps struct {
	Transactions importModels.Transactions
	Membership   ClassListMembership
	Persons      PersonDirectory
	Students     StudentDirectory
	Audit        AuditCommand
}

// RelationshipResolver preloads and resolves the names in an upload.
type RelationshipResolver interface {
	PreloadReferenceData(context.Context) error
	PreloadRooms(context.Context) error
	ResolveGroup(context.Context, string) (*int64, []importModels.ValidationError)
	ResolveRoom(context.Context, string) (*int64, []importModels.ValidationError)
}

// GuardianPortDecorator binds a recording or failing owner port in tests.
type GuardianPortDecorator func(GuardianDirectory) GuardianDirectory
