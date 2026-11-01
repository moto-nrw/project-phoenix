package application

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// payrollRulesStoreFake keeps the rows in memory like the tenant tables do.
type payrollRulesStoreFake struct {
	wageTypes    []workforce.PayrollWageType
	rules        []workforce.PayrollSurchargeRule
	absenceTypes []workforce.PayrollAbsenceTypeWageType
	nextID       int64
	locks        int
}

func (f *payrollRulesStoreFake) id() int64 { f.nextID++; return f.nextID }

func (f *payrollRulesStoreFake) LockPayrollRules(context.Context) error { f.locks++; return nil }

func (f *payrollRulesStoreFake) ListPayrollWageTypes(context.Context) ([]workforce.PayrollWageType, error) {
	return append([]workforce.PayrollWageType(nil), f.wageTypes...), nil
}

func (f *payrollRulesStoreFake) InsertPayrollWageType(_ context.Context, value workforce.PayrollWageType) (workforce.PayrollWageType, error) {
	for _, existing := range f.wageTypes {
		if !existing.Archived && existing.Name == value.Name {
			return workforce.PayrollWageType{}, workforce.ErrPayrollWageTypeNameTaken
		}
	}
	value.ID = f.id()
	f.wageTypes = append(f.wageTypes, value)
	return value, nil
}

func (f *payrollRulesStoreFake) UpdatePayrollWageType(_ context.Context, value workforce.PayrollWageType) (bool, error) {
	for i, existing := range f.wageTypes {
		if existing.ID == value.ID && !existing.Archived {
			f.wageTypes[i] = value
			return true, nil
		}
	}
	return false, nil
}

func (f *payrollRulesStoreFake) ListPayrollSurchargeRules(context.Context) ([]workforce.PayrollSurchargeRule, error) {
	return append([]workforce.PayrollSurchargeRule(nil), f.rules...), nil
}

func (f *payrollRulesStoreFake) InsertPayrollSurchargeRule(_ context.Context, value workforce.PayrollSurchargeRule) (workforce.PayrollSurchargeRule, error) {
	value.ID = f.id()
	f.rules = append(f.rules, value)
	return value, nil
}

func (f *payrollRulesStoreFake) UpdatePayrollSurchargeRule(_ context.Context, value workforce.PayrollSurchargeRule) (bool, error) {
	for i, existing := range f.rules {
		if existing.ID == value.ID && !existing.Archived {
			f.rules[i] = value
			return true, nil
		}
	}
	return false, nil
}

func (f *payrollRulesStoreFake) ListPayrollAbsenceTypes(context.Context) ([]workforce.PayrollAbsenceTypeWageType, error) {
	return append([]workforce.PayrollAbsenceTypeWageType(nil), f.absenceTypes...), nil
}

func (f *payrollRulesStoreFake) SetAbsenceTypeWageType(_ context.Context, absenceTypeID int64, wageTypeID *int64) (bool, error) {
	for i, existing := range f.absenceTypes {
		if existing.AbsenceTypeID == absenceTypeID {
			f.absenceTypes[i].WageTypeID = wageTypeID
			return true, nil
		}
	}
	return false, nil
}

type payrollAuditFake struct{ changes []workforce.PayrollRuleChange }

func (a *payrollAuditFake) RecordPayrollRuleChange(_ context.Context, change workforce.PayrollRuleChange) error {
	a.changes = append(a.changes, change)
	return nil
}

const payrollActor = int64(42)

func newPayrollRulesForTest(t *testing.T) (*PayrollRules, *payrollRulesStoreFake, *payrollAuditFake) {
	t.Helper()
	store, audit := &payrollRulesStoreFake{}, &payrollAuditFake{}
	rules, err := NewPayrollRules(store, audit)
	require.NoError(t, err)
	return rules, store, audit
}

func TestPayrollRules_CreateWageTypeTrimsAndAuditsTheNewObject(t *testing.T) {
	t.Parallel()
	rules, _, audit := newPayrollRulesForTest(t)

	created, err := rules.CreateWageType(t.Context(), payrollActor, workforce.PayrollWageTypeInput{Name: "  Nachtzuschlag ", Number: " 310 ", Unit: workforce.WageTypeUnitHours})

	require.NoError(t, err)
	assert.Equal(t, "Nachtzuschlag", created.Name)
	assert.Equal(t, "310", created.Number)
	require.Len(t, audit.changes, 1)
	change := audit.changes[0]
	assert.Equal(t, "payroll.wage_type."+jsonID(created.ID), change.Key)
	assert.Equal(t, "set", change.Action)
	assert.Nil(t, change.Old)
	assert.JSONEq(t, `{"id":`+jsonID(created.ID)+`,"name":"Nachtzuschlag","number":"310","unit":"stunden","archived":false}`, string(change.New))
	assert.Equal(t, payrollActor, change.ActorAccountID)
}

func TestPayrollRules_CreateWageTypeRejectsWhatDATEVCannotBook(t *testing.T) {
	t.Parallel()
	rules, _, audit := newPayrollRulesForTest(t)
	for name, input := range map[string]workforce.PayrollWageTypeInput{
		"no name":         {Name: " ", Unit: workforce.WageTypeUnitHours},
		"five digits":     {Name: "Zuschlag", Number: "12345", Unit: workforce.WageTypeUnitHours},
		"letters":         {Name: "Zuschlag", Number: "12a", Unit: workforce.WageTypeUnitHours},
		"unknown unit":    {Name: "Zuschlag", Unit: "minuten"},
		"too long a name": {Name: string(make([]rune, 101)), Unit: workforce.WageTypeUnitHours},
	} {
		_, err := rules.CreateWageType(t.Context(), payrollActor, input)
		assert.ErrorIs(t, err, workforce.ErrPayrollRuleInvalid, name)
	}
	assert.Empty(t, audit.changes)
}

func TestPayrollRules_ANightRuleWithoutWindowGetsTheDefaultWindow(t *testing.T) {
	t.Parallel()
	rules, _, _ := newPayrollRulesForTest(t)
	wageType, err := rules.CreateWageType(t.Context(), payrollActor, workforce.PayrollWageTypeInput{Name: "Nacht", Unit: workforce.WageTypeUnitHours})
	require.NoError(t, err)

	rule, err := rules.CreateSurchargeRule(t.Context(), payrollActor, workforce.PayrollSurchargeRuleInput{
		Kind: workforce.SurchargeKindNight, RateType: workforce.SurchargeRatePercent, RateHundredths: 2500, WageTypeID: wageType.ID,
	})

	require.NoError(t, err)
	assert.Equal(t, 20*60, rule.WindowStartMinute)
	assert.Equal(t, 6*60, rule.WindowEndMinute)
}

func TestPayrollRules_ARuleBooksOntoAWageTypeOfTheMatchingUnit(t *testing.T) {
	t.Parallel()
	rules, _, _ := newPayrollRulesForTest(t)
	hours, err := rules.CreateWageType(t.Context(), payrollActor, workforce.PayrollWageTypeInput{Name: "Stunden", Unit: workforce.WageTypeUnitHours})
	require.NoError(t, err)
	amount, err := rules.CreateWageType(t.Context(), payrollActor, workforce.PayrollWageTypeInput{Name: "Betrag", Unit: workforce.WageTypeUnitAmount})
	require.NoError(t, err)
	sunday := func(rateType string, wageTypeID int64) workforce.PayrollSurchargeRuleInput {
		return workforce.PayrollSurchargeRuleInput{Kind: workforce.SurchargeKindSunday, RateType: rateType, RateHundredths: 500, WageTypeID: wageTypeID}
	}

	_, err = rules.CreateSurchargeRule(t.Context(), payrollActor, sunday(workforce.SurchargeRatePercent, amount.ID))
	assert.ErrorIs(t, err, workforce.ErrPayrollRuleInvalid, "a percent rule is booked as hours")
	_, err = rules.CreateSurchargeRule(t.Context(), payrollActor, sunday(workforce.SurchargeRateAmount, hours.ID))
	assert.ErrorIs(t, err, workforce.ErrPayrollRuleInvalid, "an amount rule is booked as an amount")
	_, err = rules.CreateSurchargeRule(t.Context(), payrollActor, sunday(workforce.SurchargeRateAmount, amount.ID+100))
	assert.ErrorIs(t, err, workforce.ErrPayrollRuleNotFound)
	start, end := 60, 120
	withWindow := sunday(workforce.SurchargeRatePercent, hours.ID)
	withWindow.WindowStartMinute, withWindow.WindowEndMinute = &start, &end
	_, err = rules.CreateSurchargeRule(t.Context(), payrollActor, withWindow)
	assert.ErrorIs(t, err, workforce.ErrPayrollRuleInvalid, "only night rules have a window")
	_, err = rules.CreateSurchargeRule(t.Context(), payrollActor, sunday(workforce.SurchargeRateAmount, amount.ID))
	assert.NoError(t, err)
}

func TestPayrollRules_AWageTypeInUseCannotBeArchived(t *testing.T) {
	t.Parallel()
	rules, store, audit := newPayrollRulesForTest(t)
	wageType, err := rules.CreateWageType(t.Context(), payrollActor, workforce.PayrollWageTypeInput{Name: "Sonntag", Unit: workforce.WageTypeUnitHours})
	require.NoError(t, err)
	rule, err := rules.CreateSurchargeRule(t.Context(), payrollActor, workforce.PayrollSurchargeRuleInput{
		Kind: workforce.SurchargeKindSunday, RateType: workforce.SurchargeRatePercent, RateHundredths: 5000, WageTypeID: wageType.ID,
	})
	require.NoError(t, err)

	assert.ErrorIs(t, rules.ArchiveWageType(t.Context(), payrollActor, wageType.ID), workforce.ErrPayrollWageTypeInUse)

	require.NoError(t, rules.ArchiveSurchargeRule(t.Context(), payrollActor, rule.ID))
	require.NoError(t, rules.ArchiveWageType(t.Context(), payrollActor, wageType.ID))
	assert.True(t, store.wageTypes[0].Archived)
	last := audit.changes[len(audit.changes)-1]
	assert.Equal(t, "delete", last.Action)
	assert.Equal(t, "payroll.wage_type."+jsonID(wageType.ID), last.Key)
	assert.NotNil(t, last.Old)
	assert.ErrorIs(t, rules.ArchiveWageType(t.Context(), payrollActor, wageType.ID), workforce.ErrPayrollRuleNotFound, "archived once")
}

func TestPayrollRules_AnAbsenceTypeBooksOntoAnHourOrDayWageType(t *testing.T) {
	t.Parallel()
	rules, store, audit := newPayrollRulesForTest(t)
	store.absenceTypes = []workforce.PayrollAbsenceTypeWageType{{AbsenceTypeID: 900, Name: "Sonderurlaub", IsActive: true}}
	days, err := rules.CreateWageType(t.Context(), payrollActor, workforce.PayrollWageTypeInput{Name: "Sonderurlaub", Number: "250", Unit: workforce.WageTypeUnitDays})
	require.NoError(t, err)
	amount, err := rules.CreateWageType(t.Context(), payrollActor, workforce.PayrollWageTypeInput{Name: "Betrag", Unit: workforce.WageTypeUnitAmount})
	require.NoError(t, err)

	assert.ErrorIs(t, rules.SetAbsenceTypeWageType(t.Context(), payrollActor, 900, &amount.ID), workforce.ErrPayrollRuleInvalid)
	assert.ErrorIs(t, rules.SetAbsenceTypeWageType(t.Context(), payrollActor, 901, &days.ID), workforce.ErrPayrollRuleNotFound)
	require.NoError(t, rules.SetAbsenceTypeWageType(t.Context(), payrollActor, 900, &days.ID))

	last := audit.changes[len(audit.changes)-1]
	assert.Equal(t, "payroll.absence_type_wage_type.900", last.Key)
	assert.JSONEq(t, `null`, string(last.Old))
	assert.JSONEq(t, jsonID(days.ID), string(last.New))

	set, err := rules.CurrentPayrollRuleSet(t.Context())
	require.NoError(t, err)
	assert.Equal(t, map[int64]int64{900: days.ID}, set.AbsenceTypeWageTypes)
	assert.ErrorIs(t, rules.ArchiveWageType(t.Context(), payrollActor, days.ID), workforce.ErrPayrollWageTypeInUse)
}

func TestPayrollRules_TheCurrentRuleSetLeavesArchivedEntriesOut(t *testing.T) {
	t.Parallel()
	rules, _, _ := newPayrollRulesForTest(t)
	kept, err := rules.CreateWageType(t.Context(), payrollActor, workforce.PayrollWageTypeInput{Name: "Nacht", Number: "310", Unit: workforce.WageTypeUnitHours})
	require.NoError(t, err)
	gone, err := rules.CreateWageType(t.Context(), payrollActor, workforce.PayrollWageTypeInput{Name: "Alt", Unit: workforce.WageTypeUnitHours})
	require.NoError(t, err)
	require.NoError(t, rules.ArchiveWageType(t.Context(), payrollActor, gone.ID))
	rule, err := rules.CreateSurchargeRule(t.Context(), payrollActor, workforce.PayrollSurchargeRuleInput{
		Kind: workforce.SurchargeKindNight, RateType: workforce.SurchargeRatePercent, RateHundredths: 2500, WageTypeID: kept.ID,
	})
	require.NoError(t, err)

	set, err := rules.CurrentPayrollRuleSet(t.Context())

	require.NoError(t, err)
	assert.Equal(t, []workforce.PayrollWageType{*kept}, set.WageTypes)
	assert.Equal(t, []workforce.PayrollSurchargeRule{*rule}, set.SurchargeRules)
	assert.Empty(t, set.AbsenceTypeWageTypes)
}

func TestPayrollRules_CurrentRuleSetLocksBeforeReadingTheRuleState(t *testing.T) {
	t.Parallel()
	rules, store, _ := newPayrollRulesForTest(t)

	_, err := rules.CurrentPayrollRuleSet(t.Context())

	require.NoError(t, err)
	assert.Equal(t, 1, store.locks)
}

func jsonID(id int64) string {
	data, _ := json.Marshal(id)
	return string(data)
}
