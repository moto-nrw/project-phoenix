package repositoryadapter

import (
	"fmt"
	"time"

	iotModels "github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
)

// toFilter translates the retired filter map into the owner's typed filter.
// An unknown key is an error rather than a dynamically built predicate: the
// device table is never queried through a caller-supplied column name.
func toFilter(filters map[string]any) (devicefleet.DeviceFilter, error) {
	filter := devicefleet.DeviceFilter{}
	for field, value := range filters {
		if value == nil {
			continue
		}
		if err := applyFilterField(&filter, field, value); err != nil {
			return devicefleet.DeviceFilter{}, err
		}
	}
	return filter, nil
}

func applyFilterField(filter *devicefleet.DeviceFilter, field string, value any) error {
	switch field {
	case "device_id_like":
		return assignString(&filter.DeviceIDContains, field, value)
	case "name_like":
		return assignString(&filter.NameContains, field, value)
	case "status":
		return assignStatus(filter, value)
	case "device_type":
		return assignString(&filter.DeviceType, field, value)
	case "exclude_device_type":
		return assignString(&filter.ExcludeDeviceType, field, value)
	case "exclude_device_id":
		return assignString(&filter.ExcludeDeviceID, field, value)
	case "seen_after":
		return assignTime(&filter.SeenAfter, field, value)
	case "seen_before":
		return assignTime(&filter.SeenBefore, field, value)
	case "room_id":
		return assignInt64(&filter.RoomID, field, value)
	case "registered_by_id":
		return assignInt64(&filter.RegisteredByID, field, value)
	case "has_name":
		flag, ok := value.(bool)
		if !ok {
			return unsupportedFilter(field, value)
		}
		filter.HasName = &flag
		return nil
	default:
		return fmt.Errorf("devicefleet repository adapter: unsupported device filter %q", field)
	}
}

func assignStatus(filter *devicefleet.DeviceFilter, value any) error {
	switch typed := value.(type) {
	case devicefleet.DeviceStatus:
		filter.Status = &typed
		return nil
	case iotModels.DeviceStatus:
		status := devicefleet.DeviceStatus(typed)
		filter.Status = &status
		return nil
	case string:
		status := devicefleet.DeviceStatus(typed)
		filter.Status = &status
		return nil
	default:
		return unsupportedFilter("status", value)
	}
}

func assignString(target **string, field string, value any) error {
	switch typed := value.(type) {
	case string:
		*target = &typed
	case devicefleet.DeviceStatus:
		text := string(typed)
		*target = &text
	case iotModels.DeviceStatus:
		text := string(typed)
		*target = &text
	default:
		return unsupportedFilter(field, value)
	}
	return nil
}

func assignTime(target **time.Time, field string, value any) error {
	moment, ok := value.(time.Time)
	if !ok {
		return unsupportedFilter(field, value)
	}
	*target = &moment
	return nil
}

func assignInt64(target **int64, field string, value any) error {
	switch typed := value.(type) {
	case int64:
		*target = &typed
	case int:
		converted := int64(typed)
		*target = &converted
	case int32:
		converted := int64(typed)
		*target = &converted
	default:
		return unsupportedFilter(field, value)
	}
	return nil
}

func unsupportedFilter(field string, value any) error {
	return fmt.Errorf("devicefleet repository adapter: device filter %q does not accept %T", field, value)
}
