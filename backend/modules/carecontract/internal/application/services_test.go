package application

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
)

// The contract prints every weekday from Monday to Friday: the modules of
// the day with their end times, or "keine Betreuung" (#4678).
func TestServiceFactsNameEveryWeekdayWithItsModules(t *testing.T) {
	t.Parallel()
	facts := ServiceFacts([]carecontract.Service{
		{Weekday: "mon", CareOfferingID: 3, ModuleName: "Frühbetreuung", EndsAt: "08:00"},
		{Weekday: "mon", CareOfferingID: 1, ModuleName: "Kurzbetreuung", EndsAt: "14:00"},
		{Weekday: "tue", CareOfferingID: 2, ModuleName: "Ganztag", EndsAt: "16:30"},
		{Weekday: "thu", CareOfferingID: 4, ModuleName: "Hausaufgaben"},
	})
	assert.Equal(t, []LetterFact{
		{Label: "Montag", Value: "Frühbetreuung bis 08:00 Uhr und Kurzbetreuung bis 14:00 Uhr"},
		{Label: "Dienstag", Value: "Ganztag bis 16:30 Uhr"},
		{Label: "Mittwoch", Value: "keine Betreuung"},
		{Label: "Donnerstag", Value: "Hausaufgaben"},
		{Label: "Freitag", Value: "keine Betreuung"},
	}, facts)
}

// One contract carries one set of conditions: every module needs its
// conditions, and they must agree.
func TestContractTermsNeedEqualConditionsOfEveryModule(t *testing.T) {
	t.Parallel()
	twelve := 12
	fixed := carecontract.TemplateInput{TermKind: carecontract.TermFixedMonths, TermMonths: &twelve, NoticeMonths: 1,
		NoticeTo: carecontract.NoticeMonthEnd}
	sameMonths := 12
	copied := fixed
	copied.TermMonths = &sameMonths
	index := templateIndex{{7, 1}: fixed, {7, 2}: copied, {7, 3}: {TermKind: carecontract.TermOpenEnded, NoticeTo: carecontract.NoticeMonthEnd}}

	terms, issue := index.terms(7, []int64{1, 2})
	assert.Empty(t, issue, "equal conditions behind two pointers are equal")
	assert.Equal(t, fixed.TermKind, terms.TermKind)
	_, issue = index.terms(7, []int64{1, 3})
	assert.Equal(t, carecontract.SkipTemplatesDiffer, issue)
	_, issue = index.terms(7, []int64{1, 4})
	assert.Equal(t, carecontract.SkipTemplateMissing, issue)
	_, issue = index.terms(8, []int64{1})
	assert.Equal(t, carecontract.SkipTemplateMissing, issue, "conditions belong to the school's offering")
}
