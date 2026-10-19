package owners

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	organizationCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"

	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	schoolMembershipCompose "github.com/moto-nrw/project-phoenix/modules/schoolmembership/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/transactionruntime"
)

// deviceAPIKeyConstraint is the unique constraint PostgreSQL generated for
// iot.devices.api_key (migration 001003009).
const deviceAPIKeyConstraint = "devices_api_key_key"

// accountEmailQuery is the operator person listing's account-facts port.
type accountEmailQuery interface {
	ListAccountEmails(context.Context, []int64) (map[int64]string, error)
	CountActiveAccountsBySchoolGroups(context.Context, map[int64][]int64) (map[int64]int, error)
}

// operatorPersonStudents is the operator person listing's student-facts port.
type operatorPersonStudents func(ctx context.Context, personIDs []int64) (map[int64]bool, error)

// provisioningPersonRows reads the person a deletion removes.
type provisioningPersonRows interface {
	FindByID(ctx context.Context, id any) (*peopledirectory.DirectoryPersonSnapshot, error)
}

// ProvisioningPersonDirectory is the People Directory the operator person
// listing reads and a person's deletion writes through.
type ProvisioningPersonDirectory interface {
	peopledirectory.Query
	peopledirectory.PersonRemovalCommand
	UnlinkTag(ctx context.Context, personID int64) error
	UnlinkAccount(ctx context.Context, personID int64) error
}

// provisioningStaffRows reads the staff row of a person.
type provisioningStaffRows interface {
	FindByPersonID(ctx context.Context, personID int64) (*peopledirectory.Staff, error)
}

// OperatorProvisioningAdapters bind the provisioning seams to the retained
// owners the root still composes.
type OperatorProvisioningAdapters struct {
	Devices    organizationCompose.ProvisioningDevices
	People     organizationCompose.ProvisioningPeople
	Presence   organizationCompose.ProvisioningPresence
	Categories organizationCompose.ProvisioningCategories
	Audit      organizationCompose.OperatorAudit
	// ActiveAccountCounts supplies bounded dashboard account aggregates.
	ActiveAccountCounts func(context.Context, map[int64][]int64) (map[int64]int, error)
}

// NewOperatorProvisioningAdapters binds the provisioning seams (#3253) to the
// owner capabilities and retained rows. personStudents marks which persons of
// the operator listing are students (peopleCompose.LegacyRepositoryRuntime
// over the student directory projection). Every dependency is required.
func NewOperatorProvisioningAdapters(
	personStudents operatorPersonStudents, devices devicefleet.Capability, persons ProvisioningPersonDirectory,
	membership schoolmembership.Query, personRows provisioningPersonRows, staffRows provisioningStaffRows,
	accounts accountEmailQuery, activeGroups interface {
		FindActiveByDeviceIDWithNames(ctx context.Context, deviceID int64) (*studentpresence.SessionDetail, error)
	}, supervisors interface {
		FindActiveByStaffID(ctx context.Context, staffID int64) ([]*studentpresence.GroupSupervision, error)
	}, categories OperatorCategoryCreator, auditLog OperatorAuditEntries,
) (OperatorProvisioningAdapters, error) {
	if personStudents == nil || devices == nil || persons == nil || membership == nil || personRows == nil ||
		staffRows == nil || accounts == nil || activeGroups == nil || supervisors == nil ||
		categories == nil || auditLog == nil {
		return OperatorProvisioningAdapters{}, errors.New("operator provisioning adapters: all dependencies are required")
	}
	return OperatorProvisioningAdapters{
		Devices: provisioningDevices{devices: devices},
		People: provisioningPeople{
			persons: persons, membership: membership, childQuota: schoolMembershipCompose.NewChildQuotaCounts(),
			personRows: personRows,
			staffRows:  staffRows, accounts: accounts, students: personStudents,
		},
		Presence:            provisioningPresence{groups: activeGroups, supervisors: supervisors},
		Categories:          provisioningCategories{categories: categories},
		Audit:               provisioningAudit{log: auditLog},
		ActiveAccountCounts: accounts.CountActiveAccountsBySchoolGroups,
	}, nil
}

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
	created, err := d.devices.CreateDevice(peopleCompose.WithCallerTenant(ctx, device.TenantID), devicefleet.CreateDevice{
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
	case transactionruntime.IsUniqueViolationOn(err, deviceAPIKeyConstraint):
		return fmt.Errorf("%w: %w", organizationCompose.ErrDeviceAPIKeyTaken, err)
	case transactionruntime.IsUniqueViolation(err):
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

type provisioningPeople struct {
	persons    provisioningPersons
	membership schoolmembership.Query
	childQuota schoolmembership.ChildQuotaCounts
	personRows provisioningPersonRows
	staffRows  provisioningStaffRows
	accounts   accountEmailQuery
	students   operatorPersonStudents
}

// provisioningPersons are the People Directory reads and writes the operator
// person listing and a person's deletion call.
type provisioningPersons interface {
	CountPersonsByTenant(ctx context.Context) (map[int64]int, error)
	ListPersonsByTenantIDs(ctx context.Context, tenantIDs []int64) ([]peopledirectory.Person, error)
	UnlinkTag(ctx context.Context, personID int64) error
	UnlinkAccount(ctx context.Context, personID int64) error
	AnonymizeAndSoftDeletePerson(ctx context.Context, personID int64) error
}

func (p provisioningPeople) CountPersonsByTenant(ctx context.Context) (map[int64]int, error) {
	return p.persons.CountPersonsByTenant(ctx)
}

// CountChildQuotaByTenant asks School Membership, which owns the
// Kontingentzahl rule (#3568).
func (p provisioningPeople) CountChildQuotaByTenant(ctx context.Context) (map[int64]int, error) {
	return p.childQuota.CountChildQuotaByTenant(ctx)
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
	students, err := p.students(ctx, personIDs)
	if err != nil {
		return nil, err
	}
	staff := make(map[int64]bool, len(personIDs))
	members, err := p.membership.ListStaff(ctx, schoolmembership.StaffFilter{PersonIDs: personIDs, MembershipOnly: true})
	if err != nil {
		return nil, fmt.Errorf("load operator staff membership: %w", err)
	}
	for _, member := range members {
		staff[member.PersonID] = true
	}
	emails, err := p.accounts.ListAccountEmails(ctx, accountIDs)
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
	person, err := p.personRows.FindByID(ctx, id)
	if peoplePorts.IsRepositoryNoRows(err) {
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
	staff, err := p.staffRows.FindByPersonID(ctx, personID)
	if err != nil || staff == nil {
		return organizationCompose.StaffMember{}, false, err
	}
	return organizationCompose.StaffMember{ID: staff.ID, TenantID: staff.TenantID}, true, nil
}

func (p provisioningPeople) UnlinkRFIDCard(ctx context.Context, personID int64) error {
	return p.persons.UnlinkTag(ctx, personID)
}

func (p provisioningPeople) UnlinkAccount(ctx context.Context, personID int64) error {
	return p.persons.UnlinkAccount(ctx, personID)
}

func (p provisioningPeople) AnonymizeAndSoftDelete(ctx context.Context, personID int64) error {
	return p.persons.AnonymizeAndSoftDeletePerson(ctx, personID)
}

type provisioningPresence struct {
	groups interface {
		FindActiveByDeviceIDWithNames(ctx context.Context, deviceID int64) (*studentpresence.SessionDetail, error)
	}
	supervisors interface {
		FindActiveByStaffID(ctx context.Context, staffID int64) ([]*studentpresence.GroupSupervision, error)
	}
}

// ActiveDeviceSession reads the device's open kiosk session under the
// device's school, which the presence owner requires for the read.
func (p provisioningPresence) ActiveDeviceSession(ctx context.Context, tenantID, deviceID int64) (*organizationCompose.DeviceSession, error) {
	group, err := p.groups.FindActiveByDeviceIDWithNames(peopleCompose.WithCallerTenant(ctx, tenantID), deviceID)
	if err != nil || group == nil {
		return nil, err
	}
	session := &organizationCompose.DeviceSession{ID: group.ID, StartedAt: group.StartTime}
	if group.Activity != nil {
		session.ActivityName = &group.Activity.Name
	}
	if group.Room != nil {
		session.RoomName = &group.Room.Name
	}
	return session, nil
}

// CountActiveSupervisions reads under the staff member's school: the
// operator transaction is cross-tenant, but the presence owner requires it.
func (p provisioningPresence) CountActiveSupervisions(ctx context.Context, tenantID, staffID int64) (int, error) {
	supervisors, err := p.supervisors.FindActiveByStaffID(peopleCompose.WithCallerTenant(ctx, tenantID), staffID)
	return len(supervisors), err
}

type provisioningCategories struct {
	categories OperatorCategoryCreator
}

// OperatorCategoryCreator creates one activity category at the school in
// ctx; a category the school already has fails with a unique violation.
type OperatorCategoryCreator interface {
	CreateCategory(ctx context.Context, name, description, color string) error
}

// SeedCategories creates the categories at the school; one the school
// already has is skipped.
func (c provisioningCategories) SeedCategories(ctx context.Context, tenantID int64, categories []organizationCompose.ActivityCategory) error {
	if tenantID <= 0 {
		return nil
	}
	schoolCtx := peopleCompose.WithCallerTenant(ctx, tenantID)
	for _, value := range categories {
		if err := c.categories.CreateCategory(schoolCtx, value.Name, value.Description, value.Color); err != nil {
			if transactionruntime.IsUniqueViolation(err) {
				continue
			}
			return err
		}
	}
	return nil
}

type provisioningAudit struct {
	log OperatorAuditEntries
}

func (a provisioningAudit) RecordOperatorAction(ctx context.Context, entry organizationCompose.OperatorAuditEntry) error {
	return a.log.Create(ctx, &auditModels.OperatorAuditEntry{
		OperatorID: entry.OperatorID, Action: entry.Action, ResourceType: entry.ResourceType,
		ResourceID: entry.ResourceID, RequestIP: entry.ClientIP, Changes: json.RawMessage(entry.Changes),
	})
}
