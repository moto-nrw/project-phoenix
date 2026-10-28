package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

const (
	webManualDeviceID   = "WEB-MANUAL-001"
	webManualDeviceName = "Web-Portal (Manuell)"
)

type configureProfileStep struct {
	definition demoProfileDefinition
}

type operatorSeedDevice struct {
	ID         int64   `json:"id"`
	DeviceID   string  `json:"device_id"`
	DeviceType string  `json:"device_type"`
	Name       *string `json:"name"`
	Status     string  `json:"status"`
}

func (configureProfileStep) Name() string { return "Configuring demo school profile" }

func (s configureProfileStep) Run(_ context.Context, rt *Runtime) error {
	if rt.Bootstrap == nil {
		return fmt.Errorf("bootstrap state not available")
	}
	keys := sortedProfileSettingKeys(s.definition.Settings)
	for _, key := range keys {
		setting := s.definition.Settings[key]
		path := "/api/settings/values/" + key
		auth := rt.TenantAuth
		switch setting.ManagedBy {
		case SettingManagedByOperator:
			path = fmt.Sprintf("/operator/schools/%d/settings/values/%s", rt.Bootstrap.SchoolID, key)
			auth = rt.OperatorAuth
		case SettingManagedByTenant:
		default:
			return fmt.Errorf("setting %s has unknown manager %q", key, setting.ManagedBy)
		}
		if _, err := rt.Client.PutWithAuth(auth, path, map[string]any{"value": setting.Value}); err != nil {
			return fmt.Errorf("set profile setting %s: %w", key, err)
		}
	}
	fmt.Printf("Configured profile %s with %d explicit settings\n", s.definition.Key, len(keys))
	return nil
}

// configureDevicePINStep makes the seed's staff PIN the school's device PIN.
// Device auth resolves security.ogs_device_pin, whose registry default is
// 1234. OGS_DEVICE_PIN is only a demo/seeder input, never a runtime fallback. Without
// this write every device call of a school seeded with another PIN fails with
// 401. The setting is admin-only, so the school admin writes it. Its
// dependency on module.nfc_anwesenheit only hides it in the settings screen;
// the write also succeeds while NFC is off, as on the public demo.
type configureDevicePINStep struct{}

func (configureDevicePINStep) Name() string { return "Configuring device PIN" }

func (configureDevicePINStep) Run(_ context.Context, rt *Runtime) error {
	path := "/api/settings/values/" + profileSettingDevicePIN
	if _, err := rt.Client.PutWithAuth(rt.TenantAuth, path, map[string]any{"value": rt.StaffPIN}); err != nil {
		return fmt.Errorf("set device PIN: %w", err)
	}
	return nil
}

type verifyProfileStep struct {
	definition demoProfileDefinition
}

func (verifyProfileStep) Name() string { return "Verifying demo school profile" }

func (s verifyProfileStep) Run(_ context.Context, rt *Runtime) error {
	if rt.Bootstrap == nil {
		return fmt.Errorf("bootstrap state not available")
	}
	if err := verifyProfileSettings(rt, s.definition); err != nil {
		return err
	}
	virtualDevice, err := verifyProfileDevices(rt, s.definition.Expected.PhysicalDevices)
	if err != nil {
		return err
	}
	rt.Values["profile.virtual_device"] = virtualDevice
	if err := verifyProfileStudents(rt, s.definition.Expected.Students); err != nil {
		return err
	}
	if err := verifyProfilePlanning(rt, s.definition.Expected.ScheduledActivities); err != nil {
		return err
	}
	fmt.Printf("Verified API contract for profile %s\n", s.definition.Key)
	return nil
}

func verifyProfileSettings(rt *Runtime, definition demoProfileDefinition) error {
	tenantRaw, err := rt.Client.GetWithAuth(rt.TenantAuth, "/api/settings/schema")
	if err != nil {
		return fmt.Errorf("read tenant settings: %w", err)
	}
	operatorPath := fmt.Sprintf("/operator/schools/%d/settings/schema", rt.Bootstrap.SchoolID)
	operatorRaw, err := rt.Client.GetWithAuth(rt.OperatorAuth, operatorPath)
	if err != nil {
		return fmt.Errorf("read operator settings: %w", err)
	}
	tenantValues, err := decodeSettingsValues(tenantRaw)
	if err != nil {
		return fmt.Errorf("decode tenant settings: %w", err)
	}
	operatorValues, err := decodeSettingsValues(operatorRaw)
	if err != nil {
		return fmt.Errorf("decode operator settings: %w", err)
	}
	for _, key := range sortedProfileSettingKeys(definition.Settings) {
		expected := definition.Settings[key]
		values := tenantValues
		if expected.ManagedBy == SettingManagedByOperator {
			values = operatorValues
		}
		actual, ok := values[key]
		if !ok {
			return fmt.Errorf("setting %s missing from %s read API", key, expected.ManagedBy)
		}
		if !bytes.Equal(actual, expected.Value) {
			return fmt.Errorf("setting %s: expected %s, got %s", key, expected.Value, actual)
		}
	}
	return nil
}

func decodeSettingsValues(raw []byte) (map[string]json.RawMessage, error) {
	var envelope struct {
		Data struct {
			Tabs []struct {
				Categories []struct {
					Items []struct {
						Key   string          `json:"key"`
						Value json.RawMessage `json:"value"`
					} `json:"items"`
				} `json:"categories"`
			} `json:"tabs"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, err
	}
	values := make(map[string]json.RawMessage)
	for _, tab := range envelope.Data.Tabs {
		for _, category := range tab.Categories {
			for _, item := range category.Items {
				values[item.Key] = item.Value
			}
		}
	}
	return values, nil
}

func verifyProfileDevices(rt *Runtime, minimumPhysical int) (SeedDevice, error) {
	physical, err := listSeedDevices(rt, "terminal")
	if err != nil {
		return SeedDevice{}, fmt.Errorf("read physical devices: %w", err)
	}
	if len(physical) < minimumPhysical {
		return SeedDevice{}, fmt.Errorf("physical devices: expected at least %d, got %d", minimumPhysical, len(physical))
	}
	virtual, err := listSeedDevices(rt, "virtual")
	if err != nil {
		return SeedDevice{}, fmt.Errorf("read virtual devices: %w", err)
	}
	for _, device := range virtual {
		if device.DeviceID == webManualDeviceID && device.DeviceType == "virtual" {
			device.Protected = true
			return device, nil
		}
	}
	return SeedDevice{}, fmt.Errorf("protected virtual web device %s missing", webManualDeviceID)
}

func verifyDeviceAdministrationLocked(rt *Runtime) (SeedDevice, error) {
	if rt.Bootstrap == nil || rt.Bootstrap.SchoolID <= 0 {
		return SeedDevice{}, fmt.Errorf("device administration: bootstrap school is unavailable")
	}
	// Schools without NFC attendance cannot administer devices. Verify that
	// tenant-facing lock first, then inspect the provisioned web device through
	// the operator API, which is not part of the locked tenant surface.
	if err := verifyDeviceAdministrationRefusal(rt); err != nil {
		return SeedDevice{}, err
	}
	devices, err := listOperatorSeedDevices(rt, rt.Bootstrap.SchoolID)
	if err != nil {
		return SeedDevice{}, err
	}
	webDevice, err := findWebManualSeedDevice(devices)
	if err != nil {
		return SeedDevice{}, err
	}
	return verifyWebManualDeviceProtection(rt, webDevice)
}

func verifyDeviceAdministrationRefusal(rt *Runtime) error {
	_, err := listSeedDevices(rt, "terminal")
	var apiErr *APIError
	if err == nil {
		return fmt.Errorf("device administration: expected 403 module.not_booked, got success")
	}
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 403 || apiErr.Code != "module.not_booked" {
		return fmt.Errorf("device administration: expected 403 module.not_booked, got %w", err)
	}
	return nil
}

func listOperatorSeedDevices(rt *Runtime, schoolID int64) ([]operatorSeedDevice, error) {
	raw, err := rt.Client.GetWithAuth(rt.OperatorAuth, fmt.Sprintf("/operator/schools/%d/devices", schoolID))
	if err != nil {
		return nil, fmt.Errorf("list operator devices: %w", err)
	}
	var devices struct {
		Data []operatorSeedDevice `json:"data"`
	}
	if err := json.Unmarshal(raw, &devices); err != nil {
		return nil, fmt.Errorf("decode operator devices: %w", err)
	}
	return devices.Data, nil
}

func findWebManualSeedDevice(devices []operatorSeedDevice) (SeedDevice, error) {
	var webDevice *SeedDevice
	for _, device := range devices {
		if device.DeviceType == "terminal" {
			return SeedDevice{}, fmt.Errorf("device administration: unexpected physical terminal %s", device.DeviceID)
		}
		if device.DeviceID != webManualDeviceID {
			continue
		}
		if webDevice != nil {
			return SeedDevice{}, fmt.Errorf("device administration: multiple virtual web devices %s", webManualDeviceID)
		}
		if device.ID <= 0 || device.DeviceType != "virtual" || device.Name == nil || *device.Name != webManualDeviceName || device.Status != "active" {
			return SeedDevice{}, fmt.Errorf("device administration: invalid virtual web device %s", webManualDeviceID)
		}
		webDevice = &SeedDevice{ID: device.ID, DeviceID: device.DeviceID, DeviceType: device.DeviceType, Name: *device.Name}
	}
	if webDevice == nil {
		return SeedDevice{}, fmt.Errorf("device administration: virtual web device %s missing", webManualDeviceID)
	}
	return *webDevice, nil
}

func verifyWebManualDeviceProtection(rt *Runtime, webDevice SeedDevice) (SeedDevice, error) {
	raw, err := rt.Client.GetWithAuth(rt.OperatorAuth, fmt.Sprintf("/operator/devices/%d/transfer-status", webDevice.ID))
	if err != nil {
		return SeedDevice{}, fmt.Errorf("read virtual web device protection: %w", err)
	}
	var transferStatus struct {
		Data struct {
			IsProtected bool `json:"is_protected"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &transferStatus); err != nil {
		return SeedDevice{}, fmt.Errorf("decode virtual web device protection: %w", err)
	}
	if !transferStatus.Data.IsProtected {
		return SeedDevice{}, fmt.Errorf("virtual web device %s is not protected", webManualDeviceID)
	}
	webDevice.Protected = true
	return webDevice, nil
}

func listSeedDevices(rt *Runtime, deviceType string) ([]SeedDevice, error) {
	raw, err := rt.Client.Get("/api/iot/?device_type=" + deviceType)
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Data []SeedDevice `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, err
	}
	return envelope.Data, nil
}

func verifyProfileStudents(rt *Runtime, minimum int) error {
	raw, err := rt.Client.Get("/api/students?page=1&page_size=1")
	if err != nil {
		return fmt.Errorf("read students: %w", err)
	}
	var envelope struct {
		Pagination struct {
			TotalRecords int `json:"total_records"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("decode students: %w", err)
	}
	if envelope.Pagination.TotalRecords < minimum {
		return fmt.Errorf("students: expected at least %d, got %d", minimum, envelope.Pagination.TotalRecords)
	}
	return nil
}

func verifyProfilePlanning(rt *Runtime, expected SeedScheduledActivityState) error {
	today := todaySeedDate()
	raw, err := rt.Client.Get(fmt.Sprintf("/api/timetable/instances?from=%s&to=%s", today.String(), today.AddDays(6).String()))
	if err != nil {
		return fmt.Errorf("read planned activities: %w", err)
	}
	var envelope struct {
		Data struct {
			Instances []struct {
				Status     string  `json:"status"`
				RoomID     int64   `json:"room_id"`
				StudentIDs []int64 `json:"student_ids"`
				Staff      []struct {
					StaffID int64 `json:"staff_id"`
				} `json:"staff"`
			} `json:"instances"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("decode planned activities: %w", err)
	}
	matching := 0
	for _, instance := range envelope.Data.Instances {
		if matchesScheduledActivity(instance.Status, instance.RoomID, instance.StudentIDs, len(instance.Staff), expected) {
			matching++
		}
	}
	if matching < expected.Minimum {
		return fmt.Errorf("scheduled activities: expected at least %d with rooms, students, and staff; got %d", expected.Minimum, matching)
	}
	return nil
}

func matchesScheduledActivity(status string, roomID int64, studentIDs []int64, staffCount int, expected SeedScheduledActivityState) bool {
	return status == "planned" &&
		(!expected.Rooms || roomID != 0) &&
		(!expected.Students || len(studentIDs) > 0) &&
		(!expected.Staff || staffCount > 0)
}

func sortedProfileSettingKeys(settings map[string]SeedSetting) []string {
	keys := make([]string, 0, len(settings))
	for key := range settings {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	// A rerun may start from any valid scope set. Expanding visibility first
	// and restricting it last keeps every intermediate set valid; the sorted
	// order already puts the overview scope after every action scope.
	if setting, ok := settings[profileSettingOverviewScope]; !ok || !bytes.Equal(setting.Value, []byte(`"all_staff"`)) {
		return keys
	}
	for index, key := range keys {
		if key == profileSettingOverviewScope {
			copy(keys[1:index+1], keys[:index])
			keys[0] = key
			break
		}
	}
	return keys
}
