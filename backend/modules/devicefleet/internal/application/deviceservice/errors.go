package iot

import "github.com/moto-nrw/project-phoenix/modules/devicefleet"

var (
	ErrDeviceNotFound    = devicefleet.ErrAdministrationDeviceNotFound
	ErrInvalidDeviceData = devicefleet.ErrAdministrationInvalidDeviceData
	ErrDuplicateDeviceID = devicefleet.ErrAdministrationDuplicateDeviceID
	ErrInvalidStatus     = devicefleet.ErrAdministrationInvalidStatus
	ErrDeviceOffline     = devicefleet.ErrAdministrationDeviceOffline
	ErrNetworkScanFailed = devicefleet.ErrAdministrationNetworkScanFailed
	ErrDatabaseOperation = devicefleet.ErrAdministrationDatabaseOperation
	ErrDeviceProtected   = devicefleet.ErrAdministrationDeviceProtected
)

type IoTError = devicefleet.AdministrationError

type DeviceNotFoundError = devicefleet.AdministrationDeviceNotFoundError

type DuplicateDeviceIDError = devicefleet.AdministrationDuplicateDeviceIDError
