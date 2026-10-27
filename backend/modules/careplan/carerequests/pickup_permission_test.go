package carerequests

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeWeekdays(t *testing.T) {
	t.Parallel()

	days, err := NormalizeWeekdays([]int{4, 2, 2})
	require.NoError(t, err)
	assert.Equal(t, []int{2, 4}, days)

	all, err := NormalizeWeekdays([]int{1, 2, 3, 4, 5})
	require.NoError(t, err)
	assert.Nil(t, all, "all five weekdays mean every day")

	none, err := NormalizeWeekdays(nil)
	require.NoError(t, err)
	assert.Nil(t, none)

	for _, invalid := range [][]int{{0}, {6}, {-1, 2}} {
		_, err := NormalizeWeekdays(invalid)
		assert.ErrorIs(t, err, ErrInvalidPayload, "%v", invalid)
	}
}

func TestPickupPermissionPayloadRoundTrip(t *testing.T) {
	t.Parallel()

	input := PickupPermissionCreateInput{
		StudentID: 1, GuardianAccountID: 2, GuardianProfileID: 3, RelationshipID: 4,
		PersonName: " Erika Muster ", Weekdays: []int{3, 1}, PreviousLabel: "Nicht abholberechtigt",
		PreviousPermission: PickupPermissionState{CanPickup: true, Weekdays: []int{4, 2, 2}},
		Signature:          SignedStatement{SignatureID: 9, Statement: "Ich erlaube …"},
	}
	raw, err := PickupPermissionPayload(input)
	require.NoError(t, err)

	permission, err := ParsePickupPermission(raw)
	require.NoError(t, err)
	assert.Equal(t, input.GuardianProfileID, permission.GuardianProfileID)
	assert.Equal(t, input.RelationshipID, permission.RelationshipID)
	assert.Equal(t, "Erika Muster", permission.PersonName)
	assert.Equal(t, []int{1, 3}, permission.Weekdays)
	require.NotNil(t, permission.PreviousPermission)
	assert.True(t, permission.PreviousPermission.CanPickup)
	assert.Equal(t, []int{2, 4}, permission.PreviousPermission.Weekdays)
	require.NotNil(t, permission.Signature)
	assert.Equal(t, input.Signature.SignatureID, permission.Signature.SignatureID)

	entries := RequestedSummary(raw)
	require.Len(t, entries, 1)
	assert.Equal(t, "Abholberechtigung · Erika Muster", entries[0].Label)
	assert.Equal(t, "Nur Mo, Mi", entries[0].New)
	assert.Equal(t, KindPickupPerson, entries[0].CareKind)
}

func TestPickupPermissionPayloadRequiresASignature(t *testing.T) {
	t.Parallel()

	_, err := PickupPermissionPayload(PickupPermissionCreateInput{
		StudentID: 1, GuardianAccountID: 2, GuardianProfileID: 3, RelationshipID: 4, PersonName: "Erika",
	})
	assert.ErrorIs(t, err, ErrInvalidPayload)
}

func TestWeekdayListLabel(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "An allen Tagen", WeekdayListLabel(nil))
	assert.Equal(t, "Nur Di, Fr", WeekdayListLabel([]int{2, 5}))
}

func TestPickupTermsReadTheOneDayCollector(t *testing.T) {
	t.Parallel()

	const collectorID int64 = 7
	raw := json.RawMessage(`{"date":"2026-11-10","reason":"Oma","pickup_person":{"guardian_profile_id":7,"name":"Erika Muster"}}`)
	change, ok := PickupTerms(raw)
	require.True(t, ok, "a collector alone is a valid one-day request")
	require.NotNil(t, change.PickupPerson)
	assert.Equal(t, collectorID, change.PickupPerson.GuardianProfileID)

	terms := StoredPickupTerms(raw)
	require.NotNil(t, terms)
	assert.Equal(t, "Erika Muster", terms.PickupPersonName)

	entries := RequestedSummary(raw)
	require.Len(t, entries, 1)
	assert.Equal(t, "10.11.2026 · Abholperson", entries[0].Label)
	assert.Equal(t, "Erika Muster", entries[0].New)

	_, ok = PickupTerms(json.RawMessage(`{"date":"2026-11-10","pickup_person":{"guardian_profile_id":0,"name":""}}`))
	assert.False(t, ok, "an empty collector is not a request")
}

func TestPickupPersonAllowed(t *testing.T) {
	t.Parallel()

	assert.True(t, PickupPersonAllowed(""))
	assert.True(t, PickupPersonAllowed("pickup"))
	assert.False(t, PickupPersonAllowed("alone"))
	assert.False(t, PickupPersonAllowed("bus"))
}

func TestSignatureReference(t *testing.T) {
	t.Parallel()

	statement := &SignedStatement{SignatureID: 5, Statement: "Ich erlaube …"}
	raw, err := WithSignature(json.RawMessage(`{"weekdays":[{"weekday":2,"mode":"alone"}]}`), statement)
	require.NoError(t, err)
	signed := StoredSignature(raw)
	require.NotNil(t, signed)
	assert.Equal(t, statement.SignatureID, signed.SignatureID)

	unchanged, err := WithSignature(json.RawMessage(`{"a":1}`), nil)
	require.NoError(t, err)
	assert.JSONEq(t, `{"a":1}`, string(unchanged))
	assert.Nil(t, StoredSignature(unchanged))

	_, err = WithSignature(json.RawMessage(`{}`), &SignedStatement{SignatureID: 0, Statement: "x"})
	assert.ErrorIs(t, err, ErrInvalidPayload, "a reference needs a stored signature")
	assert.Nil(t, StoredSignature(json.RawMessage(`{"signature":{"id":3,"statement":"  "}}`)))
}

func TestLedgerRequestTypeOfAPickupPermission(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "pickup_change", LedgerRequestType(KindPickupPermission))
	assert.Equal(t, "care_schedule", LedgerRequestType("weekly_schedule"))
}
