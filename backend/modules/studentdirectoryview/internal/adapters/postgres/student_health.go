package studentdirectoryview

import (
	"errors"
	"fmt"
)

// HealthCipher seals and opens single database values bound to their
// context. The roots bind Security Runtime's field cipher under
// HEALTH_DATA_KEYS (#4722).
type HealthCipher interface {
	Seal(plaintext, context []byte) ([]byte, error)
	Open(sealed, context []byte) ([]byte, error)
}

// ErrStudentHealthUnreadable reports a sealed health field that does not
// open: a key missing from the ring or a changed value. It is a server
// error, never a reason to answer without the value.
var ErrStudentHealthUnreadable = errors.New("student health information does not open")

// StudentHealthContext binds the sealed health field of a child to its school
// and care row (#5472). Migration 1.15.730 seals with the same context.
func StudentHealthContext(tenantID, membershipID int64) []byte {
	return fmt.Appendf(nil, "users.student_care_profiles.health_data:%d:%d", tenantID, membershipID)
}

// SealStudentHealth seals the health field Care Plan writes into the care
// row of membershipID; nil stays NULL.
func (r Reader) SealStudentHealth(tenantID, membershipID int64, value *string) ([]byte, error) {
	return sealStudentHealth(r.health, tenantID, membershipID, value)
}

// OpenStudentHealth opens the health_data column a directory row carries;
// NULL opens to nil.
func (r Reader) OpenStudentHealth(tenantID, membershipID int64, sealed []byte) (*string, error) {
	return openStudentHealth(r.health, tenantID, membershipID, sealed)
}

func sealStudentHealth(health HealthCipher, tenantID, membershipID int64, value *string) ([]byte, error) {
	if value == nil {
		return nil, nil
	}
	if health == nil {
		return nil, errors.New("studentdirectoryview: health cipher is required to seal the health field")
	}
	if tenantID <= 0 || membershipID <= 0 {
		return nil, errors.New("studentdirectoryview: school and membership are required to seal the health field")
	}
	return health.Seal([]byte(*value), StudentHealthContext(tenantID, membershipID))
}

func openStudentHealth(health HealthCipher, tenantID, membershipID int64, sealed []byte) (*string, error) {
	if sealed == nil {
		return nil, nil
	}
	if health == nil {
		return nil, errors.New("studentdirectoryview: health cipher is required to open the health field")
	}
	plaintext, err := health.Open(sealed, StudentHealthContext(tenantID, membershipID))
	if err != nil {
		return nil, fmt.Errorf("%w: membership %d", ErrStudentHealthUnreadable, membershipID)
	}
	value := string(plaintext)
	return &value, nil
}
