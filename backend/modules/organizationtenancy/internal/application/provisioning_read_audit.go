package application

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy"
	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy/internal/domain"
)

// RecordOperatorRead must commit before the HTTP adapter releases any data.
// Failed reads are attempts, not evidence that a response reached its recipient.
func (p *Provisioning) RecordOperatorRead(ctx context.Context, access organizationtenancy.OperatorReadAccess) error {
	if access.OperatorID <= 0 {
		return invalidData("operator is required")
	}
	resource, err := operatorReadResource(access.View)
	if err != nil {
		return err
	}
	changes := map[string]any{"view": access.View}
	if access.SchoolID > 0 {
		changes["school_id"] = access.SchoolID
	}
	if access.OrganizationID > 0 {
		changes["organization_id"] = access.OrganizationID
	}
	var resourceID *int64
	if access.AccountID > 0 {
		resourceID = &access.AccountID
	}
	if access.AnnouncementID > 0 {
		resourceID = &access.AnnouncementID
	}
	if resource == domain.AuditResourceSchool && access.SchoolID > 0 {
		resourceID = &access.SchoolID
	}
	if (resource == domain.AuditResourceTraegerSetting || resource == domain.AuditResourceBelegRetention || resource == domain.AuditResourceTraegerStaffRelease) && access.OrganizationID > 0 {
		resourceID = &access.OrganizationID
	}
	_, err = adminValue(ctx, p, func(adminCtx context.Context) (struct{}, error) {
		return struct{}{}, p.recordAction(adminCtx, access.OperatorID, domain.AuditActionView, resource, resourceID, access.ClientIP, changes)
	})
	return err
}

// operatorReadResource names the audit resource of an operator read view.
func operatorReadResource(view organizationtenancy.OperatorReadView) (string, error) {
	resource := domain.AuditResourceAccount
	switch view {
	case organizationtenancy.ReadSchoolBookingAuthorityImpact, organizationtenancy.ReadSchools,
		organizationtenancy.ReadSchool, organizationtenancy.ReadSchoolSummaries, organizationtenancy.ReadOrganizationSchools,
		organizationtenancy.ReadSchoolSettingsSchema, organizationtenancy.ReadSchoolSettingReveal,
		organizationtenancy.ReadDemoPresenceHistory:
		resource = domain.AuditResourceSchool
	case organizationtenancy.ReadSchoolAdminInvitation:
		resource = domain.AuditResourceInvitation
	case organizationtenancy.ReadUnregisteredTagScans:
		resource = domain.AuditResourceTagScan
	case organizationtenancy.ReadTraegerMembers:
		resource = domain.AuditResourceTraegerMember
	case organizationtenancy.ReadTraegerSettings:
		resource = domain.AuditResourceTraegerSetting
	case organizationtenancy.ReadBelegRetention:
		resource = domain.AuditResourceBelegRetention
	case organizationtenancy.ReadTraegerStaffRelease:
		resource = domain.AuditResourceTraegerStaffRelease
	case organizationtenancy.ReadTraegerInvitations:
		resource = domain.AuditResourceTraegerInvitation
	case organizationtenancy.ReadSchoolPersons, organizationtenancy.ReadOrganizationPersons:
		resource = domain.AuditResourcePerson
	case organizationtenancy.ReadAnnouncementViews, organizationtenancy.ReadAnnouncements, organizationtenancy.ReadAnnouncement:
		resource = domain.AuditResourceAnnouncement
	case organizationtenancy.ReadAllAccounts, organizationtenancy.ReadSchoolAccounts,
		organizationtenancy.ReadOrganizationAccounts, organizationtenancy.ReadSchoolAccountCaregiver,
		organizationtenancy.ReadSchoolAccountMFA, organizationtenancy.ReadAccountMFA, organizationtenancy.ReadAccountTenants,
		organizationtenancy.ReadSchoolAccountCreation, organizationtenancy.ReadCaterers:
	default:
		return "", invalidData("unknown operator read view")
	}
	return resource, nil
}
