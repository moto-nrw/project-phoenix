package owners

import (
	"context"
	"errors"
	"fmt"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ExternalCaregiverRecordMembershipRepository serves the People Directory's
// ExternalCaregiverRecordRepository over the School Membership capability.
type ExternalCaregiverRecordMembershipRepository struct {
	Membership schoolmembership.Capability
}

func externalCaregiverRecordFieldsFromLegacy(record *peopleValues.ExternalCaregiverRecord) schoolmembership.ExternalCaregiverRecordFields {
	return schoolmembership.ExternalCaregiverRecordFields{
		StaffID:           record.StaffID,
		Organization:      record.Organization,
		ContactEmail:      record.ContactEmail,
		ContactPhone:      record.ContactPhone,
		ActivityExpertise: record.ActivityExpertise,
		StartDate:         calendarDateString(record.StartDate),
		EndDate:           calendarDateString(record.EndDate),
		Notes:             record.Notes,
	}
}

func applyExternalCaregiverRecordToLegacy(target *peopleValues.ExternalCaregiverRecord, value schoolmembership.ExternalCaregiverRecord) {
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

func toLegacyExternalCaregiverRecord(value schoolmembership.ExternalCaregiverRecord) *peopleValues.ExternalCaregiverRecord {
	record := new(peopleValues.ExternalCaregiverRecord)
	applyExternalCaregiverRecordToLegacy(record, value)
	return record
}

func toLegacyExternalCaregiverRecordList(values []schoolmembership.ExternalCaregiverRecord) []*peopleValues.ExternalCaregiverRecord {
	result := make([]*peopleValues.ExternalCaregiverRecord, 0, len(values))
	for _, value := range values {
		result = append(result, toLegacyExternalCaregiverRecord(value))
	}
	return result
}

func (r ExternalCaregiverRecordMembershipRepository) Create(ctx context.Context, entity *peopleValues.ExternalCaregiverRecord) error {
	if entity == nil {
		return repositoryError("create external caregiver record", errors.New("record cannot be nil"))
	}
	created, err := r.Membership.CreateExternalCaregiverRecord(ctx, schoolmembership.CreateExternalCaregiverRecord{ExternalCaregiverRecordFields: externalCaregiverRecordFieldsFromLegacy(entity)})
	if err != nil {
		return membershipError("create external caregiver record", err)
	}
	applyExternalCaregiverRecordToLegacy(entity, created)
	return nil
}

func (r ExternalCaregiverRecordMembershipRepository) Update(ctx context.Context, entity *peopleValues.ExternalCaregiverRecord) error {
	if entity == nil {
		return repositoryError("update external caregiver record", errors.New("record cannot be nil"))
	}
	updated, err := r.Membership.UpdateExternalCaregiverRecord(ctx, schoolmembership.UpdateExternalCaregiverRecord{ID: entity.ID, ExternalCaregiverRecordFields: externalCaregiverRecordFieldsFromLegacy(entity)})
	if err != nil {
		return membershipError("update external caregiver record", err)
	}
	applyExternalCaregiverRecordToLegacy(entity, updated)
	return nil
}

func (r ExternalCaregiverRecordMembershipRepository) Delete(ctx context.Context, id any) error {
	recordID, err := MembershipID(id)
	if err != nil {
		return repositoryError("delete external caregiver record", err)
	}
	return membershipError("delete external caregiver record", r.Membership.DeleteExternalCaregiverRecord(ctx, recordID))
}

func (r ExternalCaregiverRecordMembershipRepository) FindByID(ctx context.Context, id any) (*peopleValues.ExternalCaregiverRecord, error) {
	recordID, err := MembershipID(id)
	if err != nil {
		return nil, repositoryError("find record by id", err)
	}
	value, err := r.Membership.FindExternalCaregiverRecord(ctx, recordID)
	if err != nil {
		return nil, membershipError("find record by id", err)
	}
	return toLegacyExternalCaregiverRecord(value), nil
}

func (r ExternalCaregiverRecordMembershipRepository) FindByStaffID(ctx context.Context, staffID int64) (*peopleValues.ExternalCaregiverRecord, error) {
	value, err := r.Membership.FindExternalCaregiverRecordByStaff(ctx, staffID)
	if err != nil {
		return nil, membershipError("find by staff ID", err)
	}
	return toLegacyExternalCaregiverRecord(value), nil
}

func externalCaregiverRecordFilterFromLegacy(filters map[string]any) (schoolmembership.ExternalCaregiverRecordFilter, error) {
	filter := schoolmembership.ExternalCaregiverRecordFilter{}
	for field, value := range filters {
		if value == nil {
			continue
		}
		apply, ok := externalCaregiverRecordFilterFields[field]
		if !ok {
			return filter, fmt.Errorf("unsupported record filter %q", field)
		}
		if err := apply(&filter, field, value); err != nil {
			return filter, err
		}
	}
	return filter, nil
}

// externalCaregiverRecordFilterFields translates each supported legacy filter key.
var externalCaregiverRecordFilterFields = map[string]func(*schoolmembership.ExternalCaregiverRecordFilter, string, any) error{
	"id": func(filter *schoolmembership.ExternalCaregiverRecordFilter, _ string, value any) error {
		id, err := MembershipID(value)
		if err == nil {
			filter.IDs = append(filter.IDs, id)
		}
		return err
	},
	"staff_id": func(filter *schoolmembership.ExternalCaregiverRecordFilter, _ string, value any) error {
		id, err := MembershipID(value)
		if err == nil {
			filter.StaffIDs = append(filter.StaffIDs, id)
		}
		return err
	},
	"organization_like": func(filter *schoolmembership.ExternalCaregiverRecordFilter, field string, value any) error {
		text, err := legacyFilterString("external caregiver record", field, value)
		filter.OrganizationContains = text
		return err
	},
	"expertise_like": func(filter *schoolmembership.ExternalCaregiverRecordFilter, field string, value any) error {
		text, err := legacyFilterString("external caregiver record", field, value)
		filter.ExpertiseContains = text
		return err
	},
	"active": func(filter *schoolmembership.ExternalCaregiverRecordFilter, field string, value any) error {
		active, err := legacyFilterBool("external caregiver record", field, value)
		if err == nil && active {
			filter.ActiveOn = calendar.TodayDate().String()
		}
		return err
	},
	"current_date": func(filter *schoolmembership.ExternalCaregiverRecordFilter, field string, value any) error {
		day, ok := value.(interface{ String() string })
		if !ok {
			return fmt.Errorf("record filter %q must be a calendar date", field)
		}
		filter.ActiveOn = day.String()
		return nil
	},
	"has_organization": func(filter *schoolmembership.ExternalCaregiverRecordFilter, field string, value any) error {
		has, err := legacyFilterBool("external caregiver record", field, value)
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

func (r ExternalCaregiverRecordMembershipRepository) List(ctx context.Context, filters map[string]any) ([]*peopleValues.ExternalCaregiverRecord, error) {
	filter, err := externalCaregiverRecordFilterFromLegacy(filters)
	if err != nil {
		return nil, repositoryError("list external caregiver records", err)
	}
	return r.listExternalCaregiverRecords(ctx, filter)
}

// ActiveExternalCaregiverRecords lists the records whose window covers today's calendar date.
func (r ExternalCaregiverRecordMembershipRepository) ActiveExternalCaregiverRecords(ctx context.Context) ([]*peopleValues.ExternalCaregiverRecord, error) {
	return r.listExternalCaregiverRecords(ctx, schoolmembership.ExternalCaregiverRecordFilter{ActiveOn: calendar.TodayDate().String()})
}

func (r ExternalCaregiverRecordMembershipRepository) listExternalCaregiverRecords(ctx context.Context, filter schoolmembership.ExternalCaregiverRecordFilter) ([]*peopleValues.ExternalCaregiverRecord, error) {
	values, err := r.Membership.ListExternalCaregiverRecords(ctx, filter)
	if err != nil {
		return nil, membershipError("list external caregiver records", err)
	}
	return toLegacyExternalCaregiverRecordList(values), nil
}
