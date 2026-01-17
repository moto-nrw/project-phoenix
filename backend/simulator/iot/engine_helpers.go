package iot

import "strings"

func (e *Engine) isDeviceAllowed(action ActionConfig, deviceID string) bool {
	if len(action.DeviceIDs) == 0 {
		return true
	}
	for _, allowed := range action.DeviceIDs {
		if allowed == deviceID {
			return true
		}
	}
	return false
}

func (e *Engine) deviceConfig(deviceID string) (DeviceConfig, bool) {
	cfg, ok := e.deviceConfigs[deviceID]
	return cfg, ok
}

func (e *Engine) randIntn(n int) int {
	e.randMu.Lock()
	defer e.randMu.Unlock()
	return e.rand.Intn(n)
}

func ptrInt64(v int64) *int64 {
	vv := v
	return &vv
}

func isVisitMissingError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "visit not found") ||
		strings.Contains(msg, "no active visit") ||
		strings.Contains(msg, "room_id is required for check-in")
}
