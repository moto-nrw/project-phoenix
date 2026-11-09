package application

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// boundCipher opens a value only with the context it was sealed with, like
// the field cipher's associated data.
type boundCipher struct{}

func (boundCipher) Seal(plaintext, context []byte) ([]byte, error) {
	return append(append(append([]byte{}, context...), '|'), plaintext...), nil
}

func (boundCipher) Open(sealed, context []byte) ([]byte, error) {
	plain, ok := bytes.CutPrefix(sealed, append(append([]byte{}, context...), '|'))
	if !ok {
		return nil, errors.New("wrong context")
	}
	return plain, nil
}

func exportRow(values map[string]string) map[string]json.RawMessage {
	row := make(map[string]json.RawMessage, len(values))
	for k, v := range values {
		row[k] = json.RawMessage(v)
	}
	return row
}

// The return export opens the holiday health information only with the
// Träger or school and the row it was sealed to (#3838, #6328).
func TestHolidayHealthExportFieldsOpenOnlyTheirOwnRow(t *testing.T) {
	t.Parallel()
	cipher := boundCipher{}
	record := `{"allergies":"Erdnüsse"}`
	family, _ := cipher.Seal([]byte(record), enrollment.HolidayFamilyHealthContext(7, 21))
	child, _ := cipher.Seal([]byte(record), enrollment.HolidayChildHealthContext(7, 21, 2))
	guest, _ := cipher.Seal([]byte(record), enrollment.HolidayGuestHealthContext(3, 31))
	fields := NewHolidayHealthExportFields(cipher)

	for table, open := range map[string]struct {
		row    map[string]json.RawMessage
		sealed []byte
	}{
		"enrollment.holiday_registrations":         {exportRow(map[string]string{"organization_id": "7", "id": "21"}), family},
		"enrollment.holiday_registration_children": {exportRow(map[string]string{"organization_id": "7", "holiday_registration_id": "21", "position": "2"}), child},
		"enrollment.holiday_guests":                {exportRow(map[string]string{"tenant_id": "3", "holiday_registration_child_id": "31"}), guest},
	} {
		value, err := fields.OpenExportField(table, "health_data", open.row, open.sealed)
		require.NoError(t, err, table)
		require.JSONEq(t, record, value, table)
	}

	for name, call := range map[string]func() (string, error){
		"other Träger": func() (string, error) {
			return fields.OpenExportField("enrollment.holiday_registrations", "health_data", exportRow(map[string]string{"organization_id": "8", "id": "21"}), family)
		},
		"other position": func() (string, error) {
			return fields.OpenExportField("enrollment.holiday_registration_children", "health_data", exportRow(map[string]string{"organization_id": "7", "holiday_registration_id": "21", "position": "1"}), child)
		},
		"other school": func() (string, error) {
			return fields.OpenExportField("enrollment.holiday_guests", "health_data", exportRow(map[string]string{"tenant_id": "4", "holiday_registration_child_id": "31"}), guest)
		},
		"child value as family value": func() (string, error) {
			return fields.OpenExportField("enrollment.holiday_registrations", "health_data", exportRow(map[string]string{"organization_id": "7", "id": "21"}), child)
		},
		"row without registration": func() (string, error) {
			return fields.OpenExportField("enrollment.holiday_registration_children", "health_data", exportRow(map[string]string{"organization_id": "7", "position": "2"}), child)
		},
		"foreign table": func() (string, error) {
			return fields.OpenExportField("users.student_care_profiles", "health_data", exportRow(map[string]string{"organization_id": "7", "id": "21"}), family)
		},
		"unknown column": func() (string, error) {
			return fields.OpenExportField("enrollment.holiday_registrations", "custom_data", exportRow(map[string]string{"organization_id": "7", "id": "21"}), family)
		},
		"without key ring": func() (string, error) {
			return NewHolidayHealthExportFields(nil).OpenExportField("enrollment.holiday_registrations", "health_data", exportRow(map[string]string{"organization_id": "7", "id": "21"}), family)
		},
	} {
		value, err := call()
		require.Error(t, err, name)
		require.Empty(t, value, name)
		require.NotContains(t, err.Error(), "Erdnüsse", name)
	}
}
