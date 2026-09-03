package repositories

import (
	"context"
	"errors"
	"fmt"

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

// classListEntryMembershipRepository serves the legacy
// users.ClassListEntryRepository over the School Membership capability (#2668).
// users.class_list_entries belongs to that owner; nothing here builds SQL.
type classListEntryMembershipRepository struct {
	membership schoolmembership.Capability
}

var _ userModels.ClassListEntryRepository = classListEntryMembershipRepository{}

// classListEntryError maps an owner error onto the legacy repository error
// shape. A duplicate keeps its cause in the chain, so the service's
// index-name classification (IsUniqueViolationOn) keeps working; an invalid
// membership surfaces as the validation error the base repository returned.
func classListEntryError(op string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, schoolmembership.ErrClassListEntryNotFound) {
		return membershipNotFound(op)
	}
	return usersRepo.WrapError(op, err)
}

func classListEntryFieldsFromLegacy(entry *userModels.ClassListEntry) schoolmembership.ClassListEntryFields {
	return schoolmembership.ClassListEntryFields{
		FirstName: entry.FirstName, LastName: entry.LastName, SchoolClass: entry.SchoolClass,
	}
}

func applyClassListEntryToLegacy(target *userModels.ClassListEntry, value schoolmembership.ClassListEntry) {
	target.ID = value.ID
	target.CreatedAt = value.CreatedAt
	target.UpdatedAt = value.UpdatedAt
	target.SetTenantID(value.TenantID)
	target.FirstName = value.FirstName
	target.LastName = value.LastName
	target.SchoolClass = value.SchoolClass
	target.CreatedBy = value.CreatedBy
}

func toLegacyClassListEntry(value schoolmembership.ClassListEntry) *userModels.ClassListEntry {
	entry := new(userModels.ClassListEntry)
	applyClassListEntryToLegacy(entry, value)
	return entry
}

func toLegacyClassListEntries(values []schoolmembership.ClassListEntry) []*userModels.ClassListEntry {
	result := make([]*userModels.ClassListEntry, 0, len(values))
	for _, value := range values {
		result = append(result, toLegacyClassListEntry(value))
	}
	return result
}

func (r classListEntryMembershipRepository) Create(ctx context.Context, entity *userModels.ClassListEntry) error {
	if entity == nil {
		return usersRepo.WrapError("create class list entry", errors.New("class list entry cannot be nil"))
	}
	created, err := r.membership.CreateClassListEntry(ctx, schoolmembership.CreateClassListEntry{
		ClassListEntryFields: classListEntryFieldsFromLegacy(entity),
		CreatedBy:            entity.CreatedBy,
	})
	if err != nil {
		return classListEntryError("create class list entry", err)
	}
	applyClassListEntryToLegacy(entity, created)
	return nil
}

func (r classListEntryMembershipRepository) FindByID(ctx context.Context, id any) (*userModels.ClassListEntry, error) {
	entryID, err := membershipID(id)
	if err != nil {
		return nil, usersRepo.WrapError("find class list entry by id", err)
	}
	value, err := r.membership.FindClassListEntry(ctx, entryID)
	if err != nil {
		return nil, classListEntryError("find class list entry by id", err)
	}
	return toLegacyClassListEntry(value), nil
}

func (r classListEntryMembershipRepository) Update(ctx context.Context, entity *userModels.ClassListEntry) error {
	if entity == nil {
		return usersRepo.WrapError("update class list entry", errors.New("class list entry cannot be nil"))
	}
	updated, err := r.membership.UpdateClassListEntry(ctx, schoolmembership.UpdateClassListEntry{
		ID: entity.ID, ClassListEntryFields: classListEntryFieldsFromLegacy(entity),
	})
	if err != nil {
		return classListEntryError("update class list entry", err)
	}
	applyClassListEntryToLegacy(entity, updated)
	return nil
}

// Delete stays idempotent like the base repository it replaces: deleting a
// row that is already gone is not an error for the callers that replay a
// grade-transition ledger.
func (r classListEntryMembershipRepository) Delete(ctx context.Context, id any) error {
	entryID, err := membershipID(id)
	if err != nil {
		return usersRepo.WrapError("delete class list entry", err)
	}
	err = r.membership.DeleteClassListEntry(ctx, entryID)
	if errors.Is(err, schoolmembership.ErrClassListEntryNotFound) {
		return nil
	}
	return classListEntryError("delete class list entry", err)
}

// List keeps the legacy equality-filter shape for the keys real callers pass;
// anything else is an explicit error instead of a silently ignored filter.
func (r classListEntryMembershipRepository) List(ctx context.Context, filters map[string]any) ([]*userModels.ClassListEntry, error) {
	filter := schoolmembership.ClassListEntryFilter{}
	for field, value := range filters {
		if value == nil {
			continue
		}
		switch field {
		case "id":
			id, err := membershipID(value)
			if err != nil {
				return nil, usersRepo.WrapError("list class list entries", err)
			}
			filter.IDs = append(filter.IDs, id)
		case "school_class":
			schoolClass, ok := value.(string)
			if !ok {
				return nil, usersRepo.WrapError("list class list entries", fmt.Errorf("unsupported school_class filter %T", value))
			}
			filter.SchoolClass = schoolClass
		default:
			return nil, usersRepo.WrapError("list class list entries", fmt.Errorf("unsupported class list entry filter %q", field))
		}
	}
	values, err := r.membership.ListClassListEntries(ctx, filter)
	if err != nil {
		return nil, classListEntryError("list class list entries", err)
	}
	return toLegacyClassListEntries(values), nil
}

func (r classListEntryMembershipRepository) FindBySchoolClass(ctx context.Context, schoolClass string) ([]*userModels.ClassListEntry, error) {
	values, err := r.membership.ListClassListEntries(ctx, schoolmembership.ClassListEntryFilter{SchoolClass: schoolClass})
	if err != nil {
		return nil, classListEntryError("find by school class", err)
	}
	return toLegacyClassListEntries(values), nil
}

func (r classListEntryMembershipRepository) FindByNameAndClass(ctx context.Context, firstName, lastName, schoolClass string) ([]*userModels.ClassListEntry, error) {
	values, err := r.membership.ListClassListEntries(ctx, schoolmembership.ClassListEntryFilter{
		FirstName: firstName, LastName: lastName, SchoolClass: schoolClass,
	})
	if err != nil {
		return nil, classListEntryError("find by name and class", err)
	}
	return toLegacyClassListEntries(values), nil
}
