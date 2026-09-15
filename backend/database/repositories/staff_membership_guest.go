package repositories

import (
	"context"
	"errors"
	"fmt"

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

// guestMembershipRepository serves users.GuestRepository over the School
// Membership capability.
type guestMembershipRepository struct {
	membership schoolmembership.Capability
}

var _ userModels.GuestRepository = guestMembershipRepository{}

func guestFieldsFromLegacy(guest *userModels.Guest) schoolmembership.GuestFields {
	return schoolmembership.GuestFields{
		StaffID:           guest.StaffID,
		Organization:      guest.Organization,
		ContactEmail:      guest.ContactEmail,
		ContactPhone:      guest.ContactPhone,
		ActivityExpertise: guest.ActivityExpertise,
		StartDate:         usersRepo.CalendarDateString(guest.StartDate),
		EndDate:           usersRepo.CalendarDateString(guest.EndDate),
		Notes:             guest.Notes,
	}
}

func applyGuestToLegacy(target *userModels.Guest, value schoolmembership.Guest) {
	target.ID = value.ID
	target.CreatedAt = value.CreatedAt
	target.UpdatedAt = value.UpdatedAt
	target.SetTenantID(value.TenantID)
	target.StaffID = value.StaffID
	target.Organization = value.Organization
	target.ContactEmail = value.ContactEmail
	target.ContactPhone = value.ContactPhone
	target.ActivityExpertise = value.ActivityExpertise
	target.StartDate = usersRepo.ParseCalendarDate(value.StartDate)
	target.EndDate = usersRepo.ParseCalendarDate(value.EndDate)
	target.Notes = value.Notes
}

func toLegacyGuest(value schoolmembership.Guest) *userModels.Guest {
	guest := new(userModels.Guest)
	applyGuestToLegacy(guest, value)
	return guest
}

func toLegacyGuestList(values []schoolmembership.Guest) []*userModels.Guest {
	result := make([]*userModels.Guest, 0, len(values))
	for _, value := range values {
		result = append(result, toLegacyGuest(value))
	}
	return result
}

func (r guestMembershipRepository) Create(ctx context.Context, entity *userModels.Guest) error {
	if entity == nil {
		return usersRepo.WrapError("create guest", errors.New("guest cannot be nil"))
	}
	created, err := r.membership.CreateGuest(ctx, schoolmembership.CreateGuest{GuestFields: guestFieldsFromLegacy(entity)})
	if err != nil {
		return membershipError("create guest", err)
	}
	applyGuestToLegacy(entity, created)
	return nil
}

func (r guestMembershipRepository) Update(ctx context.Context, entity *userModels.Guest) error {
	if entity == nil {
		return usersRepo.WrapError("update guest", errors.New("guest cannot be nil"))
	}
	updated, err := r.membership.UpdateGuest(ctx, schoolmembership.UpdateGuest{ID: entity.ID, GuestFields: guestFieldsFromLegacy(entity)})
	if err != nil {
		return membershipError("update guest", err)
	}
	applyGuestToLegacy(entity, updated)
	return nil
}

func (r guestMembershipRepository) Delete(ctx context.Context, id any) error {
	guestID, err := membershipID(id)
	if err != nil {
		return usersRepo.WrapError("delete guest", err)
	}
	return membershipError("delete guest", r.membership.DeleteGuest(ctx, guestID))
}

func (r guestMembershipRepository) FindByID(ctx context.Context, id any) (*userModels.Guest, error) {
	guestID, err := membershipID(id)
	if err != nil {
		return nil, usersRepo.WrapError("find guest by id", err)
	}
	value, err := r.membership.FindGuest(ctx, guestID)
	if err != nil {
		return nil, membershipError("find guest by id", err)
	}
	return toLegacyGuest(value), nil
}

func (r guestMembershipRepository) FindByStaffID(ctx context.Context, staffID int64) (*userModels.Guest, error) {
	value, err := r.membership.FindGuestByStaff(ctx, staffID)
	if err != nil {
		return nil, membershipError("find by staff ID", err)
	}
	return toLegacyGuest(value), nil
}

func (r guestMembershipRepository) FindActive(ctx context.Context) ([]*userModels.Guest, error) {
	values, err := r.membership.ListGuests(ctx, schoolmembership.GuestFilter{ActiveOn: usersRepo.TodayCalendarDate()})
	if err != nil {
		return nil, membershipError("find active", err)
	}
	return toLegacyGuestList(values), nil
}

func guestFilterFromLegacy(filters map[string]any) (schoolmembership.GuestFilter, error) {
	filter := schoolmembership.GuestFilter{}
	for field, value := range filters {
		if value == nil {
			continue
		}
		switch field {
		case "id":
			id, err := membershipID(value)
			if err != nil {
				return filter, err
			}
			filter.IDs = append(filter.IDs, id)
		case "staff_id":
			id, err := membershipID(value)
			if err != nil {
				return filter, err
			}
			filter.StaffIDs = append(filter.StaffIDs, id)
		case "organization_like":
			text, ok := value.(string)
			if !ok {
				return filter, fmt.Errorf("guest filter %q must be a string", field)
			}
			filter.OrganizationContains = text
		case "expertise_like":
			text, ok := value.(string)
			if !ok {
				return filter, fmt.Errorf("guest filter %q must be a string", field)
			}
			filter.ExpertiseContains = text
		case "active":
			active, ok := value.(bool)
			if !ok {
				return filter, fmt.Errorf("guest filter %q must be a bool", field)
			}
			if active {
				filter.ActiveOn = usersRepo.TodayCalendarDate()
			}
		case "current_date":
			day, ok := value.(interface{ String() string })
			if !ok {
				return filter, fmt.Errorf("guest filter %q must be a calendar date", field)
			}
			filter.ActiveOn = day.String()
		case "has_organization":
			has, ok := value.(bool)
			if !ok {
				return filter, fmt.Errorf("guest filter %q must be a bool", field)
			}
			filter.HasOrganization = &has
		default:
			return filter, fmt.Errorf("unsupported guest filter %q", field)
		}
	}
	return filter, nil
}

func (r guestMembershipRepository) List(ctx context.Context, filters map[string]any) ([]*userModels.Guest, error) {
	filter, err := guestFilterFromLegacy(filters)
	if err != nil {
		return nil, usersRepo.WrapError("list guests", err)
	}
	values, err := r.membership.ListGuests(ctx, filter)
	if err != nil {
		return nil, membershipError("list guests", err)
	}
	return toLegacyGuestList(values), nil
}
