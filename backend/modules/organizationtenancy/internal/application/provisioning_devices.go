package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
)

const (
	// listedDeviceOnlineWindow is the online cutoff of the device listings.
	listedDeviceOnlineWindow = 5 * time.Minute
	// defaultDeviceOnlineWindow is the transfer preflight's cutoff when the
	// school's iot.device_online_window_minutes setting cannot be resolved
	// (#586).
	defaultDeviceOnlineWindow = 5 * time.Minute
	maxAPIKeyRetries          = 3
	webManualDeviceReason     = "system device required for manual web check-ins"
)

// deviceFilter narrows the operator device listing. At most one field is set.
type deviceFilter struct {
	deviceRowID    *int64
	schoolID       *int64
	organizationID *int64
}

// ListAllDevices lists the devices of every live school.
func (p *Provisioning) ListAllDevices(ctx context.Context) ([]organizationtenancy.OperatorDevice, error) {
	return adminValue(ctx, p, func(adminCtx context.Context) ([]organizationtenancy.OperatorDevice, error) {
		return p.listDevices(adminCtx, deviceFilter{})
	})
}

// ListSchoolDevices lists the devices of a non-deleted school.
func (p *Provisioning) ListSchoolDevices(ctx context.Context, schoolID int64) ([]organizationtenancy.OperatorDevice, error) {
	return adminValue(ctx, p, func(adminCtx context.Context) ([]organizationtenancy.OperatorDevice, error) {
		if _, err := p.findLiveSchool(adminCtx, schoolID); err != nil {
			return nil, err
		}
		return p.listDevices(adminCtx, deviceFilter{schoolID: &schoolID})
	})
}

// ListOrganizationDevices lists the devices of a non-deleted organisation.
func (p *Provisioning) ListOrganizationDevices(ctx context.Context, organizationID int64) ([]organizationtenancy.OperatorDevice, error) {
	return adminValue(ctx, p, func(adminCtx context.Context) ([]organizationtenancy.OperatorDevice, error) {
		organization, err := p.organizations.FindOrganization(adminCtx, organizationID)
		if err != nil {
			return nil, mapOrganizationError(err, organizationID)
		}
		if organization.IsDeleted() {
			return nil, &organizationtenancy.OrganizationDeletedError{OrganizationID: organizationID}
		}
		return p.listDevices(adminCtx, deviceFilter{organizationID: &organizationID})
	})
}

// listDevices assembles the operator device listing from Device Fleet and the
// school and organization directory. Devices of a soft-deleted school or organisation
// are never listed, so global listings do not surface Papierkorb tenants.
func (p *Provisioning) listDevices(ctx context.Context, filter deviceFilter) ([]organizationtenancy.OperatorDevice, error) {
	schools, err := p.organizations.ListSchools(ctx)
	if err != nil {
		return nil, fmt.Errorf("load schools for operator device listing: %w", err)
	}
	organizations, err := p.organizations.ListOrganizations(ctx)
	if err != nil {
		return nil, fmt.Errorf("load organizations for operator device listing: %w", err)
	}
	liveOrganizations := make(map[int64]string, len(organizations))
	for _, organization := range organizations {
		if organization.DeletedAt == nil {
			liveOrganizations[organization.ID] = organization.Name
		}
	}
	visible := make(map[int64]domain.SchoolSummary, len(schools))
	tenantIDs := make([]int64, 0, len(schools))
	for _, school := range schools {
		organizationName, live := liveOrganizations[school.OrganizationID]
		if school.DeletedAt != nil || !live {
			continue
		}
		if filter.schoolID != nil && school.ID != *filter.schoolID {
			continue
		}
		if filter.organizationID != nil && school.OrganizationID != *filter.organizationID {
			continue
		}
		visible[school.ID] = domain.SchoolSummary{ID: school.ID, Name: school.Name, OrganizationID: school.OrganizationID, OrganizationName: organizationName}
		tenantIDs = append(tenantIDs, school.ID)
	}
	if len(tenantIDs) == 0 {
		return []organizationtenancy.OperatorDevice{}, nil
	}
	devices, err := p.devices.ListDevicesByTenant(ctx, tenantIDs)
	if err != nil {
		return nil, fmt.Errorf("list devices for operator device listing: %w", err)
	}
	result := make([]organizationtenancy.OperatorDevice, 0, len(devices))
	for _, device := range devices {
		if filter.deviceRowID != nil && device.ID != *filter.deviceRowID {
			continue
		}
		school, found := visible[device.TenantID]
		if !found {
			continue
		}
		result = append(result, operatorDevice(device, school))
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].OrganizationName != result[j].OrganizationName {
			return result[i].OrganizationName < result[j].OrganizationName
		}
		if result[i].SchoolName != result[j].SchoolName {
			return result[i].SchoolName < result[j].SchoolName
		}
		return result[i].DeviceID < result[j].DeviceID
	})
	return result, nil
}

func operatorDevice(device domain.Device, school domain.SchoolSummary) organizationtenancy.OperatorDevice {
	view := organizationtenancy.OperatorDevice{
		ID: device.ID, DeviceID: device.DeviceID, DeviceType: device.DeviceType,
		Name: device.Name, Status: device.Status, APIKey: device.APIKey,
		MaskedAPIKey: maskAPIKey(device.APIKey), LastSeen: device.LastSeen,
		SchoolID: school.ID, SchoolName: school.Name,
		OrganizationID: school.OrganizationID, OrganizationName: school.OrganizationName,
		CreatedAt: device.CreatedAt, UpdatedAt: device.UpdatedAt,
	}
	if device.LastSeen != nil {
		view.IsOnline = time.Since(*device.LastSeen) <= listedDeviceOnlineWindow
	}
	return view
}

// maskAPIKey returns the first 10 characters followed by an ellipsis.
func maskAPIKey(key *string) string {
	if key == nil || *key == "" {
		return ""
	}
	if len(*key) <= 10 {
		return *key
	}
	return (*key)[:10] + "..."
}

// listedDevice re-reads one device after a write for the response.
func (p *Provisioning) listedDevice(ctx context.Context, op string, deviceRowID int64) (*organizationtenancy.OperatorDevice, error) {
	devices, err := p.listDevices(ctx, deviceFilter{deviceRowID: &deviceRowID})
	if err != nil {
		return nil, fmt.Errorf("%s: re-query failed: %w", op, err)
	}
	if len(devices) == 0 {
		p.logger.Error("device not found after successful write",
			slog.String("op", op),
			slog.Int64("device_row_id", deviceRowID),
		)
		return nil, fmt.Errorf("%s: device not found after write (inconsistent state)", op)
	}
	return &devices[0], nil
}

// apiKey returns the operator's key, or generates one when none was given.
func (p *Provisioning) apiKey(provided *string) (string, error) {
	if provided != nil && *provided != "" {
		return *provided, nil
	}
	return p.secrets.APIKey()
}

// CreateDevice registers a device at an active school. A generated API key
// that collides is regenerated; an operator-supplied one is a conflict.
func (p *Provisioning) CreateDevice(ctx context.Context, schoolID int64, deviceID, deviceType string, name, apiKey *string, operatorID int64, clientIP net.IP) (*organizationtenancy.OperatorDevice, error) {
	if schoolID <= 0 {
		return nil, invalidData("school_id is required")
	}
	if strings.TrimSpace(deviceID) == "" {
		return nil, invalidData("device_id is required")
	}
	if strings.TrimSpace(deviceType) == "" {
		return nil, invalidData("device_type is required")
	}
	manual := apiKey != nil && *apiKey != ""
	return adminValue(ctx, p, func(adminCtx context.Context) (*organizationtenancy.OperatorDevice, error) {
		if _, err := p.findOperableSchool(adminCtx, schoolID); err != nil {
			return nil, err
		}
		device, err := p.createDeviceWithKey(adminCtx, domain.NewDevice{
			TenantID: schoolID, DeviceID: strings.TrimSpace(deviceID), DeviceType: strings.TrimSpace(deviceType),
			Name: name, Status: domain.DeviceStatusActive,
		}, apiKey, manual)
		if err != nil {
			return nil, err
		}
		p.logAction(adminCtx, operatorID, domain.AuditActionCreate, domain.AuditResourceDevice, &device.ID, clientIP, map[string]any{
			"device_id":    device.DeviceID,
			"device_type":  device.DeviceType,
			"school_id":    schoolID,
			"api_key_mode": apiKeyMode(manual),
		})
		return p.listedDevice(adminCtx, "CreateDevice", device.ID)
	})
}

func (p *Provisioning) createDeviceWithKey(ctx context.Context, device domain.NewDevice, apiKey *string, manual bool) (domain.Device, error) {
	for attempt := 0; attempt < maxAPIKeyRetries; attempt++ {
		key, err := p.apiKey(apiKey)
		if err != nil {
			return domain.Device{}, fmt.Errorf("CreateDevice: generate API key: %w", err)
		}
		device.APIKey = &key
		created, err := p.devices.CreateDevice(ctx, device)
		switch {
		case err == nil:
			return created, nil
		case errors.Is(err, domain.ErrDeviceAPIKeyTaken):
			if manual {
				return domain.Device{}, conflict("api_key already in use")
			}
		case errors.Is(err, domain.ErrDeviceIDTaken):
			return domain.Device{}, conflict("device_id already exists for this school")
		default:
			return domain.Device{}, err
		}
	}
	return domain.Device{}, fmt.Errorf("CreateDevice: failed to generate unique API key after %d attempts", maxAPIKeyRetries)
}

// SetDeviceAPIKey rotates the API key of a device at an active school.
func (p *Provisioning) SetDeviceAPIKey(ctx context.Context, id int64, apiKey *string, operatorID int64, clientIP net.IP) (*organizationtenancy.OperatorDevice, error) {
	if id <= 0 {
		return nil, invalidData("device id is required")
	}
	manual := apiKey != nil && *apiKey != ""
	return adminValue(ctx, p, func(adminCtx context.Context) (*organizationtenancy.OperatorDevice, error) {
		// Load and lock the device without its tenant-scoped room projection.
		device, err := p.findDevice(adminCtx, id)
		if err != nil {
			return nil, err
		}
		// Reject key rotation for devices of unusable schools.
		if _, err := p.findOperableSchool(adminCtx, device.TenantID); err != nil {
			var notFound *organizationtenancy.SchoolNotFoundError
			if !errors.As(err, &notFound) && !isSchoolStateError(err) {
				return nil, fmt.Errorf("SetDeviceAPIKey: lookup school: %w", err)
			}
			return nil, err
		}
		if err := p.rotateDeviceKey(adminCtx, device, apiKey, manual); err != nil {
			return nil, err
		}
		p.logAction(adminCtx, operatorID, domain.AuditActionRotateAPIKey, domain.AuditResourceDevice, &id, clientIP, map[string]any{
			"device_id":    device.DeviceID,
			"school_id":    device.TenantID,
			"api_key_mode": apiKeyMode(manual),
		})
		return p.listedDevice(adminCtx, "SetDeviceAPIKey", id)
	})
}

func (p *Provisioning) rotateDeviceKey(ctx context.Context, device domain.Device, apiKey *string, manual bool) error {
	for attempt := 0; attempt < maxAPIKeyRetries; attempt++ {
		key, err := p.apiKey(apiKey)
		if err != nil {
			return fmt.Errorf("SetDeviceAPIKey: generate API key: %w", err)
		}
		device.APIKey = &key
		err = p.devices.UpdateDevice(ctx, device)
		switch {
		case err == nil:
			return nil
		case errors.Is(err, domain.ErrDeviceAPIKeyTaken):
			if manual {
				return conflict("api_key already in use")
			}
		default:
			return err
		}
	}
	return fmt.Errorf("SetDeviceAPIKey: failed to generate unique API key after %d attempts", maxAPIKeyRetries)
}

// DeleteDevice removes a device that no attendance or session row
// references. The virtual manual web check-in device cannot be removed.
func (p *Provisioning) DeleteDevice(ctx context.Context, id int64, operatorID int64, clientIP net.IP) error {
	if id <= 0 {
		return invalidData("device id is required")
	}
	return p.inAdmin(ctx, func(adminCtx context.Context) error {
		device, err := p.findDevice(adminCtx, id)
		if err != nil {
			return err
		}
		if device.DeviceID == domain.WebManualDeviceID {
			return &organizationtenancy.DeviceProtectedError{DeviceID: id, Reason: webManualDeviceReason}
		}
		if err := p.devices.DeleteDevice(adminCtx, id); err != nil {
			if errors.Is(err, domain.ErrDeviceReferenced) {
				return &organizationtenancy.DeviceInUseError{DeviceID: id}
			}
			return fmt.Errorf("DeleteDevice: %w", err)
		}
		p.logAction(adminCtx, operatorID, domain.AuditActionDelete, domain.AuditResourceDevice, &id, clientIP, map[string]any{
			"device_id":   device.DeviceID,
			"device_type": device.DeviceType,
			"school_id":   device.TenantID,
		})
		return nil
	})
}

// GetDeviceTransferStatus reports the transfer blockers without changing the
// device.
func (p *Provisioning) GetDeviceTransferStatus(ctx context.Context, id int64) (*organizationtenancy.DeviceTransferStatus, error) {
	if id <= 0 {
		return nil, invalidData("device id is required")
	}
	return adminValue(ctx, p, func(adminCtx context.Context) (*organizationtenancy.DeviceTransferStatus, error) {
		// Keep the source stable while resolving its tenant-scoped session.
		device, err := p.findDevice(adminCtx, id)
		if err != nil {
			return nil, err
		}
		return p.transferStatus(ctx, adminCtx, device)
	})
}

// TransferDevice moves the device identity and API key to another school of
// the same organisation. The source row stays archived for the historical
// foreign keys.
func (p *Provisioning) TransferDevice(ctx context.Context, id, targetSchoolID, operatorID int64, clientIP net.IP) (*organizationtenancy.OperatorDevice, error) {
	if id <= 0 || targetSchoolID <= 0 {
		return nil, invalidData("device id and target_school_id are required")
	}
	return adminValue(ctx, p, func(adminCtx context.Context) (*organizationtenancy.OperatorDevice, error) {
		source, err := p.transferSource(adminCtx, id, targetSchoolID)
		if err != nil {
			return nil, err
		}
		if err := p.validateTransferDestination(adminCtx, source, targetSchoolID); err != nil {
			return nil, err
		}
		if err := p.ensureDeviceTransferable(ctx, adminCtx, source); err != nil {
			return nil, err
		}
		target, err := p.archiveAndTransfer(adminCtx, source, targetSchoolID)
		if err != nil {
			return nil, err
		}
		p.logAction(adminCtx, operatorID, domain.AuditActionTransfer, domain.AuditResourceDevice, &id, clientIP, map[string]any{
			"device_id":        source.DeviceID,
			"source_device_id": id,
			"target_device_id": target.ID,
			"source_school_id": source.TenantID,
			"target_school_id": targetSchoolID,
		})
		return p.listedDevice(adminCtx, "TransferDevice", target.ID)
	})
}

func (p *Provisioning) transferSource(ctx context.Context, id, targetSchoolID int64) (domain.Device, error) {
	source, err := p.findDevice(ctx, id)
	if err != nil {
		return domain.Device{}, err
	}
	if source.DeviceID == domain.WebManualDeviceID {
		return domain.Device{}, &organizationtenancy.DeviceTransferProtectedError{DeviceID: id, Reason: webManualDeviceReason}
	}
	if source.TenantID == targetSchoolID {
		return domain.Device{}, &organizationtenancy.DeviceTransferSameSchoolError{SchoolID: targetSchoolID}
	}
	return source, nil
}

func (p *Provisioning) validateTransferDestination(ctx context.Context, source domain.Device, targetSchoolID int64) error {
	sourceSchool, err := p.findSchool(ctx, source.TenantID)
	if err != nil {
		return transferLookupError("source", err)
	}
	targetSchool, err := p.findOperableSchool(ctx, targetSchoolID)
	if err != nil {
		return transferLookupError("target", err)
	}
	if sourceSchool.OrganizationID != targetSchool.OrganizationID {
		return &organizationtenancy.DeviceTransferOrganizationMismatchError{SourceSchoolID: source.TenantID, TargetSchoolID: targetSchoolID}
	}
	return nil
}

func transferLookupError(side string, err error) error {
	var notFound *organizationtenancy.SchoolNotFoundError
	if errors.As(err, &notFound) || isSchoolStateError(err) {
		return err
	}
	return fmt.Errorf("TransferDevice: lookup %s school: %w", side, err)
}

func (p *Provisioning) ensureDeviceTransferable(ctx, adminCtx context.Context, source domain.Device) error {
	status, err := p.transferStatus(ctx, adminCtx, source)
	if err != nil {
		return err
	}
	if status.IsOnline {
		return &organizationtenancy.DeviceTransferBlockedError{DeviceID: source.ID, Reason: organizationtenancy.DeviceTransferBlockedOnline}
	}
	if status.ActiveSession != nil {
		return &organizationtenancy.DeviceTransferBlockedError{DeviceID: source.ID, Reason: organizationtenancy.DeviceTransferBlockedActiveSession}
	}
	return nil
}

// transferStatus reads the blockers of one device. ctx is the request
// context: the online window resolves in its own tenant transaction, which
// the nested-transaction guard rejects inside the administrative one.
func (p *Provisioning) transferStatus(ctx, adminCtx context.Context, device domain.Device) (*organizationtenancy.DeviceTransferStatus, error) {
	status := &organizationtenancy.DeviceTransferStatus{LastSeen: device.LastSeen}
	if device.DeviceID == domain.WebManualDeviceID {
		status.IsProtected = true
		return status, nil
	}
	if device.LastSeen != nil {
		status.IsOnline = time.Since(*device.LastSeen) <= p.deviceOnlineWindow(ctx, device.TenantID)
	}
	session, err := p.presence.ActiveDeviceSession(adminCtx, device.TenantID, device.ID)
	if err != nil {
		return nil, fmt.Errorf("device transfer: find active session: %w", err)
	}
	if session != nil {
		status.ActiveSession = &organizationtenancy.DeviceTransferSession{
			ID: session.ID, StartedAt: session.StartedAt, ActivityName: session.ActivityName, RoomName: session.RoomName,
		}
	}
	status.CanTransfer = !status.IsOnline && status.ActiveSession == nil
	return status, nil
}

// deviceOnlineWindow resolves the school's online window and falls back to
// the default when the setting cannot be read.
func (p *Provisioning) deviceOnlineWindow(ctx context.Context, tenantID int64) time.Duration {
	minutes, err := p.settings.DeviceOnlineWindowMinutes(ctx, tenantID)
	if err != nil {
		p.logger.Warn("device transfer: resolve online window failed, using fallback",
			slog.Int64("tenant_id", tenantID),
			slog.Any("error", err),
		)
		return defaultDeviceOnlineWindow
	}
	if minutes <= 0 {
		return defaultDeviceOnlineWindow
	}
	return time.Duration(minutes) * time.Minute
}

func (p *Provisioning) archiveAndTransfer(ctx context.Context, source domain.Device, targetSchoolID int64) (domain.Device, error) {
	originalStatus := source.Status
	var transferredAPIKey *string
	if source.APIKey != nil {
		key := *source.APIKey
		transferredAPIKey = &key
	}
	now := time.Now()
	source.ArchivedAt = &now
	source.APIKey = nil
	source.Status = domain.DeviceStatusInactive
	source.RoomID = nil
	source.RegisteredByID = nil
	if err := p.devices.UpdateDevice(ctx, source); err != nil {
		return domain.Device{}, fmt.Errorf("TransferDevice: archive source: %w", err)
	}
	target, err := p.devices.CreateDevice(ctx, domain.NewDevice{
		TenantID: targetSchoolID, DeviceID: source.DeviceID, DeviceType: source.DeviceType,
		Name: source.Name, Status: originalStatus, APIKey: transferredAPIKey,
	})
	switch {
	case errors.Is(err, domain.ErrDeviceAPIKeyTaken):
		return domain.Device{}, conflict("api_key already in use")
	case errors.Is(err, domain.ErrDeviceIDTaken):
		return domain.Device{}, conflict("device_id already exists for target school")
	case err != nil:
		return domain.Device{}, fmt.Errorf("TransferDevice: create target: %w", err)
	}
	source.TransferredToDeviceID = &target.ID
	if err := p.devices.UpdateDevice(ctx, source); err != nil {
		return domain.Device{}, fmt.Errorf("TransferDevice: link source history: %w", err)
	}
	return target, nil
}

// findDevice reads and locks one live device.
func (p *Provisioning) findDevice(ctx context.Context, id int64) (domain.Device, error) {
	device, found, err := p.devices.FindDeviceForUpdate(ctx, id)
	if err != nil {
		return domain.Device{}, err
	}
	if !found {
		return domain.Device{}, &organizationtenancy.OperatorDeviceNotFoundError{DeviceID: id}
	}
	return device, nil
}

// findOperableSchool reads a school a device may belong to: it exists, is
// not deleted and is active.
func (p *Provisioning) findOperableSchool(ctx context.Context, schoolID int64) (organizationtenancy.School, error) {
	school, err := p.findLiveSchool(ctx, schoolID)
	if err != nil {
		return organizationtenancy.School{}, err
	}
	if !school.Active {
		return organizationtenancy.School{}, &organizationtenancy.SchoolInactiveError{SchoolID: schoolID}
	}
	return school, nil
}

func isSchoolStateError(err error) bool {
	var deleted *organizationtenancy.SchoolAlreadyDeletedError
	var inactive *organizationtenancy.SchoolInactiveError
	return errors.As(err, &deleted) || errors.As(err, &inactive)
}

func apiKeyMode(manual bool) string {
	if manual {
		return "manual"
	}
	return "auto"
}

func conflict(message string) error {
	return &organizationtenancy.ProvisioningConflictError{Err: errors.New(message)}
}
