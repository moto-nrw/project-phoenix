package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// PayrollRules maintains the school's own wage types, surcharge rules and
// absence type mapping (#4807). Every write runs in the caller's tenant
// transaction, holds the school's rule lock, and records the object before
// and after in the settings audit; without the audit row nothing changes.
type PayrollRules struct {
	store ports.PayrollRulesStore
	audit workforce.PayrollRuleAudit
}

var _ workforce.PayrollRules = (*PayrollRules)(nil)

// NewPayrollRules requires the store and the audit.
func NewPayrollRules(store ports.PayrollRulesStore, audit workforce.PayrollRuleAudit) (*PayrollRules, error) {
	if store == nil || audit == nil {
		return nil, errors.New("payroll rules: store and audit are required")
	}
	return &PayrollRules{store: store, audit: audit}, nil
}

const maxWageTypeNameLength = 100

var wageTypeNumberPattern = regexp.MustCompile(`^[0-9]{1,4}$`)

func invalidPayrollRule(reason string) error {
	return fmt.Errorf("%w: %s", workforce.ErrPayrollRuleInvalid, reason)
}

func payrollRuleNotFound(what string) error {
	return fmt.Errorf("%w: %s", workforce.ErrPayrollRuleNotFound, what)
}

// Overview lists the active wage types and rules and every school absence
// type with its mapping.
func (p *PayrollRules) Overview(ctx context.Context) (*workforce.PayrollRulesOverview, error) {
	wageTypes, rules, err := p.activeRows(ctx)
	if err != nil {
		return nil, err
	}
	absenceTypes, err := p.store.ListPayrollAbsenceTypes(ctx)
	if err != nil {
		return nil, err
	}
	return &workforce.PayrollRulesOverview{WageTypes: wageTypes, SurchargeRules: rules, AbsenceTypes: absenceTypes}, nil
}

// CurrentPayrollRuleSet is the rule set an open month books with. Absence
// types mapped onto an archived wage type keep their category.
func (p *PayrollRules) CurrentPayrollRuleSet(ctx context.Context) (*workforce.PayrollRuleSet, error) {
	wageTypes, rules, err := p.activeRows(ctx)
	if err != nil {
		return nil, err
	}
	absenceTypes, err := p.store.ListPayrollAbsenceTypes(ctx)
	if err != nil {
		return nil, err
	}
	set := &workforce.PayrollRuleSet{WageTypes: wageTypes, SurchargeRules: rules, AbsenceTypeWageTypes: map[int64]int64{}}
	for _, absenceType := range absenceTypes {
		if absenceType.WageTypeID == nil {
			continue
		}
		if _, ok := set.WageType(*absenceType.WageTypeID); ok {
			set.AbsenceTypeWageTypes[absenceType.AbsenceTypeID] = *absenceType.WageTypeID
		}
	}
	return set, nil
}

func (p *PayrollRules) activeRows(ctx context.Context) ([]workforce.PayrollWageType, []workforce.PayrollSurchargeRule, error) {
	allWageTypes, err := p.store.ListPayrollWageTypes(ctx)
	if err != nil {
		return nil, nil, err
	}
	allRules, err := p.store.ListPayrollSurchargeRules(ctx)
	if err != nil {
		return nil, nil, err
	}
	wageTypes := make([]workforce.PayrollWageType, 0, len(allWageTypes))
	for _, wageType := range allWageTypes {
		if !wageType.Archived {
			wageTypes = append(wageTypes, wageType)
		}
	}
	rules := make([]workforce.PayrollSurchargeRule, 0, len(allRules))
	for _, rule := range allRules {
		if !rule.Archived {
			rules = append(rules, rule)
		}
	}
	return wageTypes, rules, nil
}

// --- wage types -------------------------------------------------------------

func normalizeWageType(input workforce.PayrollWageTypeInput) (workforce.PayrollWageType, error) {
	value := workforce.PayrollWageType{
		Name: strings.TrimSpace(input.Name), Number: strings.TrimSpace(input.Number), Unit: input.Unit,
	}
	switch {
	case value.Name == "":
		return value, invalidPayrollRule("the wage type needs a name")
	case len([]rune(value.Name)) > maxWageTypeNameLength:
		return value, invalidPayrollRule("the wage type name has more than 100 characters")
	case value.Number != "" && !wageTypeNumberPattern.MatchString(value.Number):
		return value, invalidPayrollRule("the DATEV number has 1 to 4 digits")
	}
	switch value.Unit {
	case workforce.WageTypeUnitHours, workforce.WageTypeUnitDays, workforce.WageTypeUnitAmount:
		return value, nil
	default:
		return value, invalidPayrollRule("the unit is stunden, tage or betrag")
	}
}

func (p *PayrollRules) CreateWageType(ctx context.Context, actorAccountID int64, input workforce.PayrollWageTypeInput) (*workforce.PayrollWageType, error) {
	value, err := normalizeWageType(input)
	if err != nil {
		return nil, err
	}
	if err := p.store.LockPayrollRules(ctx); err != nil {
		return nil, err
	}
	created, err := p.store.InsertPayrollWageType(ctx, value)
	if err != nil {
		return nil, err
	}
	if err := p.record(ctx, actorAccountID, wageTypeKey(created.ID), "set", nil, created); err != nil {
		return nil, err
	}
	return &created, nil
}

func (p *PayrollRules) UpdateWageType(ctx context.Context, actorAccountID, id int64, input workforce.PayrollWageTypeInput) (*workforce.PayrollWageType, error) {
	value, err := normalizeWageType(input)
	if err != nil {
		return nil, err
	}
	value.ID = id
	if err := p.store.LockPayrollRules(ctx); err != nil {
		return nil, err
	}
	state, err := p.loadState(ctx)
	if err != nil {
		return nil, err
	}
	previous, ok := state.activeWageType(id)
	if !ok {
		return nil, payrollRuleNotFound("wage type")
	}
	if value.Unit != previous.Unit {
		if err := state.unitChangeAllowed(id, value.Unit); err != nil {
			return nil, err
		}
	}
	if err := p.writeWageType(ctx, value); err != nil {
		return nil, err
	}
	if err := p.record(ctx, actorAccountID, wageTypeKey(id), "set", previous, value); err != nil {
		return nil, err
	}
	return &value, nil
}

// ArchiveWageType retires a wage type no active rule or absence type books
// onto any more.
func (p *PayrollRules) ArchiveWageType(ctx context.Context, actorAccountID, id int64) error {
	if err := p.store.LockPayrollRules(ctx); err != nil {
		return err
	}
	state, err := p.loadState(ctx)
	if err != nil {
		return err
	}
	previous, ok := state.activeWageType(id)
	if !ok {
		return payrollRuleNotFound("wage type")
	}
	if state.wageTypeInUse(id) {
		return fmt.Errorf("%w: a rule or an absence type books onto it", workforce.ErrPayrollWageTypeInUse)
	}
	archived := previous
	archived.Archived = true
	if err := p.writeWageType(ctx, archived); err != nil {
		return err
	}
	return p.record(ctx, actorAccountID, wageTypeKey(id), "delete", previous, archived)
}

func (p *PayrollRules) writeWageType(ctx context.Context, value workforce.PayrollWageType) error {
	found, err := p.store.UpdatePayrollWageType(ctx, value)
	if err != nil {
		return err
	}
	if !found {
		return payrollRuleNotFound("wage type")
	}
	return nil
}

// --- surcharge rules --------------------------------------------------------

func (p *PayrollRules) CreateSurchargeRule(ctx context.Context, actorAccountID int64, input workforce.PayrollSurchargeRuleInput) (*workforce.PayrollSurchargeRule, error) {
	if err := p.store.LockPayrollRules(ctx); err != nil {
		return nil, err
	}
	state, err := p.loadState(ctx)
	if err != nil {
		return nil, err
	}
	value, err := state.normalizeRule(input)
	if err != nil {
		return nil, err
	}
	created, err := p.store.InsertPayrollSurchargeRule(ctx, value)
	if err != nil {
		return nil, err
	}
	if err := p.record(ctx, actorAccountID, surchargeRuleKey(created.ID), "set", nil, created); err != nil {
		return nil, err
	}
	return &created, nil
}

func (p *PayrollRules) UpdateSurchargeRule(ctx context.Context, actorAccountID, id int64, input workforce.PayrollSurchargeRuleInput) (*workforce.PayrollSurchargeRule, error) {
	if err := p.store.LockPayrollRules(ctx); err != nil {
		return nil, err
	}
	state, err := p.loadState(ctx)
	if err != nil {
		return nil, err
	}
	previous, ok := state.activeRule(id)
	if !ok {
		return nil, payrollRuleNotFound("surcharge rule")
	}
	value, err := state.normalizeRule(input)
	if err != nil {
		return nil, err
	}
	value.ID = id
	if err := p.writeRule(ctx, value); err != nil {
		return nil, err
	}
	if err := p.record(ctx, actorAccountID, surchargeRuleKey(id), "set", previous, value); err != nil {
		return nil, err
	}
	return &value, nil
}

func (p *PayrollRules) ArchiveSurchargeRule(ctx context.Context, actorAccountID, id int64) error {
	if err := p.store.LockPayrollRules(ctx); err != nil {
		return err
	}
	state, err := p.loadState(ctx)
	if err != nil {
		return err
	}
	previous, ok := state.activeRule(id)
	if !ok {
		return payrollRuleNotFound("surcharge rule")
	}
	archived := previous
	archived.Archived = true
	if err := p.writeRule(ctx, archived); err != nil {
		return err
	}
	return p.record(ctx, actorAccountID, surchargeRuleKey(id), "delete", previous, archived)
}

func (p *PayrollRules) writeRule(ctx context.Context, value workforce.PayrollSurchargeRule) error {
	found, err := p.store.UpdatePayrollSurchargeRule(ctx, value)
	if err != nil {
		return err
	}
	if !found {
		return payrollRuleNotFound("surcharge rule")
	}
	return nil
}

// --- absence types ----------------------------------------------------------

// SetAbsenceTypeWageType books a school absence type onto a wage type that
// counts hours or days; nil books it with its category again.
func (p *PayrollRules) SetAbsenceTypeWageType(ctx context.Context, actorAccountID, absenceTypeID int64, wageTypeID *int64) error {
	if err := p.store.LockPayrollRules(ctx); err != nil {
		return err
	}
	state, err := p.loadState(ctx)
	if err != nil {
		return err
	}
	absenceType, ok := state.absenceType(absenceTypeID)
	if !ok {
		return payrollRuleNotFound("absence type")
	}
	if wageTypeID != nil {
		wageType, ok := state.activeWageType(*wageTypeID)
		if !ok {
			return payrollRuleNotFound("wage type")
		}
		if wageType.Unit == workforce.WageTypeUnitAmount {
			return invalidPayrollRule("an absence is booked in hours or days, not as an amount")
		}
	}
	found, err := p.store.SetAbsenceTypeWageType(ctx, absenceTypeID, wageTypeID)
	if err != nil {
		return err
	}
	if !found {
		return payrollRuleNotFound("absence type")
	}
	return p.record(ctx, actorAccountID, absenceTypeKey(absenceTypeID), "set", mappingJSON(absenceType.WageTypeID), mappingJSON(wageTypeID))
}

// --- shared -----------------------------------------------------------------

func wageTypeKey(id int64) string      { return "payroll.wage_type." + strconv.FormatInt(id, 10) }
func surchargeRuleKey(id int64) string { return "payroll.surcharge_rule." + strconv.FormatInt(id, 10) }
func absenceTypeKey(id int64) string {
	return "payroll.absence_type_wage_type." + strconv.FormatInt(id, 10)
}

// record writes one change into the settings audit. A nil before value (a
// created object) leaves the old value empty.
func (p *PayrollRules) record(ctx context.Context, actorAccountID int64, key, action string, before, after any) error {
	change := workforce.PayrollRuleChange{Key: key, Action: action, ActorAccountID: actorAccountID}
	var err error
	if before != nil {
		if change.Old, err = json.Marshal(before); err != nil {
			return fmt.Errorf("payroll rules: encode audit value: %w", err)
		}
	}
	if change.New, err = json.Marshal(after); err != nil {
		return fmt.Errorf("payroll rules: encode audit value: %w", err)
	}
	if err := p.audit.RecordPayrollRuleChange(ctx, change); err != nil {
		return fmt.Errorf("payroll rules: record settings audit: %w", err)
	}
	return nil
}

// mappingJSON is the audit value of an absence type mapping: the wage type
// ID, or null for the category.
func mappingJSON(wageTypeID *int64) json.RawMessage {
	if wageTypeID == nil {
		return json.RawMessage("null")
	}
	return json.RawMessage(strconv.FormatInt(*wageTypeID, 10))
}

// payrollRuleSetReader answers the rule set in force without the write side.
type payrollRuleSetReader struct{ rules *PayrollRules }

// NewPayrollRuleSetReader reads the current rule set; it writes nothing and
// needs no audit.
func NewPayrollRuleSetReader(store ports.PayrollRulesStore) workforce.PayrollRuleSetReader {
	if store == nil {
		panic("payroll rule set reader: store is required")
	}
	return payrollRuleSetReader{rules: &PayrollRules{store: store}}
}

// CurrentPayrollRuleSet joins the caller's tenant transaction, or opens one
// for a caller without (the worker's file build).
func (r payrollRuleSetReader) CurrentPayrollRuleSet(ctx context.Context) (*workforce.PayrollRuleSet, error) {
	var set *workforce.PayrollRuleSet
	err := tenant.WithinCurrentTenant(ctx, func(ctx context.Context) error {
		var err error
		set, err = r.rules.CurrentPayrollRuleSet(ctx)
		return err
	})
	return set, err
}
