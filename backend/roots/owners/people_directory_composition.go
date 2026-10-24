package owners

import (
	"context"
	"encoding/json"

	studentDirectoryView "github.com/moto-nrw/project-phoenix/modules/studentdirectoryview/compose"

	guardianLinkView "github.com/moto-nrw/project-phoenix/modules/guardianlinkview/compose"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

// ComposePeopleDirectory composes the person and student owner over the
// other owners' halves its write path and audit seams reach: School
// Membership's enrollment, Care Plan's care profiles and companion links, the
// Audit Platform's trails, the guardian link owners and Identity & Access's
// guardian memberships. directory carries what the caller binds from the
// database (the database, the observer and the class write gate); the owner
// seams and the photo runtime slot are bound here.
func ComposePeopleDirectory(
	directory peopleCompose.Dependencies, membership schoolmembership.Capability, careProfiles careplan.StudentProfileCommands,
	companions careplanCompose.CompanionRecords, audit peopleDirectoryAuditTrails, guardianLinks guardianLinkOwners,
	guardianMemberships peopleCompose.GuardianMembershipQuery,
) (*peopleCompose.Directory, error) {
	directory.StudentOwners = NewStudentOwners(membership, careProfiles)
	directory.StudentFieldAudit = studentFieldAuditLog{repo: audit.StudentFieldEdit}
	directory.StudentNoteDeletionAudit = studentNoteDeletionAudit{repo: audit.DataDeletion}
	directory.StudentConsentHistory = studentConsentHistory{repo: audit.StudentConsentChange}
	directory.StudentCompanions = NewStudentCompanionSeam(companions)
	directory.GuardianLinkOwners = guardianLinks
	directory.GuardianRolePresets = GuardianRolePresets
	directory.GuardianLinkRows = guardianLinkView.NewReader()
	directory.StudentDirectory = studentDirectoryView.NewReader()
	return peopleCompose.New(directory, guardianMemberships)
}

func identityAccessObservation(observe IdentityAccessObserver) func(identityCompose.Observation) {
	if observe == nil {
		return func(identityCompose.Observation) {}
	}
	return func(observation identityCompose.Observation) {
		observe(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows,
			observation.Stats.StatementDuration, identityaccess.ErrorCode(observation.Err), observation.Err)
	}
}

// peopleDirectoryAuditTrails are the Audit Platform trails behind the People
// Directory's audit seams: the deletion log, the per-child change history and
// the consent history.
type peopleDirectoryAuditTrails struct {
	DataDeletion         DataDeletionAppender
	StudentFieldEdit     studentFieldEditTrail
	StudentConsentChange interface {
		ListByStudentID(ctx context.Context, studentID int64) ([]*auditModels.StudentConsentChange, error)
	}
}

// studentFieldEditTrail appends and lists the per-child change history.
type studentFieldEditTrail interface {
	CreateBatch(ctx context.Context, edits []*auditModels.StudentFieldEdit) error
	GetByStudentID(ctx context.Context, studentID int64) ([]*auditModels.StudentFieldEdit, error)
}

type studentNoteDeletionAudit struct {
	repo DataDeletionAppender
}

func (a studentNoteDeletionAudit) RecordStudentNoteDeletion(
	ctx context.Context, studentID, noteID, actorAccountID int64,
) error {
	return a.repo.Create(ctx, auditModels.NewStudentNoteDeletion(studentID, noteID, actorAccountID))
}

type studentFieldAuditLog struct {
	repo studentFieldEditTrail
}

func (l studentFieldAuditLog) Append(ctx context.Context, edits []peopledirectory.StudentFieldEdit) error {
	rows := make([]*auditModels.StudentFieldEdit, 0, len(edits))
	for _, edit := range edits {
		rows = append(rows, &auditModels.StudentFieldEdit{
			StudentID: edit.StudentID, EditedBy: edit.EditedBy, EditedByName: edit.EditedByName,
			FieldName: edit.FieldName, OldValue: edit.OldValue, NewValue: edit.NewValue,
		})
	}
	return l.repo.CreateBatch(ctx, rows)
}

func (l studentFieldAuditLog) ListByStudent(ctx context.Context, studentID int64) ([]peopledirectory.StudentFieldEdit, error) {
	rows, err := l.repo.GetByStudentID(ctx, studentID)
	if err != nil {
		return nil, err
	}
	result := make([]peopledirectory.StudentFieldEdit, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		result = append(result, peopledirectory.StudentFieldEdit{
			ID: row.ID, StudentID: row.StudentID, EditedBy: row.EditedBy, EditedByName: row.EditedByName,
			FieldName: row.FieldName, OldValue: row.OldValue, NewValue: row.NewValue, CreatedAt: row.CreatedAt,
		})
	}
	return result, nil
}

// studentOwners serves the People Directory's student write path with the
// School Membership enrollment and the Care Plan care profile.
type studentOwners struct {
	membership studentEnrollments
	care       studentCareProfiles
}

// studentEnrollments is the School Membership enrollment the student write
// path locks, enrolls and renews through.
type studentEnrollments interface {
	LockStudentClassWrites(ctx context.Context, exclusive bool) error
	Enroll(ctx context.Context, input schoolmembership.StudentEnrollment) (int64, error)
	RenewEnrollment(ctx context.Context, input schoolmembership.StudentEnrollment) (int64, error)
}

// studentCareProfiles is the Care Plan care profile the student write path
// saves.
type studentCareProfiles interface {
	SaveStudentCareProfile(context.Context, careplan.StudentCareProfile, *careplan.StudentDeparturePlan) error
}

// NewStudentOwners binds the People Directory's student owners port to School
// Membership and Care Plan.
func NewStudentOwners(membership schoolmembership.Capability, care careplan.StudentProfileCommands) peoplePorts.StudentOwners {
	return studentOwners{membership: membership, care: care}
}

func (s studentOwners) LockClassWrites(ctx context.Context, exclusive bool) error {
	return s.membership.LockStudentClassWrites(ctx, exclusive)
}

func enrollmentOf(record peopledirectory.StudentRecord) schoolmembership.StudentEnrollment {
	return schoolmembership.StudentEnrollment{
		StudentID: record.ID, SchoolClass: record.SchoolClass, Status: record.Status,
		GroupID:      record.GroupID,
		EnrolledFrom: record.EnrolledFrom, EnrolledUntil: record.EnrolledUntil,
	}
}

func (s studentOwners) Enroll(ctx context.Context, record peopledirectory.StudentRecord) (int64, error) {
	return s.membership.Enroll(ctx, enrollmentOf(record))
}

func (s studentOwners) Renew(ctx context.Context, record peopledirectory.StudentRecord) (int64, error) {
	id, err := s.membership.RenewEnrollment(ctx, enrollmentOf(record))
	if err != nil {
		return 0, err
	}
	if id == 0 {
		return 0, peopledirectory.ErrStudentNotFound
	}
	return id, err
}

func (s studentOwners) SaveCare(ctx context.Context, membershipID int64, record peopledirectory.StudentRecord, plan peopledirectory.StudentPlan, note *string, touched bool) error {
	modes := make(map[string][]string, len(plan.AllowedDepartureModes))
	for day, values := range plan.AllowedDepartureModes {
		for _, value := range values {
			modes[day] = append(modes[day], string(value))
		}
	}
	days := make(map[string]string, len(plan.DepartureDays))
	for day, value := range plan.DepartureDays {
		days[day] = string(value)
	}
	return s.care.SaveStudentCareProfile(ctx, careplan.StudentCareProfile{
		MembershipID: membershipID, HealthInfo: record.HealthInfo,
		PickupStatus: record.PickupStatus, Sick: record.Sick != nil && *record.Sick,
		SickSince: record.SickSince, Excused: record.Excused != nil && *record.Excused, ExcusedSince: record.ExcusedSince,
	}, &careplan.StudentDeparturePlan{
		MembershipID: membershipID, PickupStatus: plan.AllowedDepartureModes.LegacyPickupStatus(),
		AllowedDepartureModes: modes, DepartureDays: days,
		BusDays: plan.BusDays, PickupDays: plan.PickupDays, CompanionNote: note, PlanTouched: touched,
	})
}

// careplanCompanions binds Care Plan's companion contract behind the People
// Directory write path's seam. Care Plan owns users.student_companions; the
// directory only asks which links a narrowed plan can no longer justify.
type careplanCompanions struct {
	carePlan careplanCompose.CompanionRecords
}

// NewStudentCompanionSeam binds Care Plan's companion slice behind People
// Directory's write path.
func NewStudentCompanionSeam(carePlan careplanCompose.CompanionRecords) peoplePorts.StudentCompanions {
	return careplanCompanions{carePlan: carePlan}
}

func (c careplanCompanions) ListCompanionEdgesForStudent(
	ctx context.Context,
	studentID int64,
) ([]peoplePorts.StudentCompanionEdge, error) {
	edges, err := c.carePlan.ListCompanionEdges(ctx, studentID)
	if err != nil {
		return nil, err
	}
	result := make([]peoplePorts.StudentCompanionEdge, 0, len(edges))
	for _, edge := range edges {
		// Care Plan stores the pair ordered by id; which side is "the student"
		// is the reader's question, and the owner answers it per edge.
		result = append(result, peoplePorts.StudentCompanionEdge{
			ID: edge.ID, StudentID: edge.StudentLowID,
			CompanionStudentID: edge.StudentHighID, Weekday: edge.Weekday,
		})
	}
	return result, nil
}

func (c careplanCompanions) CompanionDaysCoveredExcluding(
	ctx context.Context,
	studentIDs []int64,
	excludeID int64,
) (map[int64]map[string]bool, error) {
	return c.carePlan.CompanionDaysCoveredExcluding(ctx, studentIDs, excludeID)
}

func (c careplanCompanions) DeleteCompanionEdges(ctx context.Context, edgeIDs []int64) error {
	return c.carePlan.DeleteCompanionEdges(ctx, edgeIDs)
}

// newGuardianRelationshipPorts binds the Care Plan and Identity & Access
// commands behind the relationship's unit of work.
func newGuardianRelationshipPorts(pickup careplan.GuardianPickupPermissions, access identityaccess.GuardianStudentAccess) (guardianPickupPort, guardianAccessPort) {
	return guardianPickupPort{owner: pickup}, guardianAccessPort{owner: access}
}

type guardianPickupPort struct {
	owner guardianPickupPermissions
}

// guardianPickupPermissions is Care Plan's pickup permission command the
// relationship's unit of work writes.
type guardianPickupPermissions interface {
	CreateGuardianPickupPermission(context.Context, careplan.GuardianPickupPermission) error
	ChangeGuardianPickupPermission(context.Context, careplan.GuardianPickupPermissionChange) (bool, error)
}

func (p guardianPickupPort) CreateGuardianPickupPermission(ctx context.Context, tenantID, relationshipID int64, canPickup bool, notes *string) error {
	return p.owner.CreateGuardianPickupPermission(ctx, careplan.GuardianPickupPermission{
		TenantID: tenantID, RelationshipID: relationshipID, CanPickup: canPickup, PickupNotes: notes,
	})
}

func (p guardianPickupPort) ChangeGuardianPickupPermission(ctx context.Context, tenantID, relationshipID int64, canPickup *bool, setNotes bool, notes *string) (bool, error) {
	return p.owner.ChangeGuardianPickupPermission(ctx, careplan.GuardianPickupPermissionChange{
		TenantID: tenantID, RelationshipID: relationshipID, CanPickup: canPickup, SetPickupNotes: setNotes, PickupNotes: notes,
	})
}

type guardianAccessPort struct {
	owner guardianStudentAccess
}

// guardianStudentAccess is Identity & Access's relationship access command
// the relationship's unit of work writes.
type guardianStudentAccess interface {
	LockLiveGuardianAccount(context.Context, int64) (bool, error)
	GrantGuardianStudentAccess(context.Context, identityaccess.GuardianStudentAccessGrant) error
	SetGuardianStudentPermissions(ctx context.Context, tenantID, relationshipID int64, permissions json.RawMessage) (bool, error)
}

func (p guardianAccessPort) LockLiveGuardianAccount(ctx context.Context, accountID int64) (bool, error) {
	return p.owner.LockLiveGuardianAccount(ctx, accountID)
}

func (p guardianAccessPort) GrantGuardianStudentAccess(ctx context.Context, tenantID, relationshipID int64, accountID *int64, permissions json.RawMessage) error {
	return p.owner.GrantGuardianStudentAccess(ctx, identityaccess.GuardianStudentAccessGrant{
		TenantID: tenantID, RelationshipID: relationshipID, AccountID: accountID, Permissions: permissions,
	})
}

func (p guardianAccessPort) SetGuardianStudentPermissions(ctx context.Context, tenantID, relationshipID int64, permissions json.RawMessage) (bool, error) {
	return p.owner.SetGuardianStudentPermissions(ctx, tenantID, relationshipID, permissions)
}

// guardianLinkOwners serves People Directory's parents-portal link writes the
// same two owner halves the relationship store writes.
type guardianLinkOwners struct {
	guardianPickupPort
	guardianAccessPort
}
