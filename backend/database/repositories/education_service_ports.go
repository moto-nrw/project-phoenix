package repositories

import (
	"context"

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	facilityModels "github.com/moto-nrw/project-phoenix/models/facilities"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// The ports of the retained School Structure services (services/education)
// the legacy composition binds over the retained repositories (#2742). Each
// adapter translates the People Directory, Facilities and Audit Platform rows
// into the models/education vocabulary the services own, and changes nothing
// about which reads and writes run.

// EducationRooms serves the group service's room directory from the
// Facilities rooms.
type EducationRooms struct{ rooms facilityModels.RoomRepository }

// NewEducationRooms binds the room directory to the room repository.
func NewEducationRooms(rooms facilityModels.RoomRepository) EducationRooms {
	return EducationRooms{rooms: rooms}
}

// FindRoom returns the room a group may be assigned to.
func (r EducationRooms) FindRoom(ctx context.Context, id int64) (*educationModels.GroupRoom, error) {
	room, err := r.rooms.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &educationModels.GroupRoom{
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
func (c EducationCaregivers) FindActiveCaregiverByAccountID(ctx context.Context, accountID int64) (*educationModels.Caregiver, error) {
	caregiver, err := c.teachers.FindActiveCaregiverByAccountID(ctx, accountID)
	if err != nil || caregiver == nil {
		return nil, err
	}
	return educationCaregiver(caregiver), nil
}

// ListActiveCaregivers returns every active caregiver of the school, ordered
// by name.
func (c EducationCaregivers) ListActiveCaregivers(ctx context.Context) ([]*educationModels.Caregiver, error) {
	caregivers, err := c.teachers.ListActiveCaregivers(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*educationModels.Caregiver, 0, len(caregivers))
	for _, caregiver := range caregivers {
		if caregiver != nil {
			result = append(result, educationCaregiver(caregiver))
		}
	}
	return result, nil
}

func educationCaregiver(caregiver *userModels.ActiveCaregiver) *educationModels.Caregiver {
	return &educationModels.Caregiver{StaffID: caregiver.StaffID, TeacherID: caregiver.TeacherID, FullName: caregiver.FullName()}
}

// EducationHandovers serves the substitution module's handover store from the
// retained group substitution repository.
type EducationHandovers struct {
	substitutions GroupSubstitutionRepository
}

// NewEducationHandovers binds the handover store to the group substitution
// repository.
func NewEducationHandovers(substitutions GroupSubstitutionRepository) EducationHandovers {
	return EducationHandovers{substitutions: substitutions}
}

// Create stores a handover and reports educationModels.ErrHandoverExists for
// a duplicate.
func (h EducationHandovers) Create(ctx context.Context, handover *educationModels.GroupSubstitution) error {
	if err := h.substitutions.Create(ctx, handover); err != nil {
		if userModels.IsUniqueViolation(err) {
			return educationModels.ErrHandoverExists
		}
		return err
	}
	return nil
}

// Delete removes a handover.
func (h EducationHandovers) Delete(ctx context.Context, id any) error {
	return h.substitutions.Delete(ctx, id)
}

// FindByID reads one handover.
func (h EducationHandovers) FindByID(ctx context.Context, id any) (*educationModels.GroupSubstitution, error) {
	return h.substitutions.FindByID(ctx, id)
}

// FindByIDForUpdate reads one handover under a row lock.
func (h EducationHandovers) FindByIDForUpdate(ctx context.Context, id any) (*educationModels.GroupSubstitution, error) {
	return h.substitutions.FindByIDForUpdate(ctx, id)
}

// ListHandovers selects the handovers of one school.
func (h EducationHandovers) ListHandovers(ctx context.Context, query educationModels.HandoverQuery) ([]*educationModels.GroupSubstitution, error) {
	return h.substitutions.ListWithOptions(ctx, handoverQueryOptions(query))
}

// ListHandoversWithRelations is ListHandovers with the group and the staff
// members attached.
func (h EducationHandovers) ListHandoversWithRelations(ctx context.Context, query educationModels.HandoverQuery) ([]*educationModels.GroupSubstitution, error) {
	return h.substitutions.ListWithRelations(ctx, handoverQueryOptions(query))
}

func handoverQueryOptions(query educationModels.HandoverQuery) *userModels.QueryOptions {
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

// The substitution module may not name the Audit Platform, so models/education
// mirrors the actions the trail stores, and the binding below passes them
// through unchanged. A drift must not compile: a false comparison repeats the
// false key of this map literal.
var _ = map[bool]struct{}{
	false: {},
	auditModels.SubstitutionAssigned == educationModels.SubstitutionAssigned &&
		auditModels.SubstitutionEnded == educationModels.SubstitutionEnded: {},
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
func (a EducationSubstitutionAudit) RecordSubstitutionChange(ctx context.Context, change educationModels.SubstitutionChange) error {
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
func (a EducationClassAssignmentAudit) RecordSchoolClassChange(ctx context.Context, change educationModels.SchoolClassChange) error {
	return a.changes.Create(ctx, &auditModels.StaffMasterDataChange{
		StaffID:   change.StaffID,
		ChangedBy: change.ChangedBy,
		Section:   auditModels.StammdatenSectionSchoolClasses,
		FieldName: "school_classes",
		OldValue:  change.OldValue,
		NewValue:  change.NewValue,
	})
}
