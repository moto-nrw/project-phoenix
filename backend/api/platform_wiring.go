package api

import (
	"errors"
	"fmt"
	"time"

	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	deliveryModule "github.com/moto-nrw/project-phoenix/modules/delivery"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/services"
)

// PlatformWiringOwners are the owners the root has composed when it builds
// the platform, Communication and Organisation & Tenancy wiring of the
// legacy service graph (#4013).
type PlatformWiringOwners struct {
	DB            transactionruntime.Database
	Organizations organizationtenancy.Capability
	Persons       peopledirectory.Capability
	Groups        schoolstructure.Query
	Membership    schoolmembership.Capability
	// People is the People Directory wiring the root hands the graph; the
	// parent portal and the operator provisioning read the same rows.
	People services.PeopleWiring
	// Now fixes the parent announcement clock; nil is the Berlin clock.
	Now func() time.Time
}

// NewPlatformWiring builds the values over the owners and the adapters the
// legacy service graph binds the platform owners, Communication and
// Organisation & Tenancy through. The graph takes the result in place of
// building it itself (ADR 0053 point 8).
func NewPlatformWiring(owners PlatformWiringOwners) (services.PlatformWiring, error) {
	if owners.Organizations == nil || owners.Persons == nil || owners.Groups == nil || owners.Membership == nil ||
		owners.People.Identity == nil || owners.People.Persons == nil || owners.People.Staff == nil {
		return services.PlatformWiring{}, errors.New("platform wiring: the Organisation & Tenancy, People Directory, School Structure, School Membership and Identity & Access owners and the person and staff rows are required")
	}
	fleet, err := NewDeviceFleet(owners.DB)
	if err != nil {
		return services.PlatformWiring{}, fmt.Errorf("platform wiring: compose device fleet: %w", err)
	}
	auditLog := NewOperatorAuditLog(owners.DB)
	provisioning := operatorProvisioningOwners{
		DB: owners.DB, Organizations: owners.Organizations, Persons: owners.Persons, Membership: owners.Membership,
		PersonRows: owners.People.Persons, StaffRows: owners.People.Staff, AuditLog: auditLog,
	}
	parentPortal := NewParentPortalRepositories(owners.DB, ParentPortalOwners{
		People: owners.Persons, Groups: owners.Groups, Membership: owners.Membership,
		Identity: owners.People.Identity, Enrollment: enrollmentCompose.New(), Now: owners.Now,
	}).WithSchools(owners.Organizations)
	return services.PlatformWiring{
		SchoolSettings:   NewSchoolSettingsStore(owners.Organizations),
		OperatorAuditLog: auditLog,
		ParentPortal: services.ParentPortalRepositories{
			ParentPortalRepositories: parentPortal.ParentPortalRepositories,
			MessagingRepositories:    parentPortal.MessagingRepositories,
		},
		DeviceFleet: fleet,
		ListExport:  lists.NewRendererService(),

		DataAccessAudit:            NewDataAccessAudit,
		DeletionAudit:              NewDeletionAudit,
		TimeTrackingRetentionAudit: NewTimeTrackingRetentionAudit,
		GuardianDisplays: func(query services.GuardianDisplayQuery) deliveryCompose.GuardianDisplayResolver {
			return GuardianDisplayResolver{Query: query}
		},
		AnnouncementDeliveries: func(module *deliveryModule.Module) communicationCompose.ParentAnnouncementDeliveryRecorder {
			return announcementDeliveryAdapter{module: module}
		},
		EnrollmentDeliveries: func(module *deliveryModule.Module) enrollmentCompose.EnrollmentDeletionDelivery {
			return enrollmentDeliveryAdapter{module: module}
		},
		OperatorProvisioning: provisioning.compose,
		PWAUsageCounts: func(provisioning organizationtenancy.Provisioning) deliveryCompose.PWAUsageCounts {
			return pwaUsageCounts{provisioning: provisioning}
		},
	}, nil
}
