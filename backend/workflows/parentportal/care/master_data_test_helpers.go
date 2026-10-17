//go:build testhooks

package care

import "encoding/json"

func TrackBFieldStateFixture(target, field string, person *MasterDataPerson, student *MasterDataStudent, value json.RawMessage, policy DeparturePolicy) (json.RawMessage, json.RawMessage, bool, error) {
	return trackBFieldState(target, field, person, student, value, policy)
}
