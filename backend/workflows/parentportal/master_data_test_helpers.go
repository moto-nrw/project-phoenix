//go:build testhooks

package parentportal

import (
	"encoding/json"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/care"
)

type MasterDataPerson care.MasterDataPerson
type MasterDataStudent care.MasterDataStudent

func TrackBFieldStateFixture(target, field string, person *MasterDataPerson, student *MasterDataStudent, value json.RawMessage, policy DeparturePolicy) (json.RawMessage, json.RawMessage, bool, error) {
	return care.TrackBFieldStateFixture(target, field, (*care.MasterDataPerson)(person), (*care.MasterDataStudent)(student), value, policy)
}
