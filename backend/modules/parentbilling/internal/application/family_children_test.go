package application

import (
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The state of a family's number today (#5918): the most urgent check wins,
// and the expiry notice starts 30 days before the end.
func TestFamilyChildrenStatus(t *testing.T) {
	t.Parallel()
	today := calendar.NewDate(2027, 3, 10)
	confirmedAt := time.Date(2027, 1, 5, 9, 0, 0, 0, time.UTC)
	later := confirmedAt.Add(48 * time.Hour)
	confirmed := func(children int, from calendar.Date, until *calendar.Date) *parentbilling.FamilyChildCount {
		return &parentbilling.FamilyChildCount{ConfirmedChildren: ptr(children), ValidFrom: &from, ValidUntil: until, ConfirmedAt: &confirmedAt}
	}
	changed := confirmed(3, calendar.NewDate(2027, 1, 1), nil)
	changed.StatedChildren, changed.StatedAt = ptr(4), &later
	same := confirmed(3, calendar.NewDate(2027, 1, 1), nil)
	same.StatedChildren, same.StatedAt = ptr(3), &later
	cases := []struct {
		name  string
		count *parentbilling.FamilyChildCount
		want  string
	}{
		{"nobody stated a number", nil, parentbilling.FamilyChildrenMissing},
		{"the parents stated, the office did not confirm",
			&parentbilling.FamilyChildCount{StatedChildren: ptr(3), StatedAt: &later}, parentbilling.FamilyChildrenStated},
		{"confirmed and open", confirmed(3, calendar.NewDate(2027, 1, 1), nil), parentbilling.FamilyChildrenConfirmed},
		{"ended yesterday", confirmed(3, calendar.NewDate(2026, 9, 1), ptr(calendar.NewDate(2027, 3, 9))), parentbilling.FamilyChildrenExpired},
		{"ends in 30 days", confirmed(3, calendar.NewDate(2026, 9, 1), ptr(calendar.NewDate(2027, 4, 9))), parentbilling.FamilyChildrenExpiring},
		{"ends in 31 days", confirmed(3, calendar.NewDate(2026, 9, 1), ptr(calendar.NewDate(2027, 4, 10))), parentbilling.FamilyChildrenConfirmed},
		{"starts later", confirmed(3, calendar.NewDate(2027, 4, 1), nil), parentbilling.FamilyChildrenUpcoming},
		{"the parents stated another number after the confirmation", changed, parentbilling.FamilyChildrenChanged},
		{"the parents stated the same number again", same, parentbilling.FamilyChildrenConfirmed},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, familyChildrenStatus(tc.count, today), tc.name)
	}
}

// A number is a whole number from 1 to 20 with a start; the end is not
// before it.
func TestFamilyChildrenInputIsChecked(t *testing.T) {
	t.Parallel()
	from := calendar.NewDate(2027, 1, 1)
	valid := parentbilling.FamilyChildrenInput{ContractID: 1, Children: 3, ValidFrom: from}
	assert.NoError(t, validFamilyChildrenInput(valid))
	for name, input := range map[string]parentbilling.FamilyChildrenInput{
		"no child":      {ContractID: 1, Children: 0, ValidFrom: from},
		"negative":      {ContractID: 1, Children: -2, ValidFrom: from},
		"more than 20":  {ContractID: 1, Children: 21, ValidFrom: from},
		"no start":      {ContractID: 1, Children: 3},
		"end before it": {ContractID: 1, Children: 3, ValidFrom: from, ValidUntil: ptr(calendar.NewDate(2026, 12, 31))},
	} {
		assert.Error(t, validFamilyChildrenInput(input), name)
	}
}

// Successive contracts must not turn one child into several siblings.
func TestFamilyChildrenGroupsDistinctChildren(t *testing.T) {
	t.Parallel()
	today := calendar.NewDate(2027, 3, 10)
	birth := calendar.NewDate(2019, 4, 2)
	versions := []parentbilling.CareContractVersion{
		{ContractID: 1, StudentID: ptr(int64(10)), ChildFirstName: "Mia", ChildLastName: "Meier", ChildBirthDate: &birth},
		{ContractID: 2, StudentID: ptr(int64(10)), ChildFirstName: "Mia", ChildLastName: "Meier", ChildBirthDate: &birth},
		{ContractID: 3, ChildFirstName: "Mia", ChildLastName: "Meier", ChildBirthDate: &birth},
		{ContractID: 4, StudentID: ptr(int64(11)), ChildFirstName: "Mia", ChildLastName: "Meier", ChildBirthDate: &birth},
	}
	recipients := map[int64]parentbilling.CareRecipient{}
	for _, version := range versions {
		recipients[version.ContractID] = parentbilling.CareRecipient{Email: "familie@example.org"}
	}
	families := groupFamilies(versions, recipients, nil, today)
	assert.Len(t, families, 1)
	if assert.Len(t, families[0].Children, 3) {
		assert.Equal(t, []int64{1, 3, 4}, []int64{families[0].Children[0].ContractID, families[0].Children[1].ContractID, families[0].Children[2].ContractID})
	}
	// Without a second matching record, the unlinked contract joins Mia.
	families = groupFamilies(versions[:3], recipients, nil, today)
	assert.Len(t, families[0].Children, 1)
}

func TestFamilyChildrenRetainsDatedConfirmations(t *testing.T) {
	t.Parallel()
	oldStart, futureStart := calendar.NewDate(2027, 1, 1), calendar.NewDate(2027, 4, 1)
	count := parentbilling.FamilyChildCount{ConfirmedChildren: ptr(2), ValidFrom: &futureStart,
		Confirmations: []parentbilling.FamilyChildConfirmation{{Children: ptr(3), ValidFrom: &oldStart}}}
	number, found := count.ConfirmedIn(calendar.NewDate(2027, 3, 1))
	assert.True(t, found)
	assert.Equal(t, 3, number)
	number, found = count.ConfirmedIn(calendar.NewDate(2027, 4, 1))
	assert.True(t, found)
	assert.Equal(t, 2, number, "the latest applicable confirmation wins")
	_, found = count.ConfirmedIn(calendar.NewDate(2026, 12, 1))
	assert.False(t, found)
}

func TestFamilyChildrenSupersedesExpiredConfirmations(t *testing.T) {
	t.Parallel()
	january, april, july := calendar.NewDate(2027, 1, 1), calendar.NewDate(2027, 4, 1), calendar.NewDate(2027, 7, 1)
	januaryEnd, mayEnd, augustEnd := calendar.NewDate(2027, 1, 31), calendar.NewDate(2027, 5, 31), calendar.NewDate(2027, 8, 31)
	old := parentbilling.FamilyChildConfirmation{Children: ptr(3), ValidFrom: &january}
	corrected := parentbilling.FamilyChildCount{ConfirmedChildren: ptr(3), ValidFrom: &january, ValidUntil: &januaryEnd,
		Confirmations: []parentbilling.FamilyChildConfirmation{old}}
	future := parentbilling.FamilyChildCount{ConfirmedChildren: ptr(2), ValidFrom: &april, ValidUntil: &mayEnd,
		Confirmations: []parentbilling.FamilyChildConfirmation{old}}
	later := parentbilling.FamilyChildCount{ConfirmedChildren: ptr(4), ValidFrom: &july, ValidUntil: &augustEnd,
		Confirmations: []parentbilling.FamilyChildConfirmation{
			{Children: ptr(2), ValidFrom: &april, ValidUntil: &mayEnd}, old,
		}}
	for _, tc := range []struct {
		name     string
		count    parentbilling.FamilyChildCount
		month    calendar.Date
		children int
		found    bool
	}{
		{"corrected interval remains valid before its end", corrected, january, 3, true},
		{"corrected expiry does not revive the open interval", corrected, calendar.NewDate(2027, 2, 1), 0, false},
		{"future confirmation preserves earlier months", future, calendar.NewDate(2027, 3, 1), 3, true},
		{"successor includes its last month", future, calendar.NewDate(2027, 5, 1), 2, true},
		{"expired successor does not revive its predecessor", future, calendar.NewDate(2027, 6, 1), 0, false},
		{"expired history does not revive an older interval", later, calendar.NewDate(2027, 6, 1), 0, false},
		{"history before a successor remains valid", later, calendar.NewDate(2027, 3, 1), 3, true},
		{"newest expiry supersedes all history", later, calendar.NewDate(2027, 9, 1), 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			children, found := tc.count.ConfirmedIn(tc.month)
			assert.Equal(t, tc.found, found)
			assert.Equal(t, tc.children, children)
		})
	}
}

// A confirmation that ended and a later one that has not started leave a
// gap: the family needs a check now, not „Gilt später“. Without an earlier
// confirmation a later one is upcoming.
func TestFamilyChildrenStatusShowsTheGapBeforeALaterConfirmation(t *testing.T) {
	t.Parallel()
	today := calendar.NewDate(2026, 12, 10)
	confirmedAt := time.Date(2026, 12, 1, 9, 0, 0, 0, time.UTC)
	later := parentbilling.FamilyChildCount{ConfirmedChildren: ptr(3), ValidFrom: ptr(calendar.NewDate(2027, 1, 1)), ConfirmedAt: &confirmedAt,
		Confirmations: []parentbilling.FamilyChildConfirmation{
			{Children: ptr(3), ValidFrom: ptr(calendar.NewDate(2027, 1, 1))},
			{Children: ptr(2), ValidFrom: ptr(calendar.NewDate(2026, 8, 1)), ValidUntil: ptr(calendar.NewDate(2026, 11, 30))},
		}}
	assert.Equal(t, parentbilling.FamilyChildrenExpired, familyChildrenStatus(&later, today), "December has no number")
	covered := later
	covered.Confirmations = []parentbilling.FamilyChildConfirmation{
		{Children: ptr(3), ValidFrom: ptr(calendar.NewDate(2027, 1, 1))},
		{Children: ptr(2), ValidFrom: ptr(calendar.NewDate(2026, 8, 1)), ValidUntil: ptr(calendar.NewDate(2026, 12, 31))},
	}
	assert.Equal(t, parentbilling.FamilyChildrenUpcoming, familyChildrenStatus(&covered, today), "the earlier one holds until the later starts")
	// November: the earlier confirmation still holds but ends within the
	// notice and leaves December without a number: check it now.
	assert.Equal(t, parentbilling.FamilyChildrenExpiring, familyChildrenStatus(&later, calendar.NewDate(2026, 11, 10)),
		"the warning comes before the gap")
	assert.Equal(t, parentbilling.FamilyChildrenUpcoming, familyChildrenStatus(&covered, calendar.NewDate(2026, 12, 10)),
		"no gap, no warning")
	first := later
	first.Confirmations = nil
	assert.Equal(t, parentbilling.FamilyChildrenUpcoming, familyChildrenStatus(&first, today), "a first confirmation that starts later")
}

// Confirmations for September to November and for 1 to 10 December, then
// one from January: 11 to 31 December have no number. On 20 November that
// gap lies within the notice, behind the confirmation that follows the
// current one: the family needs a check now, not on 1 December.
func TestFamilyChildrenStatusFollowsTheConfirmationsToTheFirstGap(t *testing.T) {
	t.Parallel()
	confirmedAt := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	count := parentbilling.FamilyChildCount{ConfirmedChildren: ptr(3), ValidFrom: ptr(calendar.NewDate(2027, 1, 1)), ConfirmedAt: &confirmedAt,
		Confirmations: []parentbilling.FamilyChildConfirmation{
			{Children: ptr(3), ValidFrom: ptr(calendar.NewDate(2027, 1, 1))},
			{Children: ptr(2), ValidFrom: ptr(calendar.NewDate(2026, 12, 1)), ValidUntil: ptr(calendar.NewDate(2026, 12, 10))},
			{Children: ptr(2), ValidFrom: ptr(calendar.NewDate(2026, 9, 1)), ValidUntil: ptr(calendar.NewDate(2026, 11, 30))},
		}}
	assert.Equal(t, parentbilling.FamilyChildrenExpiring, familyChildrenStatus(&count, calendar.NewDate(2026, 11, 20)),
		"the gap from 11 December lies within the notice")
	assert.Equal(t, parentbilling.FamilyChildrenUpcoming, familyChildrenStatus(&count, calendar.NewDate(2026, 10, 1)),
		"the gap lies beyond the notice")
	closed := count
	closed.Confirmations = slices.Clone(count.Confirmations)
	closed.Confirmations[1].ValidUntil = ptr(calendar.NewDate(2026, 12, 31))
	assert.Equal(t, parentbilling.FamilyChildrenUpcoming, familyChildrenStatus(&closed, calendar.NewDate(2026, 11, 20)),
		"the chain reaches the later confirmation without a gap")
}

// A family that stated its number on a contract not yet in force shows for
// the check, with the recipient of that contract and no child with a
// contract.
func TestFamilyChildrenListsAStatementOnASentContract(t *testing.T) {
	t.Parallel()
	statedAt := time.Date(2026, 9, 5, 8, 0, 0, 0, time.UTC)
	counts := []parentbilling.FamilyChildCount{{ID: 5, ContractID: 77, FamilyEmail: "neu@example.org", StatedChildren: ptr(3), StatedAt: &statedAt}}
	recipients := map[int64]parentbilling.CareRecipient{77: {FirstName: "Nora", LastName: "Neu", Email: "Neu@example.org"}}
	families := groupFamilies(nil, recipients, counts, calendar.NewDate(2026, 9, 9))
	require.Len(t, families, 1)
	assert.Equal(t, "Nora", families[0].RecipientFirstName)
	assert.Equal(t, "neu@example.org", families[0].RecipientEmail)
	assert.Empty(t, families[0].Children)
	require.NotNil(t, families[0].Count)
	assert.Equal(t, parentbilling.FamilyChildrenStated, families[0].Status)
}

// A run of some schools names only the families without a confirmed number
// whose invoices it bills, and its fingerprint changes when that number
// does: confirming a family must not let an old review be released.
func TestUncountedFamiliesFollowTheRunAndItsFingerprint(t *testing.T) {
	t.Parallel()
	month := careMonth{uncounted: 2, uncountedSchools: map[string][]int64{
		"eva@example.org":   {schoolIspringen},
		"jonas@example.org": {schoolBirkenfeld},
	}}
	ispringen := narrowMonth(month, []int64{schoolIspringen}, nil)
	assert.Equal(t, 1, ispringen.uncounted, "only the Ispringen family")
	assert.Equal(t, 2, narrowMonth(month, []int64{schoolIspringen, schoolBirkenfeld}, nil).uncounted)
	assert.Zero(t, narrowMonth(month, []int64{999}, nil).uncounted, "another school names none")

	run := parentbilling.CareRun{ID: 7, SchoolIDs: []int64{schoolIspringen}}
	before := previewFingerprint(run, parentbilling.MonthRuleVersionOnFirst, ispringen)
	confirmed := narrowMonth(careMonth{uncountedSchools: map[string][]int64{}}, []int64{schoolIspringen}, nil)
	assert.NotEqual(t, before, previewFingerprint(run, parentbilling.MonthRuleVersionOnFirst, confirmed),
		"the confirmation changes what the reviewer saw")
}

// Two confirmations in one month: the one in force on the first counts,
// like an evidence; a confirmation from mid-month counts only when none
// holds on the first.
func TestFamilyChildrenConfirmedInTheMonthTakesTheFirstDay(t *testing.T) {
	t.Parallel()
	count := parentbilling.FamilyChildCount{ConfirmedChildren: ptr(2), ValidFrom: ptr(calendar.NewDate(2026, 11, 15)),
		Confirmations: []parentbilling.FamilyChildConfirmation{
			{Children: ptr(2), ValidFrom: ptr(calendar.NewDate(2026, 11, 15))},
			{Children: ptr(3), ValidFrom: ptr(calendar.NewDate(2026, 9, 1))},
		}}
	children, ok := count.ConfirmedIn(calendar.NewDate(2026, 11, 1))
	require.True(t, ok)
	assert.Equal(t, 3, children, "3 held on 1 November")
	children, ok = count.ConfirmedIn(calendar.NewDate(2026, 12, 1))
	require.True(t, ok)
	assert.Equal(t, 2, children, "from December the newer one")
	first := parentbilling.FamilyChildCount{ConfirmedChildren: ptr(2), ValidFrom: ptr(calendar.NewDate(2026, 11, 15))}
	children, ok = first.ConfirmedIn(calendar.NewDate(2026, 11, 1))
	require.True(t, ok)
	assert.Equal(t, 2, children, "a first confirmation from mid-month holds for it")
}

// A family with invoices at Ispringen and Birkenfeld, a rule that counts
// the family only at Ispringen: the missing number concerns Ispringen
// alone, so a run of Birkenfeld names no family.
func TestUncountedFamiliesNameOnlyTheSchoolsOfTheCountingRule(t *testing.T) {
	t.Parallel()
	in := careMonthInput{month: day(t, billingMonth), rule: parentbilling.MonthRuleVersionOnFirst, tariffs: chainTariffs(t),
		invoiced: map[int64]parentbilling.CareInvoice{}, recipients: map[int64]parentbilling.CareRecipient{},
		rules: familySiblingRule(true)(t)}
	for _, child := range []careChainChild{
		{contractMia, schoolIspringen, familyEva, "2017-03-01", []parentbilling.CareContractService{mon1400}},
		{contractNoah, schoolBirkenfeld, familyEva, "2015-03-01", []parentbilling.CareContractService{mon1400}},
	} {
		version := sjrVersion(t, child.contract, child.school, 1, billingMonth, "2027-01-31", child.booked...)
		born := day(t, child.born)
		version.ChildBirthDate = &born
		in.versions = append(in.versions, version)
		in.recipients[child.contract] = parentbilling.CareRecipient{FirstName: "Eva", LastName: "Beispiel", Email: child.family}
	}
	month, err := buildCareMonth(in)
	require.NoError(t, err)
	assert.Equal(t, map[string][]int64{familyEva: {schoolIspringen}}, month.uncountedSchools)
	assert.Equal(t, 1, narrowMonth(month, []int64{schoolIspringen}, nil).uncounted)
	assert.Zero(t, narrowMonth(month, []int64{schoolBirkenfeld}, nil).uncounted, "Birkenfeld has no counting rule")
}

// September to November, December, January onward: the family is covered
// without a break, so November asks for no check.
func TestFamilyChildrenStatusSeesAConfirmationInBetween(t *testing.T) {
	t.Parallel()
	count := parentbilling.FamilyChildCount{ConfirmedChildren: ptr(3), ValidFrom: ptr(calendar.NewDate(2027, 1, 1)),
		Confirmations: []parentbilling.FamilyChildConfirmation{
			{Children: ptr(3), ValidFrom: ptr(calendar.NewDate(2027, 1, 1))},
			{Children: ptr(3), ValidFrom: ptr(calendar.NewDate(2026, 12, 1)), ValidUntil: ptr(calendar.NewDate(2026, 12, 31))},
			{Children: ptr(3), ValidFrom: ptr(calendar.NewDate(2026, 9, 1)), ValidUntil: ptr(calendar.NewDate(2026, 11, 30))},
		}}
	assert.Equal(t, parentbilling.FamilyChildrenUpcoming, familyChildrenStatus(&count, calendar.NewDate(2026, 11, 10)))
}
