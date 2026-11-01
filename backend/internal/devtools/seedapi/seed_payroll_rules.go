package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
)

// seedPayrollRulesStep gives the school its own wage types and surcharge
// rules (#4807): a night, a Sunday and a holiday surcharge, and the
// Regenerationstag booked onto a day wage type. The numbers are invented like
// the fixed Lohnart numbers of the demo. Every write goes through the
// school's API and leaves its row in the settings audit. A school that
// already has wage types (a second run) keeps them.
type seedPayrollRulesStep struct{}

func (seedPayrollRulesStep) Name() string { return "Seeding payroll wage types and surcharge rules" }

const payrollRulesPath = "/api/staff/time-tracking/payroll-rules"

type seedPayrollRules struct {
	WageTypes []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"wage_types"`
	AbsenceTypes []struct {
		ID         string  `json:"id"`
		Name       string  `json:"name"`
		WageTypeID *string `json:"wage_type_id"`
	} `json:"absence_types"`
}

var demoWageTypes = []struct{ name, number, unit string }{
	{"Nachtzuschlag", "311", "stunden"},
	{"Sonntagszuschlag", "312", "betrag"},
	{"Feiertagszuschlag", "313", "stunden"},
	{"Regenerationstag", "314", "tage"},
}

var demoSurchargeRules = []struct {
	kind, rateType, wageType string
	rate                     int
}{
	{"nacht", "prozent", "Nachtzuschlag", 2500},
	{"sonntag", "betrag", "Sonntagszuschlag", 250},
	{"feiertag", "prozent", "Feiertagszuschlag", 12500},
}

func (seedPayrollRulesStep) Run(_ context.Context, rt *Runtime) error {
	rules, err := readSeedPayrollRules(rt)
	if err != nil {
		return err
	}
	if len(rules.WageTypes) > 0 {
		fmt.Printf("  %d wage types already there\n", len(rules.WageTypes))
		return nil
	}
	ids := make(map[string]string, len(demoWageTypes))
	for _, wageType := range demoWageTypes {
		raw, err := rt.Client.PostWithAuth(rt.TenantAuth, payrollRulesPath+"/wage-types", map[string]any{
			"name": wageType.name, "number": wageType.number, "unit": wageType.unit,
		})
		if err != nil {
			return fmt.Errorf("create wage type %s: %w", wageType.name, err)
		}
		id, err := parseEnvelopeStringID(raw)
		if err != nil {
			return fmt.Errorf("parse wage type %s: %w", wageType.name, err)
		}
		ids[wageType.name] = strconv.FormatInt(id, 10)
	}
	for _, rule := range demoSurchargeRules {
		if _, err := rt.Client.PostWithAuth(rt.TenantAuth, payrollRulesPath+"/surcharge-rules", map[string]any{
			"kind": rule.kind, "rate_type": rule.rateType, "rate_hundredths": rule.rate, "wage_type_id": ids[rule.wageType],
		}); err != nil {
			return fmt.Errorf("create %s surcharge rule: %w", rule.kind, err)
		}
	}
	return nil
}

// seedPayrollAbsenceTypeMappingStep books the Regenerationstag that time
// tracking created onto its own day wage type.
type seedPayrollAbsenceTypeMappingStep struct{}

func (seedPayrollAbsenceTypeMappingStep) Name() string { return "Mapping payroll absence type" }

func (seedPayrollAbsenceTypeMappingStep) Run(_ context.Context, rt *Runtime) error {
	rules, err := readSeedPayrollRules(rt)
	if err != nil {
		return err
	}
	for _, absenceType := range rules.AbsenceTypes {
		if absenceType.Name != "Regenerationstag" {
			continue
		}
		var wageTypeID string
		for _, wageType := range rules.WageTypes {
			if wageType.Name == "Regenerationstag" {
				wageTypeID = wageType.ID
				break
			}
		}
		if wageTypeID == "" {
			return nil
		}
		if _, err := rt.Client.PutWithAuth(rt.TenantAuth, payrollRulesPath+"/absence-types/"+absenceType.ID, map[string]any{
			"wage_type_id": wageTypeID,
		}); err != nil {
			return fmt.Errorf("book absence type Regenerationstag onto its wage type: %w", err)
		}
		fmt.Printf("  Regenerationstag booked onto its wage type\n")
		return nil
	}
	return fmt.Errorf("absence type Regenerationstag for the payroll rules is missing")
}

func readSeedPayrollRules(rt *Runtime) (seedPayrollRules, error) {
	raw, err := rt.Client.GetWithAuth(rt.TenantAuth, payrollRulesPath)
	if err != nil {
		return seedPayrollRules{}, fmt.Errorf("read payroll rules: %w", err)
	}
	var envelope struct {
		Data seedPayrollRules `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return seedPayrollRules{}, fmt.Errorf("decode payroll rules: %w", err)
	}
	return envelope.Data, nil
}
