package organizationtenancy

import "fmt"

// The provisioning errors carry the identifier the operator acted on. The
// HTTP layer maps them by type.

// InvalidProvisioningDataError rejects operator input that cannot be applied.
type InvalidProvisioningDataError struct{ Err error }

func (e *InvalidProvisioningDataError) Error() string { return fmt.Sprintf("invalid data: %v", e.Err) }
func (e *InvalidProvisioningDataError) Unwrap() error { return e.Err }

// ProvisioningConflictError rejects a write that collides with existing data.
type ProvisioningConflictError struct{ Err error }

func (e *ProvisioningConflictError) Error() string { return fmt.Sprintf("conflict: %v", e.Err) }
func (e *ProvisioningConflictError) Unwrap() error { return e.Err }

// OrganizationNotFoundError reports an organisation that does not exist.
type OrganizationNotFoundError struct{ OrganizationID int64 }

func (e *OrganizationNotFoundError) Error() string {
	return fmt.Sprintf("organization with ID %d not found", e.OrganizationID)
}

// OrganizationAlreadyDeletedError rejects soft-deleting a deleted organisation.
type OrganizationAlreadyDeletedError struct{ OrganizationID int64 }

func (e *OrganizationAlreadyDeletedError) Error() string {
	return fmt.Sprintf("organization with ID %d is already soft-deleted", e.OrganizationID)
}

// OrganizationNotDeletedError rejects restoring an organisation that is not deleted.
type OrganizationNotDeletedError struct{ OrganizationID int64 }

func (e *OrganizationNotDeletedError) Error() string {
	return fmt.Sprintf("organization with ID %d is not soft-deleted", e.OrganizationID)
}

// OrganizationDeletedError rejects attaching a school to a deleted organisation.
type OrganizationDeletedError struct{ OrganizationID int64 }

func (e *OrganizationDeletedError) Error() string {
	return fmt.Sprintf("organization with ID %d is soft-deleted and cannot host schools", e.OrganizationID)
}

// SchoolNotFoundError reports a school that does not exist.
type SchoolNotFoundError struct{ SchoolID int64 }

func (e *SchoolNotFoundError) Error() string {
	return fmt.Sprintf("school with ID %d not found", e.SchoolID)
}

// SchoolInactiveError rejects an operation on an inactive school.
type SchoolInactiveError struct{ SchoolID int64 }

func (e *SchoolInactiveError) Error() string {
	return fmt.Sprintf("school with ID %d is inactive", e.SchoolID)
}

// SchoolAlreadyDeletedError rejects an operation on a soft-deleted school.
type SchoolAlreadyDeletedError struct{ SchoolID int64 }

func (e *SchoolAlreadyDeletedError) Error() string {
	return fmt.Sprintf("school with ID %d is already soft-deleted", e.SchoolID)
}

// SchoolNotDeletedError rejects restoring a school that is not deleted.
type SchoolNotDeletedError struct{ SchoolID int64 }

func (e *SchoolNotDeletedError) Error() string {
	return fmt.Sprintf("school with ID %d is not soft-deleted", e.SchoolID)
}

// OperatorDeviceNotFoundError reports a device row that does not exist. The
// ID is the database row, not the human-readable device identifier.
type OperatorDeviceNotFoundError struct{ DeviceID int64 }

func (e *OperatorDeviceNotFoundError) Error() string {
	return fmt.Sprintf("device with ID %d not found", e.DeviceID)
}

// DeviceInUseError rejects deleting a device that attendance or session
// records still reference.
type DeviceInUseError struct{ DeviceID int64 }

func (e *DeviceInUseError) Error() string {
	return fmt.Sprintf("device with ID %d is still in use and cannot be deleted", e.DeviceID)
}

// DeviceProtectedError rejects deleting a system-managed device.
type DeviceProtectedError struct {
	DeviceID int64
	Reason   string
}

func (e *DeviceProtectedError) Error() string {
	return fmt.Sprintf("device with ID %d is protected: %s", e.DeviceID, e.Reason)
}

// DeviceTransferProtectedError rejects moving a system-managed device.
type DeviceTransferProtectedError struct {
	DeviceID int64
	Reason   string
}

func (e *DeviceTransferProtectedError) Error() string {
	return fmt.Sprintf("device with ID %d cannot be transferred: %s", e.DeviceID, e.Reason)
}

// The live device states that block a transfer.
const (
	DeviceTransferBlockedOnline        = "device_online"
	DeviceTransferBlockedActiveSession = "active_session"
)

// DeviceTransferBlockedError reports live device state that must end first.
type DeviceTransferBlockedError struct {
	DeviceID int64
	Reason   string
}

func (e *DeviceTransferBlockedError) Error() string {
	return fmt.Sprintf("device with ID %d cannot be transferred: %s", e.DeviceID, e.Reason)
}

// DeviceTransferOrganizationMismatchError rejects moving a device across
// organisations.
type DeviceTransferOrganizationMismatchError struct {
	SourceSchoolID int64
	TargetSchoolID int64
}

func (e *DeviceTransferOrganizationMismatchError) Error() string {
	return fmt.Sprintf("schools %d and %d belong to different organizations", e.SourceSchoolID, e.TargetSchoolID)
}

// DeviceTransferSameSchoolError reports a transfer to the device's own school.
type DeviceTransferSameSchoolError struct{ SchoolID int64 }

func (e *DeviceTransferSameSchoolError) Error() string {
	return fmt.Sprintf("device already belongs to school %d", e.SchoolID)
}

// PersonNotFoundError reports a person that does not exist or is deleted.
type PersonNotFoundError struct{ PersonID int64 }

func (e *PersonNotFoundError) Error() string {
	return fmt.Sprintf("person with ID %d not found", e.PersonID)
}

// PersonHasActiveSupervisionsError rejects deleting a person whose staff
// member still supervises an active group.
type PersonHasActiveSupervisionsError struct {
	PersonID int64
	Count    int
}

func (e *PersonHasActiveSupervisionsError) Error() string {
	return fmt.Sprintf("person with ID %d has %d active supervision(s) and cannot be deleted", e.PersonID, e.Count)
}
