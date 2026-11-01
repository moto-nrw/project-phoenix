package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// PayrollRulesStore persists active.payroll_wage_types,
// active.payroll_surcharge_rules and the wage type of
// active.staff_absence_types (#4807) in the tenant transaction of the
// context. LockPayrollRules serializes rule changes and complete rule-set
// reads. The lists include archived entries; Update* reports false for an
// unknown or archived entry.
type PayrollRulesStore interface {
	LockPayrollRules(context.Context) error
	ReadPayrollRuleState(context.Context) (workforce.PayrollRulesState, error)
	InsertPayrollWageType(context.Context, workforce.PayrollWageType) (workforce.PayrollWageType, error)
	UpdatePayrollWageType(context.Context, workforce.PayrollWageType) (bool, error)
	InsertPayrollSurchargeRule(context.Context, workforce.PayrollSurchargeRule) (workforce.PayrollSurchargeRule, error)
	UpdatePayrollSurchargeRule(context.Context, workforce.PayrollSurchargeRule) (bool, error)
	SetAbsenceTypeWageType(ctx context.Context, absenceTypeID int64, wageTypeID *int64) (bool, error)
}
