package application

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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
