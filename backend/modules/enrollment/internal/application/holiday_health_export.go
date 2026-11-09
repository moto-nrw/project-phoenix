package application

import (
	"encoding/json"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// HolidayHealthExportFields opens the sealed health information of holiday
// registrations, their children and holiday guests for the return export
// (#3838, #6328). It binds each value to the Träger or school and the row of
// its exported row, as sealing did; the plaintext is the JSON the value
// holds. A value copied into another row does not open.
type HolidayHealthExportFields struct {
	cipher enrollment.HolidayHealthCipher
}

func NewHolidayHealthExportFields(cipher enrollment.HolidayHealthCipher) *HolidayHealthExportFields {
	return &HolidayHealthExportFields{cipher: cipher}
}

var errHolidayHealthExportField = errors.New("enrollment: not a sealed holiday health export field")

// OpenExportField answers the opened value; the error never names it.
func (f *HolidayHealthExportFields) OpenExportField(table, column string, row map[string]json.RawMessage, sealed []byte) (string, error) {
	if column != "health_data" || f.cipher == nil {
		return "", errHolidayHealthExportField
	}
	context, ok := holidayHealthExportContext(table, row)
	if !ok {
		return "", errHolidayHealthExportField
	}
	plain, err := f.cipher.Open(sealed, context)
	if err != nil {
		return "", enrollment.ErrHolidayHealthUnreadable
	}
	return string(plain), nil
}

func holidayHealthExportContext(table string, row map[string]json.RawMessage) ([]byte, bool) {
	ids := func(columns ...string) ([]int64, bool) {
		values := make([]int64, len(columns))
		for i, column := range columns {
			if json.Unmarshal(row[column], &values[i]) != nil || values[i] <= 0 {
				return nil, false
			}
		}
		return values, true
	}
	switch table {
	case "enrollment.holiday_registrations":
		if v, ok := ids("organization_id", "id"); ok {
			return enrollment.HolidayFamilyHealthContext(v[0], v[1]), true
		}
	case "enrollment.holiday_registration_children":
		if v, ok := ids("organization_id", "holiday_registration_id", "position"); ok {
			return enrollment.HolidayChildHealthContext(v[0], v[1], int(v[2])), true
		}
	case "enrollment.holiday_guests":
		if v, ok := ids("tenant_id", "holiday_registration_child_id"); ok {
			return enrollment.HolidayGuestHealthContext(v[0], v[1]), true
		}
	}
	return nil, false
}
