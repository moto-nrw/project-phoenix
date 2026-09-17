package compose

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/adapters/operatordashboard"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/adapters/secrets"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
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
	Organizations organizationtenancy.Capability
	Identity      ProvisioningIdentity
	Devices       ProvisioningDevices
	People        ProvisioningPeople
	Presence      ProvisioningPresence
	Categories    ProvisioningCategories
	Settings      ProvisioningSettings
	Audit         OperatorAudit
	Logger        *slog.Logger
}

// NewProvisioning composes the operator provisioning capability over the
// Organisation & Tenancy facade and the operator dashboard projection.
func NewProvisioning(dependencies ProvisioningDependencies) (organizationtenancy.Provisioning, error) {
	dashboard := operatordashboard.New(func(ctx context.Context) (bun.IDB, error) {
		transaction, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return nil, errors.New("organization tenancy operator dashboard: transaction is required")
		}
		tx, ok := transaction.(bun.Tx)
		if !ok {
			return nil, fmt.Errorf("organization tenancy operator dashboard: unsupported transaction %T", transaction)
		}
		return tx, nil
	})
	provisioning, err := application.NewProvisioning(application.ProvisioningDependencies{
		Organizations: dependencies.Organizations, Transaction: transaction{}, Dashboard: dashboard,
		Identity: dependencies.Identity, Devices: dependencies.Devices, People: dependencies.People,
		Presence: dependencies.Presence, Categories: dependencies.Categories, Settings: dependencies.Settings,
		Audit: dependencies.Audit, Secrets: secrets.Generator{}, Logger: dependencies.Logger,
	})
	if err != nil {
		return nil, err
	}
	return provisioning, nil
}
