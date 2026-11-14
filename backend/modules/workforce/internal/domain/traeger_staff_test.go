package domain

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func account(id int64) *int64 { return new(id) }

func TestSuggestStaffLinksFollowsTheOrderOfADR0089(t *testing.T) {
	t.Parallel()
	candidates := []StaffCandidate{
		{SchoolID: 10, StaffID: 1, AccountID: account(7), FirstName: "Mira", LastName: "Sommer"},
		{SchoolID: 20, StaffID: 2, AccountID: account(7), FirstName: "Mira", LastName: "Sommer-Berg"},
		{SchoolID: 10, StaffID: 3, FirstName: "Jonas", LastName: "Keller", PersonnelNumber: "70012"},
		{SchoolID: 30, StaffID: 4, FirstName: "Jonas", LastName: "Keler", PersonnelNumber: "70012"},
		{SchoolID: 10, StaffID: 5, FirstName: "Lea", LastName: "Brandt", Birthday: "1990-04-02"},
		{SchoolID: 20, StaffID: 6, FirstName: "lea ", LastName: "BRANDT", Birthday: "1990-04-02"},
		{SchoolID: 20, StaffID: 7, FirstName: "Lea", LastName: "Brandt"},
		{SchoolID: 30, StaffID: 8, FirstName: "Ole", LastName: "Wendt", PersonnelNumber: "70099"},
		{SchoolID: 20, StaffID: 9, FirstName: "Tim", LastName: "Roth"},
	}
	records := []StaffMatchRecord{{ID: 42, FirstName: "Ole", LastName: "Wendt", PersonnelNumber: "70099", SchoolIDs: []int64{10}}}

	suggestions := SuggestStaffLinks(records, candidates)

	reasons := make([]string, 0, len(suggestions))
	for _, suggestion := range suggestions {
		reasons = append(reasons, suggestion.Reason)
	}
	assert.Equal(t, []string{StaffMatchPersonnelNumber, StaffMatchAccount, StaffMatchPersonnelNumber, StaffMatchNameBirthday, StaffMatchNone, StaffMatchNone}, reasons,
		"a match to an existing record first, then groups by account, number, name and birthday, then the rest")
	assert.Equal(t, int64(42), suggestions[0].RecordID)
	assert.Equal(t, []int64{8}, staffIDs(suggestions[0].Candidates))
	assert.Equal(t, []int64{1, 2}, staffIDs(suggestions[1].Candidates), "the same account at two schools, even with another surname")
	assert.Equal(t, []int64{3, 4}, staffIDs(suggestions[2].Candidates))
	assert.Equal(t, []int64{5, 6}, staffIDs(suggestions[3].Candidates), "name and birthday compare without case and space")
	assert.Equal(t, []int64{7}, staffIDs(suggestions[4].Candidates), "a name without birthday is no match")
	assert.Equal(t, []int64{9}, staffIDs(suggestions[5].Candidates))
}

func TestSuggestStaffLinksNeverPutsTwoEmploymentsOfOneSchoolTogether(t *testing.T) {
	t.Parallel()
	candidates := []StaffCandidate{
		{SchoolID: 10, StaffID: 1, PersonnelNumber: "70001"},
		{SchoolID: 10, StaffID: 2, PersonnelNumber: "70001"},
		{SchoolID: 20, StaffID: 3, PersonnelNumber: "70001"},
	}
	records := []StaffMatchRecord{{ID: 5, PersonnelNumber: "70002", SchoolIDs: []int64{20}}}

	suggestions := SuggestStaffLinks(records, candidates)

	require.Len(t, suggestions, 2)
	assert.Equal(t, []int64{1, 3}, staffIDs(suggestions[0].Candidates))
	assert.Equal(t, []int64{2}, staffIDs(suggestions[1].Candidates), "the second employment of the same school stays on its own")
}

func TestResolveStaffFieldsTakesTheOnlyValueAndAsksOnConflict(t *testing.T) {
	t.Parallel()
	a := StaffSource{Key: "10", SchoolID: 10, Values: map[string]string{
		StaffFieldFirstName: "Mira", StaffFieldLastName: "Sommer", StaffFieldIBAN: "DE89 3704 0044 0532 0130 00",
	}}
	b := StaffSource{Key: "20", SchoolID: 20, Values: map[string]string{
		StaffFieldFirstName: "mira", StaffFieldLastName: "Sommer-Berg", StaffFieldIBAN: "DE89370400440532013000", StaffFieldPersonnelNumber: "70001",
	}}

	_, err := ResolveStaffFields([]StaffSource{a, b}, nil)
	var choice *StaffChoiceError
	require.ErrorAs(t, err, &choice)
	assert.Equal(t, StaffFieldLastName, choice.Field)
	assert.True(t, errors.Is(err, ErrStaffChoiceMissing))

	_, err = ResolveStaffFields([]StaffSource{a, b}, map[string]string{StaffFieldLastName: "30"})
	assert.ErrorIs(t, err, ErrStaffChoiceUnknown)

	resolved, err := ResolveStaffFields([]StaffSource{a, b}, map[string]string{StaffFieldLastName: "20", StaffFieldFirstName: "20"})
	require.NoError(t, err)
	assert.Equal(t, "Mira", resolved.Values[StaffFieldFirstName], "equal names are one value; a choice without conflict is ignored")
	assert.Equal(t, "Sommer-Berg", resolved.Values[StaffFieldLastName])
	assert.Equal(t, "70001", resolved.Values[StaffFieldPersonnelNumber], "a value only one school has is taken")
	assert.Equal(t, "DE89 3704 0044 0532 0130 00", resolved.Values[StaffFieldIBAN], "an IBAN with spaces equals the same IBAN without")
	require.Len(t, resolved.NotChosen, 1)
	assert.Equal(t, StaffNotChosen{Field: StaffFieldLastName, Value: "Sommer", Source: a}, resolved.NotChosen[0],
		"the value not taken keeps its school for the trail")
}

func TestResolveStaffFieldsKeepsNoRecordValueAsNotChosen(t *testing.T) {
	t.Parallel()
	record := StaffSource{Key: StaffSourceRecord, Values: map[string]string{StaffFieldPersonnelNumber: "70001"}}
	school := StaffSource{Key: "20", SchoolID: 20, Values: map[string]string{StaffFieldPersonnelNumber: "70002"}}

	resolved, err := ResolveStaffFields([]StaffSource{record, school}, map[string]string{StaffFieldPersonnelNumber: "20"})

	require.NoError(t, err)
	assert.Equal(t, "70002", resolved.Values[StaffFieldPersonnelNumber])
	assert.Empty(t, resolved.NotChosen, "the record's replaced value is a change of the record, not a value of a school")
}

func staffIDs(candidates []StaffCandidate) []int64 {
	ids := make([]int64, 0, len(candidates))
	for _, candidate := range candidates {
		ids = append(ids, candidate.StaffID)
	}
	return ids
}
