package compose

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/ports"
)

// The provisioning seams (#3253) and the values they exchange. The root
// binds them to the owners they name.
type (
	ProvisioningIdentity   = ports.ProvisioningIdentity
	ProvisioningDevices    = ports.ProvisioningDevices
	ProvisioningPeople     = ports.ProvisioningPeople
	ProvisioningPresence   = ports.ProvisioningPresence
	ProvisioningCategories = ports.ProvisioningCategories
	ProvisioningSettings   = ports.ProvisioningSettings
	OperatorAudit          = ports.OperatorAudit

	OperatorAuditEntry           = domain.OperatorAuditEntry
	ProvisioningRole             = domain.Role
	SchoolAdminInvitationRequest = domain.SchoolAdminInvitationRequest
	SchoolAdminInvitation        = domain.SchoolAdminInvitation
	SchoolAccountRegistration    = domain.SchoolAccountRegistration
	CreatedAccount               = domain.CreatedAccount
	SchoolIdentityRequest        = domain.SchoolIdentityRequest
	SchoolAccount                = domain.SchoolAccount
	OrganizationAccount          = domain.OrganizationAccount
	ProvisioningDevice           = domain.Device
	NewProvisioningDevice        = domain.NewDevice
	DeviceSession                = domain.DeviceSession
	ProvisioningPerson           = domain.Person
	PersonListing                = domain.PersonListing
	StaffMember                  = domain.StaffMember
	ActivityCategory             = domain.ActivityCategory
)

// The provisioning port errors the adapters translate their owners' failures
// to.
var (
	ErrDeviceIDTaken         = domain.ErrDeviceIDTaken
	ErrDeviceAPIKeyTaken     = domain.ErrDeviceAPIKeyTaken
	ErrDeviceReferenced      = domain.ErrDeviceReferenced
	ErrInvalidSchoolIdentity = domain.ErrInvalidSchoolIdentity
)

// Device vocabulary the adapters share with the application.
const (
	WebManualDeviceID = domain.WebManualDeviceID
)

// ProvisioningDependencies are the owners operator provisioning works
// through. Every field except Logger is required.
type ProvisioningDependencies struct {
	PWAUsageWindowDays int
	Organizations      organizationtenancy.Capability
	Identity           ProvisioningIdentity
	Devices            ProvisioningDevices
	People             ProvisioningPeople
	Presence           ProvisioningPresence
	Categories         ProvisioningCategories
	Settings           ProvisioningSettings
	Audit              OperatorAudit
	Secrets            ports.ProvisioningSecrets
	// ActiveAccountCounts supplies bounded distinct-account aggregates.
	Logger *slog.Logger
}

// NewProvisioning composes the operator provisioning capability over the
// Organisation & Tenancy facade and the operator dashboard projection.
func NewProvisioning(dependencies ProvisioningDependencies, dashboard organizationtenancy.OperatorDashboard) (organizationtenancy.Provisioning, error) {
	return application.NewProvisioning(application.ProvisioningDependencies{
		PWAUsageWindowDays: dependencies.PWAUsageWindowDays,
		Organizations:      dependencies.Organizations, Transaction: transaction{}, Dashboard: dashboard,
		Identity: dependencies.Identity, Devices: dependencies.Devices, People: dependencies.People,
		Presence: dependencies.Presence, Categories: dependencies.Categories, Settings: dependencies.Settings,
		Audit: dependencies.Audit, Secrets: dependencies.Secrets, Logger: dependencies.Logger,
	})
}

// OperatorDatabase supplies the existing administrative runtime to root-bound projections.
func OperatorDatabase() func(context.Context) (bun.IDB, error) {
	return func(ctx context.Context) (bun.IDB, error) {
		value, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return nil, errors.New("organization tenancy operator dashboard: transaction is required")
		}
		tx, ok := value.(bun.Tx)
		if !ok {
			return nil, fmt.Errorf("organization tenancy operator dashboard: unsupported transaction %T", value)
		}
		return tx, nil
	}
}
