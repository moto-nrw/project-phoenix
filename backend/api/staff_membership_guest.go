package api

import (
	"context"
	"errors"
	"fmt"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// guestMembershipRepository serves users.GuestRepository over the School
// Membership capability.
type guestMembershipRepository struct {
	membership schoolmembership.Capability
}

var _ peoplePorts.GuestRepository = guestMembershipRepository{}

func guestFieldsFromLegacy(guest *peopleValues.Guest) schoolmembership.GuestFields {
	return schoolmembership.GuestFields{
		StaffID:           guest.StaffID,
		Organization:      guest.Organization,
		ContactEmail:      guest.ContactEmail,
		ContactPhone:      guest.ContactPhone,
		ActivityExpertise: guest.ActivityExpertise,
		StartDate:         calendarDateString(guest.StartDate),
		EndDate:           calendarDateString(guest.EndDate),
		Notes:             guest.Notes,
	}
}

func applyGuestToLegacy(target *peopleValues.Guest, value schoolmembership.Guest) {
	target.ID = value.ID
	target.CreatedAt = value.CreatedAt
	target.UpdatedAt = value.UpdatedAt
	target.SetTenantID(value.TenantID)
	target.StaffID = value.StaffID
	target.Organization = value.Organization
	target.ContactEmail = value.ContactEmail
	target.ContactPhone = value.ContactPhone
	target.ActivityExpertise = value.ActivityExpertise
	target.StartDate = parseCalendarDate(value.StartDate)
	target.EndDate = parseCalendarDate(value.EndDate)
	target.Notes = value.Notes
}

func toLegacyGuest(value schoolmembership.Guest) *peopleValues.Guest {
	guest := new(peopleValues.Guest)
	applyGuestToLegacy(guest, value)
	return guest
}

func toLegacyGuestList(values []schoolmembership.Guest) []*peopleValues.Guest {
	result := make([]*peopleValues.Guest, 0, len(values))
	for _, value := range values {
		result = append(result, toLegacyGuest(value))
	}
	return result
}

func (r guestMembershipRepository) Create(ctx context.Context, entity *peopleValues.Guest) error {
	if entity == nil {
		return repositoryError("create guest", errors.New("guest cannot be nil"))
	}
	created, err := r.membership.CreateGuest(ctx, schoolmembership.CreateGuest{GuestFields: guestFieldsFromLegacy(entity)})
	if err != nil {
		return membershipError("create guest", err)
	}
	applyGuestToLegacy(entity, created)
	return nil
}

func (r guestMembershipRepository) Update(ctx context.Context, entity *peopleValues.Guest) error {
	if entity == nil {
		return repositoryError("update guest", errors.New("guest cannot be nil"))
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
		return repositoryError("delete guest", err)
	}
	return membershipError("delete guest", r.membership.DeleteGuest(ctx, guestID))
}

func (r guestMembershipRepository) FindByID(ctx context.Context, id any) (*peopleValues.Guest, error) {
	guestID, err := membershipID(id)
	if err != nil {
		return nil, repositoryError("find guest by id", err)
	}
	value, err := r.membership.FindGuest(ctx, guestID)
	if err != nil {
		return nil, membershipError("find guest by id", err)
	}
	return toLegacyGuest(value), nil
}

func (r guestMembershipRepository) FindByStaffID(ctx context.Context, staffID int64) (*peopleValues.Guest, error) {
	value, err := r.membership.FindGuestByStaff(ctx, staffID)
	if err != nil {
		return nil, membershipError("find by staff ID", err)
	}
	return toLegacyGuest(value), nil
}

func guestFilterFromLegacy(filters map[string]any) (schoolmembership.GuestFilter, error) {
	filter := schoolmembership.GuestFilter{}
	for field, value := range filters {
		if value == nil {
			continue
		}
		apply, ok := guestFilterFields[field]
		if !ok {
			return filter, fmt.Errorf("unsupported guest filter %q", field)
		}
		if err := apply(&filter, field, value); err != nil {
			return filter, err
		}
	}
	return filter, nil
}

// guestFilterFields translates each supported legacy filter key.
var guestFilterFields = map[string]func(*schoolmembership.GuestFilter, string, any) error{
	"id": func(filter *schoolmembership.GuestFilter, _ string, value any) error {
		id, err := membershipID(value)
		if err == nil {
			filter.IDs = append(filter.IDs, id)
		}
		return err
	},
	"staff_id": func(filter *schoolmembership.GuestFilter, _ string, value any) error {
		id, err := membershipID(value)
		if err == nil {
			filter.StaffIDs = append(filter.StaffIDs, id)
		}
		return err
	},
	"organization_like": func(filter *schoolmembership.GuestFilter, field string, value any) error {
		text, err := legacyFilterString("guest", field, value)
		filter.OrganizationContains = text
		return err
	},
	"expertise_like": func(filter *schoolmembership.GuestFilter, field string, value any) error {
		text, err := legacyFilterString("guest", field, value)
		filter.ExpertiseContains = text
		return err
	},
	"active": func(filter *schoolmembership.GuestFilter, field string, value any) error {
		active, err := legacyFilterBool("guest", field, value)
		if err == nil && active {
			filter.ActiveOn = calendar.TodayDate().String()
		}
		return err
	},
	"current_date": func(filter *schoolmembership.GuestFilter, field string, value any) error {
		day, ok := value.(interface{ String() string })
		if !ok {
			return fmt.Errorf("guest filter %q must be a calendar date", field)
		}
		filter.ActiveOn = day.String()
		return nil
	},
	"has_organization": func(filter *schoolmembership.GuestFilter, field string, value any) error {
		has, err := legacyFilterBool("guest", field, value)
		if err == nil {
			filter.HasOrganization = &has
		}
		return err
	},
}

// legacyFilterString reads a string filter value of the retained equality
// filter shape.
func legacyFilterString(kind, field string, value any) (string, error) {
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%s filter %q must be a string", kind, field)
	}
	return text, nil
}

// legacyFilterBool reads a bool filter value of the retained equality filter
// shape.
func legacyFilterBool(kind, field string, value any) (bool, error) {
	flag, ok := value.(bool)
	if !ok {
		return false, fmt.Errorf("%s filter %q must be a bool", kind, field)
	}
	return flag, nil
}

func (r guestMembershipRepository) List(ctx context.Context, filters map[string]any) ([]*peopleValues.Guest, error) {
	filter, err := guestFilterFromLegacy(filters)
	if err != nil {
		return nil, repositoryError("list guests", err)
	}
	return r.listGuests(ctx, filter)
}

// ActiveGuests lists the guests whose window covers today's calendar date.
func (r guestMembershipRepository) ActiveGuests(ctx context.Context) ([]*peopleValues.Guest, error) {
	return r.listGuests(ctx, schoolmembership.GuestFilter{ActiveOn: calendar.TodayDate().String()})
}

func (r guestMembershipRepository) listGuests(ctx context.Context, filter schoolmembership.GuestFilter) ([]*peopleValues.Guest, error) {
	values, err := r.membership.ListGuests(ctx, filter)
	if err != nil {
		return nil, membershipError("list guests", err)
	}
	return toLegacyGuestList(values), nil
}
