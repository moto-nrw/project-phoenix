package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/ports"
)

// payrollRuleState is the school's wage types, rules and absence types as
// read under the rule lock; the write checks decide against it.
type payrollRuleState struct {
	wageTypes    []workforce.PayrollWageType
	rules        []workforce.PayrollSurchargeRule
	absenceTypes []workforce.PayrollAbsenceTypeWageType
}

func (p *PayrollRules) loadState(ctx context.Context) (*payrollRuleState, error) {
	return loadPayrollRuleState(ctx, p.store)
}

func loadPayrollRuleState(ctx context.Context, store ports.PayrollRulesStore) (*payrollRuleState, error) {
	state, err := store.ReadPayrollRuleState(ctx)
	if err != nil {
		return nil, err
	}
	return &payrollRuleState{wageTypes: state.WageTypes, rules: state.Rules, absenceTypes: state.AbsenceTypes}, nil
}

func (s *payrollRuleState) activeWageTypes() []workforce.PayrollWageType {
	result := make([]workforce.PayrollWageType, 0, len(s.wageTypes))
	for _, wageType := range s.wageTypes {
		if !wageType.Archived {
			result = append(result, wageType)
		}
	}
	return result
}

func (s *payrollRuleState) activeRules() []workforce.PayrollSurchargeRule {
	result := make([]workforce.PayrollSurchargeRule, 0, len(s.rules))
	for _, rule := range s.rules {
		if !rule.Archived {
			result = append(result, rule)
		}
	}
	return result
}

func (s *payrollRuleState) activeWageType(id int64) (workforce.PayrollWageType, bool) {
	for _, wageType := range s.wageTypes {
		if wageType.ID == id && !wageType.Archived {
			return wageType, true
		}
	}
	return workforce.PayrollWageType{}, false
}

func (s *payrollRuleState) activeRule(id int64) (workforce.PayrollSurchargeRule, bool) {
	for _, rule := range s.rules {
		if rule.ID == id && !rule.Archived {
			return rule, true
		}
	}
	return workforce.PayrollSurchargeRule{}, false
}

func (s *payrollRuleState) absenceType(id int64) (workforce.PayrollAbsenceTypeWageType, bool) {
	for _, absenceType := range s.absenceTypes {
		if absenceType.AbsenceTypeID == id {
			return absenceType, true
		}
	}
	return workforce.PayrollAbsenceTypeWageType{}, false
}

func (s *payrollRuleState) wageTypeInUse(id int64) bool {
	for _, rule := range s.rules {
		if rule.WageTypeID == id && !rule.Archived {
			return true
		}
	}
	for _, absenceType := range s.absenceTypes {
		if absenceType.WageTypeID != nil && *absenceType.WageTypeID == id {
			return true
		}
	}
	return false
}

// unitChangeAllowed keeps every active rule and absence type bookable: a
// percent rule needs hours, an amount rule an amount, an absence hours or
// days.
func (s *payrollRuleState) unitChangeAllowed(id int64, unit string) error {
	for _, rule := range s.rules {
		if rule.WageTypeID == id && !rule.Archived && requiredWageTypeUnit(rule.RateType) != unit {
			return invalidPayrollRule("a surcharge rule books onto this wage type in its present unit")
		}
	}
	for _, absenceType := range s.absenceTypes {
		if absenceType.WageTypeID != nil && *absenceType.WageTypeID == id && unit == workforce.WageTypeUnitAmount {
			return invalidPayrollRule("an absence type books onto this wage type in hours or days")
		}
	}
	return nil
}

// requiredWageTypeUnit is the unit a rule's line carries: percent rules book
// hours with the wage type, amount rules the computed amount.
func requiredWageTypeUnit(rateType string) string {
	if rateType == workforce.SurchargeRateAmount {
		return workforce.WageTypeUnitAmount
	}
	return workforce.WageTypeUnitHours
}

const (
	maxSurchargeRateHundredths = 100000
	minutesPerDay              = 24 * 60
)

// normalizeRule checks a rule input against the school's wage types.
func (s *payrollRuleState) normalizeRule(input workforce.PayrollSurchargeRuleInput) (workforce.PayrollSurchargeRule, error) {
	rule := workforce.PayrollSurchargeRule{
		Kind: input.Kind, RateType: input.RateType, RateHundredths: input.RateHundredths, WageTypeID: input.WageTypeID,
	}
	if err := normalizeRuleWindow(&rule, input); err != nil {
		return rule, err
	}
	switch {
	case rule.RateType != workforce.SurchargeRatePercent && rule.RateType != workforce.SurchargeRateAmount:
		return rule, invalidPayrollRule("the rate type is prozent or betrag")
	case rule.RateHundredths < 1 || rule.RateHundredths > maxSurchargeRateHundredths:
		return rule, invalidPayrollRule("the rate lies between 0.01 and 1000")
	}
	wageType, ok := s.activeWageType(rule.WageTypeID)
	if !ok {
		return rule, payrollRuleNotFound("wage type")
	}
	if wageType.Unit != requiredWageTypeUnit(rule.RateType) {
		return rule, invalidPayrollRule("a percent rule books onto an hour wage type, an amount rule onto an amount wage type")
	}
	return rule, nil
}

func normalizeRuleWindow(rule *workforce.PayrollSurchargeRule, input workforce.PayrollSurchargeRuleInput) error {
	switch rule.Kind {
	case workforce.SurchargeKindNight:
		rule.WindowStartMinute, rule.WindowEndMinute = workforce.DefaultNightWindowStartMinute, workforce.DefaultNightWindowEndMinute
		if input.WindowStartMinute != nil {
			rule.WindowStartMinute = *input.WindowStartMinute
		}
		if input.WindowEndMinute != nil {
			rule.WindowEndMinute = *input.WindowEndMinute
		}
		if !validWindowMinute(rule.WindowStartMinute) || !validWindowMinute(rule.WindowEndMinute) || rule.WindowStartMinute == rule.WindowEndMinute {
			return invalidPayrollRule("the night window has a start and a different end between 00:00 and 23:59")
		}
		return nil
	case workforce.SurchargeKindSunday, workforce.SurchargeKindHoliday:
		if input.WindowStartMinute != nil || input.WindowEndMinute != nil {
			return invalidPayrollRule("only a night rule has a window")
		}
		return nil
	default:
		return invalidPayrollRule("the kind is nacht, sonntag or feiertag")
	}
}

func validWindowMinute(minute int) bool { return minute >= 0 && minute < minutesPerDay }
