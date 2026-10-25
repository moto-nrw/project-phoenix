package organizationtenancy

import (
	"context"
	"net"
)

// OperatorReadView is a fixed read category, never a URL or free-text filter.
type OperatorReadView string

const (
	ReadAnnouncements                OperatorReadView = "announcements"
	ReadAnnouncement                 OperatorReadView = "announcement"
	ReadAllAccounts                  OperatorReadView = "accounts"
	ReadSchoolAccounts               OperatorReadView = "school_accounts"
	ReadOrganizationAccounts         OperatorReadView = "organization_accounts"
	ReadSchoolPersons                OperatorReadView = "school_persons"
	ReadOrganizationPersons          OperatorReadView = "organization_persons"
	ReadSchoolAccountCaregiver       OperatorReadView = "school_account_caregiver"
	ReadSchoolAccountMFA             OperatorReadView = "school_account_mfa"
	ReadAccountMFA                   OperatorReadView = "account_mfa"
	ReadAccountTenants               OperatorReadView = "account_tenants"
	ReadAnnouncementViews            OperatorReadView = "announcement_views"
	ReadSchoolBookingAuthorityImpact OperatorReadView = "school_booking_authority_impact"
	ReadSchools                      OperatorReadView = "schools"
	ReadSchool                       OperatorReadView = "school"
	ReadSchoolSummaries              OperatorReadView = "school_summaries"
	ReadOrganizationSchools          OperatorReadView = "organization_schools"
	ReadSchoolAdminInvitation        OperatorReadView = "school_admin_invitation"
	ReadSchoolAccountCreation        OperatorReadView = "school_account_creation"
	ReadSchoolSettingsSchema         OperatorReadView = "school_settings_schema"
	ReadSchoolSettingReveal          OperatorReadView = "school_setting_reveal"
	ReadUnregisteredTagScans         OperatorReadView = "unregistered_tag_scans"
	ReadTraegerMembers               OperatorReadView = "traeger_members"
)

// OperatorReadAccess records an attempted disclosure, before reading its data.
// No returned rows, names, addresses, search strings or credentials belong here.
type OperatorReadAccess struct {
	View           OperatorReadView
	OperatorID     int64
	SchoolID       int64
	OrganizationID int64
	AccountID      int64
	AnnouncementID int64
	ClientIP       net.IP
}

type OperatorReadAudit interface {
	RecordOperatorRead(ctx context.Context, access OperatorReadAccess) error
}
