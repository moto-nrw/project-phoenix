package device

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/core/port"
)

// IsIoTDeviceRequest checks if the request context is marked as IoT device.
func IsIoTDeviceRequest(ctx context.Context) bool {
	return port.IsIoTDeviceRequest(ctx)
}
