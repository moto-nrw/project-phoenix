package repositories

import (
	"context"
	"sort"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	facilityModels "github.com/moto-nrw/project-phoenix/models/facilities"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	educationRepo "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	workforceLegacy "github.com/moto-nrw/project-phoenix/modules/workforce/legacy"
)

// The ports of the retained School Structure services the legacy composition
// binds over the retained repositories (#2742). Each adapter translates the
// People Directory, Facilities, Workforce and Audit Platform rows into the
// School Structure values the services own (#3556), and changes nothing about
// which reads and writes run.

// EducationRooms serves the group service's room directory from the
// Facilities rooms.
type EducationRooms struct{ rooms facilityModels.RoomRepository }

// NewEducationRooms binds the room directory to the room repository.
func NewEducationRooms(rooms facilityModels.RoomRepository) EducationRooms {
	return EducationRooms{rooms: rooms}
}

// FindRoom returns the room a group may be assigned to.
func (r EducationRooms) FindRoom(ctx context.Context, id int64) (*educationRepo.RoomProjection, error) {
	room, err := r.rooms.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &educationRepo.RoomProjection{
		ID: room.ID, CreatedAt: room.CreatedAt, UpdatedAt: room.UpdatedAt,
		Name: room.Name, Building: room.Building, Floor: room.Floor,
		Capacity: room.Capacity, Category: room.Category, Color: room.Color,
	}, nil
}

// EducationStaff serves the class assignments' staff check from the retained
// staff repository.
type EducationStaff struct{ staff userModels.StaffRepository }

// NewEducationStaff binds the staff check to the staff repository.
func NewEducationStaff(staff userModels.StaffRepository) EducationStaff {
	return EducationStaff{staff: staff}
}

// StaffExists reports a missing staff member as (false, nil) and keeps every
// other failure.
func (s EducationStaff) StaffExists(ctx context.Context, id int64) (bool, error) {
	if _, err := s.staff.FindByID(ctx, id); err != nil {
		if usersRepo.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// LockStaff locks the staff member's row until the transaction ends.
func (s EducationStaff) LockStaff(ctx context.Context, id int64) error {
	_, err := s.staff.FindByIDForUpdate(ctx, id)
	return err
}

// EducationExternalCaregivers serves the running-supervision directory from
// the staff and guest repositories.
type EducationExternalCaregivers struct {
	staff  userModels.StaffRepository
	guests userModels.GuestRepository
}

// NewEducationExternalCaregivers binds the external-caregiver directory to
// the staff and guest repositories.
func NewEducationExternalCaregivers(staff userModels.StaffRepository, guests userModels.GuestRepository) EducationExternalCaregivers {
	return EducationExternalCaregivers{staff: staff, guests: guests}
}

// ListExternalCaregivers returns the currently valid external caregivers
// (#3823): staff members recorded with an active guest profile and without a
// moto account, ordered like the caregiver directory. They may help with a
// running supervision but never take over a group, so the substitution module
// keeps them out of the handover targets. Other staff without an account (an
// import without an e-mail address) stay out: nobody vouched for them as
// caregivers.
func (s EducationExternalCaregivers) ListExternalCaregivers(ctx context.Context) ([]*educationRepo.Caregiver, error) {
	guests, err := s.guests.List(ctx, map[string]any{"active": true})
	if err != nil {
		return nil, err
	}
	activeStaffIDs := make(map[int64]struct{}, len(guests))
	for _, guest := range guests {
		if guest != nil {
			activeStaffIDs[guest.StaffID] = struct{}{}
		}
	}
	members, err := s.staff.ListAllWithPerson(ctx)
	if err != nil {
		return nil, err
	}
	externals := make([]*userModels.Staff, 0, len(members))
	for _, member := range members {
		if member == nil || !member.IsGuest || member.Person == nil || member.Person.AccountID != nil {
			continue
		}
		if _, active := activeStaffIDs[member.ID]; active {
			externals = append(externals, member)
		}
	}
	// Same German dictionary order as the caregiver directory, so both
	// halves of the picker sort alike; one collator per call.
	collator := collate.New(language.German, collate.IgnoreCase)
	sort.SliceStable(externals, func(i, j int) bool {
		a, b := externals[i].Person, externals[j].Person
		if c := collator.CompareString(a.FirstName, b.FirstName); c != 0 {
			return c < 0
		}
		if c := collator.CompareString(a.LastName, b.LastName); c != 0 {
			return c < 0
		}
		return externals[i].ID < externals[j].ID
	})
	result := make([]*educationRepo.Caregiver, 0, len(externals))
	for _, member := range externals {
		result = append(result, &educationRepo.Caregiver{StaffID: member.ID, FullName: member.Person.FirstName + " " + member.Person.LastName})
	}
	return result, nil
}

// EducationCaregivers serves the substitution module's caregiver directory
// from the retained teacher repository.
type EducationCaregivers struct{ teachers userModels.TeacherRepository }

// NewEducationCaregivers binds the caregiver directory to the teacher
// repository.
func NewEducationCaregivers(teachers userModels.TeacherRepository) EducationCaregivers {
	return EducationCaregivers{teachers: teachers}
}

// FindActiveCaregiverByAccountID returns the active caregiver bound to the
// account, nil when the account is none.
func (c EducationCaregivers) FindActiveCaregiverByAccountID(ctx context.Context, accountID int64) (*educationRepo.Caregiver, error) {
	caregiver, err := c.teachers.FindActiveCaregiverByAccountID(ctx, accountID)
	if err != nil || caregiver == nil {
		return nil, err
	}
	return educationCaregiver(caregiver), nil
}

// ListActiveCaregivers returns every active caregiver of the school, ordered
// by name.
func (c EducationCaregivers) ListActiveCaregivers(ctx context.Context) ([]*educationRepo.Caregiver, error) {
	caregivers, err := c.teachers.ListActiveCaregivers(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*educationRepo.Caregiver, 0, len(caregivers))
	for _, caregiver := range caregivers {
		if caregiver != nil {
			result = append(result, educationCaregiver(caregiver))
		}
	}
	return result, nil
}

func educationCaregiver(caregiver *userModels.ActiveCaregiver) *educationRepo.Caregiver {
	return &educationRepo.Caregiver{StaffID: caregiver.StaffID, TeacherID: caregiver.TeacherID, FullName: caregiver.FullName()}
}

// EducationHandovers serves the substitution module's handover store and the
// group service's handover reader from the retained group substitution
// repository. Workforce owns the rows; the binding translates them into the
// School Structure values (#3556).
type EducationHandovers struct {
	substitutions GroupSubstitutionRepository
}

// NewEducationHandovers binds the handover store to the group substitution
// repository.
func NewEducationHandovers(substitutions GroupSubstitutionRepository) EducationHandovers {
	return EducationHandovers{substitutions: substitutions}
}

// Create stores a handover and reports educationRepo.ErrHandoverExists for
// a duplicate.
func (h EducationHandovers) Create(ctx context.Context, handover *educationRepo.GroupSubstitution) error {
	row := workforceSubstitution(handover)
	if err := h.substitutions.Create(ctx, row); err != nil {
		if userModels.IsUniqueViolation(err) {
			return educationRepo.ErrHandoverExists
		}
		return err
	}
	stored := educationHandover(row)
	stored.Group, stored.RegularStaff, stored.SubstituteStaff = handover.Group, handover.RegularStaff, handover.SubstituteStaff
	*handover = *stored
	return nil
}

// Delete removes a handover.
func (h EducationHandovers) Delete(ctx context.Context, id any) error {
	return h.substitutions.Delete(ctx, id)
}

// FindByID reads one handover.
func (h EducationHandovers) FindByID(ctx context.Context, id any) (*educationRepo.GroupSubstitution, error) {
	return educationHandoverResult(h.substitutions.FindByID(ctx, id))
}

// FindByIDForUpdate reads one handover under a row lock.
func (h EducationHandovers) FindByIDForUpdate(ctx context.Context, id any) (*educationRepo.GroupSubstitution, error) {
	return educationHandoverResult(h.substitutions.FindByIDForUpdate(ctx, id))
}

// FindByGroup lists the substitutions of one group, the group service's
// deletion guard.
func (h EducationHandovers) FindByGroup(ctx context.Context, groupID int64) ([]*educationRepo.GroupSubstitution, error) {
	return educationHandovers(h.substitutions.FindByGroup(ctx, groupID))
}

// ListHandovers selects the handovers of one school.
func (h EducationHandovers) ListHandovers(ctx context.Context, query educationRepo.HandoverQuery) ([]*educationRepo.GroupSubstitution, error) {
	return educationHandovers(h.substitutions.ListWithOptions(ctx, handoverQueryOptions(query)))
}

// ListHandoversWithRelations is ListHandovers with the group and the staff
// members attached.
func (h EducationHandovers) ListHandoversWithRelations(ctx context.Context, query educationRepo.HandoverQuery) ([]*educationRepo.GroupSubstitution, error) {
	return h.substitutions.ListWithRelations(ctx, handoverQueryOptions(query))
}

func educationHandoverResult(row *workforceLegacy.GroupSubstitution, err error) (*educationRepo.GroupSubstitution, error) {
	if err != nil {
		return nil, err
	}
	return educationHandover(row), nil
}

func educationHandovers(rows []*workforceLegacy.GroupSubstitution, err error) ([]*educationRepo.GroupSubstitution, error) {
	if err != nil {
		return nil, err
	}
	result := make([]*educationRepo.GroupSubstitution, 0, len(rows))
	for _, row := range rows {
		result = append(result, educationHandover(row))
	}
	return result, nil
}

func educationHandover(row *workforceLegacy.GroupSubstitution) *educationRepo.GroupSubstitution {
	if row == nil {
		return nil
	}
	return &educationRepo.GroupSubstitution{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		TargetType: row.TargetType, GroupID: row.GroupID, RegularStaffID: row.RegularStaffID,
		SubstituteStaffID: row.SubstituteStaffID, StartDate: row.StartDate, EndDate: row.EndDate, Reason: row.Reason,
	}
}

func workforceSubstitution(handover *educationRepo.GroupSubstitution) *workforceLegacy.GroupSubstitution {
	if handover == nil {
		return nil
	}
	return &workforceLegacy.GroupSubstitution{
		ID: handover.ID, TenantID: handover.TenantID, CreatedAt: handover.CreatedAt, UpdatedAt: handover.UpdatedAt,
		TargetType: handover.TargetType, GroupID: handover.GroupID, RegularStaffID: handover.RegularStaffID,
		SubstituteStaffID: handover.SubstituteStaffID, StartDate: handover.StartDate, EndDate: handover.EndDate, Reason: handover.Reason,
	}
}

func handoverQueryOptions(query educationRepo.HandoverQuery) *userModels.QueryOptions {
	filter := userModels.NewQueryFilter().Equal("tenant_id", query.TenantID)
	if query.TargetType != "" {
		filter.Equal("target_type", query.TargetType)
	}
	if query.GroupID > 0 {
		filter.Equal("group_id", query.GroupID)
	}
	if query.GroupIDs != nil {
		values := make([]any, len(query.GroupIDs))
		for i, id := range query.GroupIDs {
			values[i] = id
		}
		filter.In("group_id", values...)
	}
	if query.SubstituteStaffID > 0 {
		filter.Equal("substitute_staff_id", query.SubstituteStaffID)
	}
	if query.StartsOnOrBefore != nil {
		filter.LessThanOrEqual("start_date", *query.StartsOnOrBefore)
	}
	if query.EndsOnOrAfter != nil {
		filter.GreaterThanOrEqual("end_date", *query.EndsOnOrAfter)
	}
	options := userModels.NewQueryOptions()
	options.Filter = filter
	return options
}

// The substitution module may not name the Audit Platform, so School Structure
// mirrors the actions the trail stores, and the binding below passes them
// through unchanged. A drift must not compile: a false comparison repeats the
// false key of this map literal.
var _ = map[bool]struct{}{
	false: {},
	auditModels.SubstitutionAssigned == educationRepo.SubstitutionAssigned &&
		auditModels.SubstitutionEnded == educationRepo.SubstitutionEnded: {},
}

// EducationSubstitutionAudit appends the substitution module's changes to the
// audit.substitution_changes trail.
type EducationSubstitutionAudit struct {
	changes auditModels.SubstitutionChangeCreator
}

// NewEducationSubstitutionAudit binds the trail to its repository.
func NewEducationSubstitutionAudit(changes auditModels.SubstitutionChangeCreator) EducationSubstitutionAudit {
	return EducationSubstitutionAudit{changes: changes}
}

// RecordSubstitutionChange appends one change.
func (a EducationSubstitutionAudit) RecordSubstitutionChange(ctx context.Context, change educationRepo.SubstitutionChange) error {
	row := &auditModels.SubstitutionChange{
		SubstitutionID: change.SubstitutionID, TargetType: change.TargetType, Action: string(change.Action),
		GroupID: change.GroupID, TargetStaffID: change.TargetStaffID, ActorAccountID: change.ActorAccountID,
		StartDate: auditModels.Date(change.StartDate),
	}
	if change.EndDate != nil {
		endDate := auditModels.Date(*change.EndDate)
		row.EndDate = &endDate
	}
	return a.changes.Create(ctx, row)
}

// EducationClassAssignmentAudit appends class assignment rewrites (#1772) to
// the Stammdaten audit trail.
type EducationClassAssignmentAudit struct {
	changes auditModels.StaffMasterDataChangeCreator
}

// NewEducationClassAssignmentAudit binds the trail to its repository.
func NewEducationClassAssignmentAudit(changes auditModels.StaffMasterDataChangeCreator) EducationClassAssignmentAudit {
	return EducationClassAssignmentAudit{changes: changes}
}

// RecordSchoolClassChange appends one rewrite in the school classes section.
func (a EducationClassAssignmentAudit) RecordSchoolClassChange(ctx context.Context, change educationRepo.SchoolClassChange) error {
	return a.changes.Create(ctx, &auditModels.StaffMasterDataChange{
		StaffID:   change.StaffID,
		ChangedBy: change.ChangedBy,
		Section:   auditModels.StammdatenSectionSchoolClasses,
		FieldName: "school_classes",
		OldValue:  change.OldValue,
		NewValue:  change.NewValue,
	})
}
