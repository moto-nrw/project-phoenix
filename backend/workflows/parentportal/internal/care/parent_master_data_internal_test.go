package care

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMasterDataSmallHelpers(t *testing.T) {
	t.Parallel()

	type phoneChoice struct {
		ID          int64
		PhoneNumber string
		IsPrimary   bool
	}
	primaryPhone := func(phone *phoneChoice) bool { return phone.IsPrimary }
	primary := &phoneChoice{PhoneNumber: "2", IsPrimary: true}
	primary.ID = 2
	secondary := &phoneChoice{PhoneNumber: "1"}
	secondary.ID = 1
	assert.Equal(t, primary, pickPrimaryPhone([]*phoneChoice{secondary, primary}, primaryPhone))
	assert.Equal(t, secondary, pickPrimaryPhone([]*phoneChoice{secondary}, primaryPhone))
	assert.Nil(t, pickPrimaryPhone(nil, primaryPhone))

	got, err := decodeStringValue(json.RawMessage(`"  abc  "`))
	require.NoError(t, err)
	assert.Equal(t, "abc", got)
	_, err = decodeStringValue(nil)
	assert.ErrorIs(t, err, ErrMasterDataInvalidValue)
	_, err = decodeStringValue(json.RawMessage(`123`))
	assert.ErrorIs(t, err, ErrMasterDataInvalidValue)

	assert.Nil(t, strPtrOrNil("  "))
	assert.Equal(t, "abc", *strPtrOrNil("abc"))
	assert.JSONEq(t, `null`, string(jsonStringPtr(nil)))

	email := "parent@example.test"
	street := "Musterweg 1"
	profile := GuardianContactRecord{
		Email:                  &email,
		AddressStreet:          &street,
		PreferredContactMethod: "email",
		LanguagePreference:     "de",
	}
	svc := &Service{}
	assert.JSONEq(t, `"parent@example.test"`, string(svc.guardianProfileFieldJSON(profile, "email")))
	assert.JSONEq(t, `"Musterweg 1"`, string(svc.guardianProfileFieldJSON(profile, "address_street")))
	assert.JSONEq(t, `"email"`, string(svc.guardianProfileFieldJSON(profile, "preferred_contact_method")))
	assert.JSONEq(t, `"de"`, string(svc.guardianProfileFieldJSON(profile, "language_preference")))
	assert.JSONEq(t, `null`, string(svc.guardianProfileFieldJSON(profile, "unknown")))
}
