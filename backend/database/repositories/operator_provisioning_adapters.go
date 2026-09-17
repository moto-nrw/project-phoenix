package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	organizationCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	activeModels "github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/models/active"
	"github.com/uptrace/bun"
)

// deviceAPIKeyConstraint is the unique constraint PostgreSQL generated for
// iot.devices.api_key (migration 001003009).
const deviceAPIKeyConstraint = "devices_api_key_key"

// OperatorProvisioningDependencies are the owner capabilities and retained
// repositories the Organisation & Tenancy provisioning seams (#3253) bind to.
type OperatorProvisioningDependencies struct {
	DB           *bun.DB
	Devices      devicefleet.Capability
	Persons      peopledirectory.Query
	Membership   schoolmembership.Query
	PersonRepo   userModels.PersonRepository
	StaffRepo    userModels.StaffRepository
	Accounts     authModels.AccountRepository
	ActiveGroups interface {
		FindActiveByDeviceIDWithNames(ctx context.Context, deviceID int64) (*activeModels.Group, error)
	}
	Supervisors activeModels.GroupSupervisorRepository
	Categories  activitiesModels.CategoryRepository
	AuditLog    platformModels.OperatorAuditLogRepository
}

// OperatorProvisioningAdapters bind the provisioning seams to the retained
// owners the root still composes.
type OperatorProvisioningAdapters struct {
	Devices    organizationCompose.ProvisioningDevices
	People     organizationCompose.ProvisioningPeople
	Presence   organizationCompose.ProvisioningPresence
	Categories organizationCompose.ProvisioningCategories
	Audit      organizationCompose.OperatorAudit
}

// NewOperatorProvisioningAdapters binds the provisioning seams.
func NewOperatorProvisioningAdapters(deps OperatorProvisioningDependencies) (OperatorProvisioningAdapters, error) {
	if deps.DB == nil || deps.Devices == nil || deps.Persons == nil || deps.Membership == nil || deps.PersonRepo == nil ||
		deps.StaffRepo == nil || deps.Accounts == nil || deps.ActiveGroups == nil || deps.Supervisors == nil ||
		deps.Categories == nil || deps.AuditLog == nil {
		return OperatorProvisioningAdapters{}, errors.New("operator provisioning adapters: all dependencies are required")
	}
	return OperatorProvisioningAdapters{
		Devices: provisioningDevices{devices: deps.Devices},
		People: provisioningPeople{
			persons: deps.Persons, membership: deps.Membership, personRepo: deps.PersonRepo,
			staffRepo: deps.StaffRepo, accounts: deps.Accounts, db: deps.DB,
		},
		Presence:   provisioningPresence{groups: deps.ActiveGroups, supervisors: deps.Supervisors},
		Categories: provisioningCategories{categories: deps.Categories},
		Audit:      provisioningAudit{log: deps.AuditLog},
	}, nil
}

// --- Device Fleet ------------------------------------------------------------

type provisioningDevices struct{ devices devicefleet.Capability }

func (d provisioningDevices) CountDevicesByTenant(ctx context.Context) (map[int64]int, error) {
	return d.devices.CountDevicesByTenant(ctx)
}

func (d provisioningDevices) ListDevicesByTenant(ctx context.Context, tenantIDs []int64) ([]organizationCompose.ProvisioningDevice, error) {
	devices, err := d.devices.ListDevicesByTenant(ctx, tenantIDs)
	if err != nil {
		return nil, err
	}
	result := make([]organizationCompose.ProvisioningDevice, 0, len(devices))
	for _, device := range devices {
		result = append(result, provisioningDevice(device))
	}
	return result, nil
}

func (d provisioningDevices) FindDeviceForUpdate(ctx context.Context, id int64) (organizationCompose.ProvisioningDevice, bool, error) {
	device, err := d.devices.FindDeviceForUpdate(ctx, id)
	if errors.Is(err, devicefleet.ErrDeviceNotFound) {
		return organizationCompose.ProvisioningDevice{}, false, nil
	}
	if err != nil {
		return organizationCompose.ProvisioningDevice{}, false, err
	}
	return provisioningDevice(device), true, nil
}

func (d provisioningDevices) CreateDevice(ctx context.Context, device organizationCompose.NewProvisioningDevice) (organizationCompose.ProvisioningDevice, error) {
	created, err := d.devices.CreateDevice(usersRepo.WithTenantID(ctx, device.TenantID), devicefleet.CreateDevice{
		DeviceID: device.DeviceID, DeviceType: device.DeviceType, Name: device.Name,
		Status: devicefleet.DeviceStatus(device.Status), APIKey: device.APIKey,
	})
	if err != nil {
		return organizationCompose.ProvisioningDevice{}, deviceWriteError(err)
	}
	return provisioningDevice(created), nil
}

func (d provisioningDevices) UpdateDevice(ctx context.Context, device organizationCompose.ProvisioningDevice) error {
	_, err := d.devices.UpdateDevice(ctx, devicefleet.UpdateDevice{
		ID: device.ID, DeviceID: device.DeviceID, DeviceType: device.DeviceType, Name: device.Name,
		Status: devicefleet.DeviceStatus(device.Status), APIKey: device.APIKey, LastSeen: device.LastSeen,
		RegisteredByID: device.RegisteredByID, RoomID: device.RoomID, ArchivedAt: device.ArchivedAt,
		TransferredToDeviceID: device.TransferredToDeviceID,
	})
	if err != nil {
		return deviceWriteError(err)
	}
	return nil
}

func (d provisioningDevices) DeleteDevice(ctx context.Context, id int64) error {
	err := d.devices.DeleteDevice(ctx, id)
	if isForeignKeyViolation(err) {
		return fmt.Errorf("%w: %w", organizationCompose.ErrDeviceReferenced, err)
	}
	return err
}

func provisioningDevice(device devicefleet.Device) organizationCompose.ProvisioningDevice {
	return organizationCompose.ProvisioningDevice{
		ID: device.ID, TenantID: device.TenantID, CreatedAt: device.CreatedAt, UpdatedAt: device.UpdatedAt,
		DeviceID: device.DeviceID, DeviceType: device.DeviceType, Name: device.Name, Status: string(device.Status),
		APIKey: device.APIKey, LastSeen: device.LastSeen, RegisteredByID: device.RegisteredByID, RoomID: device.RoomID,
		ArchivedAt: device.ArchivedAt, TransferredToDeviceID: device.TransferredToDeviceID,
	}
}

// deviceWriteError names the unique violations provisioning reacts to: the
// API key constraint and, for any other one, the per-school device ID.
func deviceWriteError(err error) error {
	switch {
	case usersRepo.IsUniqueViolationOn(err, deviceAPIKeyConstraint):
		return fmt.Errorf("%w: %w", organizationCompose.ErrDeviceAPIKeyTaken, err)
	case usersRepo.IsUniqueViolation(err):
		return fmt.Errorf("%w: %w", organizationCompose.ErrDeviceIDTaken, err)
	default:
		return err
	}
}

// isForeignKeyViolation reports PostgreSQL error 23503: attendance or
// session rows still reference the device (ON DELETE RESTRICT).
func isForeignKeyViolation(err error) bool {
	var postgresError interface {
		error
		Field(byte) string
	}
	return errors.As(err, &postgresError) && postgresError.Field('C') == "23503"
}

// --- People Directory and School Membership ---------------------------------

type provisioningPeople struct {
	persons    peopledirectory.Query
	membership schoolmembership.Query
	personRepo userModels.PersonRepository
	staffRepo  userModels.StaffRepository
	accounts   authModels.AccountRepository
	db         *bun.DB
}

func (p provisioningPeople) CountPersonsByTenant(ctx context.Context) (map[int64]int, error) {
	return p.persons.CountPersonsByTenant(ctx)
}

// ListPersons lists the persons of the schools with their staff, student and
// account facts. Staff membership belongs to School Membership; the listing
// asks the owner instead of joining users.staff.
func (p provisioningPeople) ListPersons(ctx context.Context, tenantIDs []int64) ([]organizationCompose.PersonListing, error) {
	persons, err := p.persons.ListPersonsByTenantIDs(ctx, tenantIDs)
	if err != nil || len(persons) == 0 {
		return []organizationCompose.PersonListing{}, err
	}
	personIDs, accountIDs := operatorPersonIDs(persons)
	students, err := usersRepo.FindOperatorPersonStudentMembership(ctx, p.db, personIDs)
	if err != nil {
		return nil, err
	}
	staff := make(map[int64]bool, len(personIDs))
	members, err := p.membership.ListStaff(ctx, schoolmembership.StaffFilter{PersonIDs: personIDs})
	if err != nil {
		return nil, fmt.Errorf("load operator staff membership: %w", err)
	}
	for _, member := range members {
		staff[member.PersonID] = true
	}
	emails, err := p.accounts.FindEmailsByAccountIDs(ctx, accountIDs)
	if err != nil {
		return nil, fmt.Errorf("load operator account emails: %w", err)
	}
	result := make([]organizationCompose.PersonListing, 0, len(persons))
	for _, person := range persons {
		listing := organizationCompose.PersonListing{
			ID: person.ID, TenantID: person.TenantID, FirstName: person.FirstName, LastName: person.LastName,
			HasAccount: person.AccountID != nil, HasRFIDCard: person.TagID != nil,
			IsStaff: staff[person.ID], IsStudent: students[person.ID], CreatedAt: person.CreatedAt,
		}
		if person.AccountID != nil {
			if email, found := emails[*person.AccountID]; found {
				listing.AccountEmail = &email
			}
		}
		result = append(result, listing)
	}
	return result, nil
}

func operatorPersonIDs(persons []peopledirectory.Person) ([]int64, []int64) {
	personIDs := make([]int64, 0, len(persons))
	accountIDs := make([]int64, 0, len(persons))
	for _, person := range persons {
		personIDs = append(personIDs, person.ID)
		if person.AccountID != nil {
			accountIDs = append(accountIDs, *person.AccountID)
		}
	}
	return personIDs, accountIDs
}

// FindPerson reads one non-deleted person of any school. The administrative
// context carries no tenant, so the repository's tenant filter is a no-op.
func (p provisioningPeople) FindPerson(ctx context.Context, id int64) (organizationCompose.ProvisioningPerson, bool, error) {
	person, err := p.personRepo.FindByID(ctx, id)
	if usersRepo.IsNotFound(err) {
		return organizationCompose.ProvisioningPerson{}, false, nil
	}
	if err != nil {
		return organizationCompose.ProvisioningPerson{}, false, err
	}
	return organizationCompose.ProvisioningPerson{
		ID: person.ID, TenantID: person.TenantID, AccountID: person.AccountID, HasRFIDCard: person.TagID != nil,
	}, true, nil
}

func (p provisioningPeople) FindStaff(ctx context.Context, personID int64) (organizationCompose.StaffMember, bool, error) {
	staff, err := p.staffRepo.FindByPersonID(ctx, personID)
	if err != nil || staff == nil {
		return organizationCompose.StaffMember{}, false, err
	}
	return organizationCompose.StaffMember{ID: staff.ID, TenantID: staff.TenantID}, true, nil
}

func (p provisioningPeople) UnlinkRFIDCard(ctx context.Context, personID int64) error {
	return p.personRepo.UnlinkFromRFIDCard(ctx, personID)
}

func (p provisioningPeople) UnlinkAccount(ctx context.Context, personID int64) error {
	return p.personRepo.UnlinkFromAccount(ctx, personID)
}

func (p provisioningPeople) AnonymizeAndSoftDelete(ctx context.Context, personID int64) error {
	return p.personRepo.AnonymizeAndSoftDelete(ctx, personID)
}

// --- Student Presence --------------------------------------------------------

type provisioningPresence struct {
	groups interface {
		FindActiveByDeviceIDWithNames(ctx context.Context, deviceID int64) (*activeModels.Group, error)
	}
	supervisors activeModels.GroupSupervisorRepository
}

// ActiveDeviceSession reads the device's open kiosk session under the
// device's school, which the presence owner requires for the read.
func (p provisioningPresence) ActiveDeviceSession(ctx context.Context, tenantID, deviceID int64) (*organizationCompose.DeviceSession, error) {
	group, err := p.groups.FindActiveByDeviceIDWithNames(usersRepo.WithTenantID(ctx, tenantID), deviceID)
	if err != nil || group == nil {
		return nil, err
	}
	session := &organizationCompose.DeviceSession{ID: group.ID, StartedAt: group.StartTime}
	if group.ActualGroup != nil {
		session.ActivityName = &group.ActualGroup.Name
	}
	if group.Room != nil {
		session.RoomName = &group.Room.Name
	}
	return session, nil
}

// CountActiveSupervisions reads under the staff member's school: the
// operator transaction is cross-tenant, but the presence owner requires it.
func (p provisioningPresence) CountActiveSupervisions(ctx context.Context, tenantID, staffID int64) (int, error) {
	supervisors, err := p.supervisors.FindActiveByStaffID(usersRepo.WithTenantID(ctx, tenantID), staffID)
	return len(supervisors), err
}

// --- Timetable & Activities --------------------------------------------------

type provisioningCategories struct {
	categories activitiesModels.CategoryRepository
}

// SeedCategories creates the categories at the school; one the school
// already has is skipped.
func (c provisioningCategories) SeedCategories(ctx context.Context, tenantID int64, categories []organizationCompose.ActivityCategory) error {
	if tenantID <= 0 {
		return nil
	}
	schoolCtx := usersRepo.WithTenantID(ctx, tenantID)
	for _, value := range categories {
		category := &activitiesModels.Category{Name: value.Name, Description: value.Description, Color: value.Color}
		if err := c.categories.Create(schoolCtx, category); err != nil {
			if usersRepo.IsUniqueViolation(err) {
				continue
			}
			return err
		}
	}
	return nil
}

// --- Audit -------------------------------------------------------------------

type provisioningAudit struct {
	log platformModels.OperatorAuditLogRepository
}

func (a provisioningAudit) RecordOperatorAction(ctx context.Context, entry organizationCompose.OperatorAuditEntry) error {
	return a.log.Create(ctx, &platformModels.OperatorAuditLog{
		OperatorID: entry.OperatorID, Action: entry.Action, ResourceType: entry.ResourceType,
		ResourceID: entry.ResourceID, RequestIP: entry.ClientIP, Changes: json.RawMessage(entry.Changes),
	})
}
