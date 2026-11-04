package workforce

import (
	"context"
	"errors"
)

// Eigene Lohnarten und Zuschläge (#4807). A school adds its own wage types
// next to the seven fixed categories of the DATEV export, maps its own
// absence types onto them, and defines surcharge rules for night, Sunday and
// holiday work. They are school objects of Workforce, not settings; reading
// and writing needs config:manage like the fixed Lohnart settings.

// Units of a wage type: what its DATEV line carries.
const (
	WageTypeUnitHours  = "stunden"
	WageTypeUnitDays   = "tage"
	WageTypeUnitAmount = "betrag"
)

// Kinds of a surcharge rule.
const (
	SurchargeKindNight   = "nacht"
	SurchargeKindSunday  = "sonntag"
	SurchargeKindHoliday = "feiertag"
)

// How a surcharge rule states its height: a percentage of the hourly wage,
// booked as hours with the wage type, or a fixed amount per hour, booked as
// the computed amount.
const (
	SurchargeRatePercent = "prozent"
	SurchargeRateAmount  = "betrag"
)

// The night window a new night rule starts with (20:00 to 06:00), in minutes
// after midnight on the school's wall clock.
const (
	DefaultNightWindowStartMinute = 20 * 60
	DefaultNightWindowEndMinute   = 6 * 60
)

var (
	// ErrPayrollRuleInvalid rejects a wage type, rule or mapping the export
	// could not book correctly.
	ErrPayrollRuleInvalid = errors.New("payroll rule invalid")
	// ErrPayrollRuleNotFound names a wage type, rule or absence type the
	// school does not have.
	ErrPayrollRuleNotFound = errors.New("payroll rule not found")
	// ErrPayrollWageTypeNameTaken rejects a second active wage type with the
	// same name.
	ErrPayrollWageTypeNameTaken = errors.New("payroll wage type name taken")
	// ErrPayrollWageTypeInUse refuses to archive a wage type an active rule
	// or an absence type still books onto.
	ErrPayrollWageTypeInUse = errors.New("payroll wage type in use")
)

// PayrollWageType is one wage type of the school. Number is the DATEV
// Lohnartnummer (1 to 4 digits); empty means the line is missing from the
// DATEV file and the preflight names it.
type PayrollWageType struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Number   string `json:"number"`
	Unit     string `json:"unit"`
	Archived bool   `json:"archived"`
}

// PayrollSurchargeRule is one surcharge rule. The window applies to night
// rules only. RateHundredths is the percentage times 100 (25 % = 2500) for
// percent rules and the amount per hour in cent for amount rules.
type PayrollSurchargeRule struct {
	ID                int64  `json:"id"`
	Kind              string `json:"kind"`
	WindowStartMinute int    `json:"window_start_minute"`
	WindowEndMinute   int    `json:"window_end_minute"`
	RateType          string `json:"rate_type"`
	RateHundredths    int    `json:"rate_hundredths"`
	WageTypeID        int64  `json:"wage_type_id"`
	Archived          bool   `json:"archived"`
}

// PayrollRuleSet is the payroll configuration one month is booked with: the
// current one for an open month, the one frozen at the close for a closed
// month. Archived entries are left out.
type PayrollRuleSet struct {
	WageTypes      []PayrollWageType      `json:"wage_types"`
	SurchargeRules []PayrollSurchargeRule `json:"surcharge_rules"`
	// AbsenceTypeWageTypes maps a school absence type to its wage type.
	AbsenceTypeWageTypes map[int64]int64 `json:"absence_type_wage_types"`
}

// WageType returns the active wage type with the given ID.
func (s *PayrollRuleSet) WageType(id int64) (PayrollWageType, bool) {
	if s == nil {
		return PayrollWageType{}, false
	}
	for _, wageType := range s.WageTypes {
		if wageType.ID == id {
			return wageType, true
		}
	}
	return PayrollWageType{}, false
}

// PayrollAbsenceTypeWageType is one school absence type with its wage type.
type PayrollAbsenceTypeWageType struct {
	AbsenceTypeID int64  `json:"absence_type_id"`
	Name          string `json:"name"`
	IsActive      bool   `json:"is_active"`
	WageTypeID    *int64 `json:"wage_type_id"`
}

// PayrollRulesState is the complete rule state a maintenance operation reads
// under the payroll-rule lock.
type PayrollRulesState struct {
	WageTypes    []PayrollWageType
	Rules        []PayrollSurchargeRule
	AbsenceTypes []PayrollAbsenceTypeWageType
}

// PayrollRulesOverview is what /payroll shows: the active wage types and
// rules and every school absence type with its mapping.
type PayrollRulesOverview struct {
	WageTypes      []PayrollWageType            `json:"wage_types"`
	SurchargeRules []PayrollSurchargeRule       `json:"surcharge_rules"`
	AbsenceTypes   []PayrollAbsenceTypeWageType `json:"absence_types"`
}

// PayrollWageTypeInput creates or changes a wage type.
type PayrollWageTypeInput struct {
	Name   string `json:"name"`
	Number string `json:"number"`
	Unit   string `json:"unit"`
}

// PayrollSurchargeRuleInput creates or changes a surcharge rule. A night rule
// without a window gets the default 20:00 to 06:00.
type PayrollSurchargeRuleInput struct {
	Kind              string `json:"kind"`
	WindowStartMinute *int   `json:"window_start_minute"`
	WindowEndMinute   *int   `json:"window_end_minute"`
	RateType          string `json:"rate_type"`
	RateHundredths    int    `json:"rate_hundredths"`
	WageTypeID        int64  `json:"wage_type_id"`
}

// PayrollRules maintains the wage types, surcharge rules and absence type
// mapping of the school. Every change is written to the settings audit with
// the acting account, in the caller's tenant transaction.
type PayrollRules interface {
	Overview(ctx context.Context) (*PayrollRulesOverview, error)
	CreateWageType(ctx context.Context, actorAccountID int64, input PayrollWageTypeInput) (*PayrollWageType, error)
	UpdateWageType(ctx context.Context, actorAccountID, id int64, input PayrollWageTypeInput) (*PayrollWageType, error)
	ArchiveWageType(ctx context.Context, actorAccountID, id int64) error
	CreateSurchargeRule(ctx context.Context, actorAccountID int64, input PayrollSurchargeRuleInput) (*PayrollSurchargeRule, error)
	UpdateSurchargeRule(ctx context.Context, actorAccountID, id int64, input PayrollSurchargeRuleInput) (*PayrollSurchargeRule, error)
	ArchiveSurchargeRule(ctx context.Context, actorAccountID, id int64) error
	SetAbsenceTypeWageType(ctx context.Context, actorAccountID, absenceTypeID int64, wageTypeID *int64) error
}

// PayrollRuleChange is one change for the settings audit: Key names the
// object ("payroll.wage_type.12"), Action is "set" or "delete", Old and New
// are the JSON of the object before and after.
type PayrollRuleChange struct {
	Key            string
	Action         string
	Old, New       []byte
	ActorAccountID int64
}

// PayrollRuleAudit writes a change into the settings audit of the tenant in
// the context, in its transaction. The root binds it to the Settings
// Platform.
type PayrollRuleAudit interface {
	RecordPayrollRuleChange(context.Context, PayrollRuleChange) error
}

// PayrollRuleSetReader answers a rule set in force now, serialized against
// rule changes; the export books open months with it and the close freezes it.
type PayrollRuleSetReader interface {
	CurrentPayrollRuleSet(context.Context) (*PayrollRuleSet, error)
}
