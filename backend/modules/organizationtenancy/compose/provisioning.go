package compose

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	postgresNative "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/ports"
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
	PWAUsageWindowDays int
	Organizations      organizationtenancy.Capability
	Observe            func(Observation) // of the owner's records command (#4720)
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
func NewProvisioning(dependencies ProvisioningDependencies, dashboard organizationtenancy.OperatorDashboard) (organizationtenancy.OperatorProvisioning, error) {
	if dependencies.Observe == nil {
		return organizationtenancy.OperatorProvisioning{}, errors.New("organization tenancy provisioning: the observer is required")
	}
	provisioning, err := application.NewProvisioning(application.ProvisioningDependencies{
		PWAUsageWindowDays: dependencies.PWAUsageWindowDays, Organizations: dependencies.Organizations,
		ErasureOrderRecords: application.New(postgresNative.New(postgresNative.NewDatabase()), postgresNative.Transaction{}, dependencies.Observe),
		Transaction:         postgresNative.Transaction{}, Dashboard: dashboard,
		Identity: dependencies.Identity, Devices: dependencies.Devices, People: dependencies.People,
		Presence: dependencies.Presence, Categories: dependencies.Categories, Settings: dependencies.Settings,
		Audit: dependencies.Audit, Secrets: dependencies.Secrets, Logger: dependencies.Logger,
	})
	if err != nil {
		return organizationtenancy.OperatorProvisioning{}, err
	}
	return organizationtenancy.OperatorProvisioning{Provisioning: provisioning, ErasureOrderRecords: provisioning}, nil
}

// BelegRetentionAudit answers the operator audit of the Beleg retention
// (#4720, ADR 0184) of the provisioning NewProvisioning composed; it records
// on the same audit and transaction as every provisioning write.
func BelegRetentionAudit(provisioning organizationtenancy.Provisioning) (organizationtenancy.BelegRetentionChangeAudit, error) {
	if audit, ok := provisioning.(*application.Provisioning); ok {
		return audit, nil
	}
	return nil, fmt.Errorf("organization tenancy: the Beleg retention audit needs the composed provisioning, got %T", provisioning)
}

func OperatorDatabase() func(context.Context) (bun.IDB, error) {
	return postgresNative.OperatorDatabase()
}
