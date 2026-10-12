package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	membershipCompose "github.com/moto-nrw/project-phoenix/modules/schoolmembership/compose"
	"github.com/uptrace/bun"
)

// The People Directory is composed here since its construction left the
// repository composition (#2743): its write path reaches School Membership,
// Care Plan and Identity & Access, and its audit seams reach the Audit
// composition (ADR 0049), which only this composition may name.

// NewPeopleDirectory composes the person and student owner for test graphs
// and CLI roots. The serve root composes the observed instance through
// NewPeopleDirectoryWithPhotosAndObserver.
func NewPeopleDirectory(db *bun.DB) (peopledirectory.Capability, error) {
	capability, _, err := NewPeopleDirectoryWithPhotos(db)
	return capability, err
}

// MustNewPeopleDirectory is NewPeopleDirectory for composition seams that
// cannot return an error; composing only fails on missing dependencies, which
// is a programming error at wiring time.
func MustNewPeopleDirectory(db *bun.DB) peopledirectory.Capability {
	capability, err := NewPeopleDirectory(db)
	if err != nil {
		panic(fmt.Sprintf("compose people directory: %v", err))
	}
	return capability
}

// NewPeopleDirectoryWithPhotos additionally hands back the slot the photo
// lifecycle resolves its runtime from, so a caller that has the feature gate,
// the caller access and the file cleanup fills it after the directory exists.
func NewPeopleDirectoryWithPhotos(db *bun.DB) (peopledirectory.Capability, *peopleCompose.StudentPhotoRuntime, error) {
	module, photoRuntime, err := NewPeopleDirectoryWithPhotosAndObserver(db, func(peopleCompose.Observation) {})
	return module, photoRuntime, err
}

// NewPeopleDirectoryWithPhotosAndObserver composes the owner for a graph that
// records People Directory observations. The Identity & Access membership
// query, the School Membership and Care Plan write owners and the Audit seams
// are bound here.
func NewPeopleDirectoryWithPhotosAndObserver(
	db *bun.DB,
	observe func(peopleCompose.Observation),
) (*peopledirectory.Module, *peopleCompose.StudentPhotoRuntime, error) {
	photoRuntime := new(peopleCompose.StudentPhotoRuntime)
	// Care Plan's companion slice is constructible from the database alone, so
	// the directory's write path gets it here rather than waiting for the full
	// Care Plan — which needs this directory and therefore cannot be built yet.
	companions, err := careplanCompose.NewCompanionRecords(db, func(careplanCompose.Observation) {})
	if err != nil {
		return nil, nil, err
	}
	membership, err := repositories.NewSchoolMembership(db)
	if err != nil {
		return nil, nil, err
	}
	careProfiles, err := careplanCompose.NewStudentProfiles(db, func(careplanCompose.Observation) {})
	if err != nil {
		return nil, nil, err
	}
	audit := peopleDirectoryAudit(db)
	capability, err := peopleCompose.NewWithGuardianMemberships(peopleCompose.Dependencies{
		StudentClassWriteGateQuery: func(ctx context.Context) (*bun.SelectQuery, error) {
			return membershipCompose.StudentClassWriteGateQuery(ctx, db)
		},
		StudentOwners:            NewStudentOwners(membership, careProfiles),
		DB:                       db,
		Observe:                  observe,
		StudentFieldAudit:        studentFieldAuditLog{repo: audit.StudentFieldEdit},
		StudentNoteDeletionAudit: studentNoteDeletionAudit{repo: audit.DataDeletion},
		StudentConsentHistory:    studentConsentHistory{repo: audit.StudentConsentChange},
		StudentPhotoRuntime:      func() peopleCompose.StudentPhotoRuntime { return *photoRuntime },
		StudentCompanions:        NewStudentCompanionSeam(companions),
		GuardianLinkOwners:       newGuardianLinkOwners(db),
	},
		newPeopleIdentityAccess(db, nil).FindActiveGuardianMemberships,
	)
	if err != nil {
		return nil, nil, err
	}
	return capability, photoRuntime, nil
}

// newPeopleIdentityAccess composes the Identity & Access module the People
// Directory repositories read accounts and memberships through. A nil
// observer composes it unobserved, which is what test graphs and CLI roots
// use.
func newPeopleIdentityAccess(db *bun.DB, observe IdentityAccessObserver) *identityaccess.Module {
	module, err := identityCompose.New(identityCompose.Dependencies{DB: db, Observe: identityAccessObservation(observe)})
	if err != nil {
		panic(fmt.Sprintf("compose identity access: %v", err))
	}
	return module
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

// peopleDirectoryAudit builds the Audit Platform persistence behind the People
// Directory's audit seams (note deletion audit, field audit, consent history)
// (ADR 0049). The repositories read and append through the Audit handoff on
// the context, falling back to db.
func peopleDirectoryAudit(db *bun.DB) peopleDirectoryAuditRepositories {
	repositories := auditCompose.MustNewPersistence(auditCompose.PersistenceDependencies{DB: db})
	return peopleDirectoryAuditRepositories{
		DataDeletion: repositories.DataDeletion, StudentFieldEdit: repositories.StudentFieldEdit,
		StudentConsentChange: repositories.StudentConsentChange,
	}
}

type peopleDirectoryAuditRepositories struct {
	DataDeletion         auditModels.DataDeletionRepository
	StudentFieldEdit     auditModels.StudentFieldEditRepository
	StudentConsentChange auditModels.StudentConsentChangeRepository
}

// NewStudentNoteDeletionAudit binds the Audit Platform deletion ledger behind
// the People Directory note command. The append uses the ambient tenant
// transaction, so missing evidence rolls the note's soft deletion back.
func NewStudentNoteDeletionAudit(db *bun.DB) peopleCompose.StudentNoteDeletionAudit {
	return studentNoteDeletionAudit{repo: peopleDirectoryAudit(db).DataDeletion}
}

type studentNoteDeletionAudit struct {
	repo auditModels.DataDeletionRepository
}

func (a studentNoteDeletionAudit) RecordStudentNoteDeletion(
	ctx context.Context, studentID, noteID, actorAccountID int64,
) error {
	record := auditModels.NewDataDeletion(studentID, auditModels.DeletionTypeManual, 1,
		"account:"+strconv.FormatInt(actorAccountID, 10))
	record.DeletionReason = "student note deleted"
	record.SetMetadata("student_note_id", noteID)
	record.SetMetadata("deleted_by_account_id", actorAccountID)
	return a.repo.Create(ctx, record)
}

func (a studentNoteDeletionAudit) RecordLegacyStudentNoteDeletion(
	ctx context.Context, studentID, noteID int64,
) error {
	record := auditModels.NewDataDeletion(studentID, auditModels.DeletionTypeManual, 1, "system")
	record.DeletionReason = "legacy supervisor notes cleared"
	record.SetMetadata("student_note_id", noteID)
	record.SetMetadata("source", "supervisor_notes")
	return a.repo.Create(ctx, record)
}

// NewStudentFieldAuditLog binds the Audit Platform trail behind the People
// Directory change-history seam. Audit Platform owns
// audit.student_field_edits; this is the composition seam that hands the
// directory an append and a read of it.
func NewStudentFieldAuditLog(db *bun.DB) peopleCompose.StudentFieldAuditLog {
	return studentFieldAuditLog{repo: peopleDirectoryAudit(db).StudentFieldEdit}
}

type studentFieldAuditLog struct {
	repo auditModels.StudentFieldEditRepository
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
	membership schoolmembership.Capability
	care       careplan.StudentProfileCommands
}

// NewStudentOwners binds the People Directory's student owners port to School
// Membership and Care Plan.
func NewStudentOwners(membership schoolmembership.Capability, care careplan.StudentProfileCommands) peopleCompose.StudentOwners {
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
		MembershipID: membershipID, SupervisorNotes: record.SupervisorNotes, HealthInfo: record.HealthInfo,
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
func NewStudentCompanionSeam(carePlan careplanCompose.CompanionRecords) peopleCompose.StudentCompanions {
	return careplanCompanions{carePlan: carePlan}
}

func (c careplanCompanions) ListCompanionEdgesForStudent(
	ctx context.Context,
	studentID int64,
) ([]peopleCompose.StudentCompanionEdge, error) {
	edges, err := c.carePlan.ListCompanionEdges(ctx, studentID)
	if err != nil {
		return nil, err
	}
	result := make([]peopleCompose.StudentCompanionEdge, 0, len(edges))
	for _, edge := range edges {
		// Care Plan stores the pair ordered by id; which side is "the student"
		// is the reader's question, and the owner answers it per edge.
		result = append(result, peopleCompose.StudentCompanionEdge{
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

// DeleteCompanionEdges also records that this write touched links, which is the
// only honest basis for announcing student_companions_changed. It belongs here
// rather than in the owner: the signal is carried on the caller's context, and
// this is the moment the links actually change.
func (c careplanCompanions) DeleteCompanionEdges(ctx context.Context, edgeIDs []int64) error {
	if err := c.carePlan.DeleteCompanionEdges(ctx, edgeIDs); err != nil {
		return err
	}
	if len(edgeIDs) > 0 {
		userModels.RecordCompanionChange(ctx)
	}
	return nil
}

// The student-guardian relationship has three owners since Cutover #2756.
// People Directory writes the relationship and runs the unit of work; Care
// Plan's pickup permission and Identity & Access's portal access reach it
// through the consumer-owned ports bound here. Both halves are constructible
// from the database alone, so every graph that builds the relationship store
// also gets its owners.

// newGuardianRelationshipOwners composes the Care Plan and Identity & Access
// commands behind the relationship's unit of work.
func newGuardianRelationshipOwners(db *bun.DB, observeIdentity IdentityAccessObserver) (guardianPickupPort, guardianAccessPort) {
	pickup, err := careplanCompose.NewGuardianPickupPermissions(db, func(careplanCompose.Observation) {})
	if err != nil {
		panic(fmt.Sprintf("compose guardian pickup permissions: %v", err))
	}
	access, err := identityCompose.NewGuardianStudentAccess(db, identityAccessObservation(observeIdentity))
	if err != nil {
		panic(fmt.Sprintf("compose guardian student access: %v", err))
	}
	return guardianPickupPort{owner: pickup}, guardianAccessPort{owner: access}
}

type guardianPickupPort struct {
	owner careplan.GuardianPickupPermissions
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
	owner identityaccess.GuardianStudentAccess
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

func newGuardianLinkOwners(db *bun.DB) guardianLinkOwners {
	pickup, access := newGuardianRelationshipOwners(db, nil)
	return guardianLinkOwners{guardianPickupPort: pickup, guardianAccessPort: access}
}
