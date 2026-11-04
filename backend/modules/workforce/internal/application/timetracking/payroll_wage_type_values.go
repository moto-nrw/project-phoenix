package timetracking

import (
	"fmt"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// Values of the school's own wage types (#4807), per export row. One
// function feeds DATEV, CSV and Excel, so the files cannot disagree:
//
//   - an absence type booked onto a wage type gives its credited minutes
//     (hour wage type) or its absence days (day wage type);
//   - a percent surcharge rule gives its minutes as hours of its wage type;
//   - an amount surcharge rule gives minutes × amount per hour, in cent.
//
// Several sources on one wage type add up.

// wageTypeValue is what one wage type books for one staff member and month.
type wageTypeValue struct {
	WageType workforce.PayrollWageType
	Minutes  int
	Days     float64
	Cents    int
}

func (v wageTypeValue) empty() bool { return v.Minutes == 0 && v.Days == 0 && v.Cents == 0 }

// rowWageTypeValues returns the non-empty wage type values of a row, ordered
// by wage type.
func rowWageTypeValues(row MonthExportRow) []wageTypeValue {
	if row.PayrollRules == nil || len(row.PayrollRules.WageTypes) == 0 {
		return nil
	}
	values := wageTypeValues{rules: row.PayrollRules, byID: map[int64]*wageTypeValue{}}
	for absenceTypeID, credit := range row.AbsenceTypeCredits {
		if wageTypeID, mapped := values.rules.AbsenceTypeWageTypes[absenceTypeID]; mapped {
			values.addAbsence(wageTypeID, credit)
		}
	}
	for _, rule := range values.rules.SurchargeRules {
		if minutes := row.SurchargeMinutes[rule.ID]; minutes > 0 {
			values.addSurcharge(rule, minutes)
		}
	}
	return values.sorted()
}

// wageTypeValues collects the values of one row per active wage type.
type wageTypeValues struct {
	rules *workforce.PayrollRuleSet
	byID  map[int64]*wageTypeValue
}

func (v wageTypeValues) of(id int64) *wageTypeValue {
	wageType, ok := v.rules.WageType(id)
	if !ok {
		return nil
	}
	if v.byID[id] == nil {
		v.byID[id] = &wageTypeValue{WageType: wageType}
	}
	return v.byID[id]
}

func (v wageTypeValues) addAbsence(wageTypeID int64, credit AbsenceTypeCredit) {
	if value := v.of(wageTypeID); value != nil {
		addAbsenceCredit(value, credit)
	}
}

func (v wageTypeValues) addSurcharge(rule PayrollSurchargeRule, minutes int) {
	if value := v.of(rule.WageTypeID); value != nil {
		addSurcharge(value, rule, minutes)
	}
}

func (v wageTypeValues) sorted() []wageTypeValue {
	result := make([]wageTypeValue, 0, len(v.byID))
	for _, value := range v.byID {
		if !value.empty() {
			result = append(result, *value)
		}
	}
	slices.SortFunc(result, func(a, b wageTypeValue) int { return compareInt64(a.WageType.ID, b.WageType.ID) })
	return result
}

func addAbsenceCredit(value *wageTypeValue, credit AbsenceTypeCredit) {
	switch value.WageType.Unit {
	case workforce.WageTypeUnitHours:
		value.Minutes += credit.Minutes
	case workforce.WageTypeUnitDays:
		value.Days += credit.Days
	}
}

func addSurcharge(value *wageTypeValue, rule PayrollSurchargeRule, minutes int) {
	switch {
	case rule.RateType == SurchargeRatePercent && value.WageType.Unit == workforce.WageTypeUnitHours:
		value.Minutes += minutes
	case rule.RateType == SurchargeRateAmount && value.WageType.Unit == workforce.WageTypeUnitAmount:
		value.Cents += SurchargeAmountCents(minutes, rule.RateHundredths)
	}
}

func compareInt64(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

// exportWageTypeColumn identifies a wage-type column. A renamed wage type
// keeps the first name, but a changed unit needs a separate column: a closed
// month can preserve a historical definition while an open one uses the
// current definition of the same wage type.
type exportWageTypeColumn struct {
	WageType workforce.PayrollWageType
}

func (c exportWageTypeColumn) key() wageTypeColumnKey {
	return wageTypeColumnKey{ID: c.WageType.ID, Unit: c.WageType.Unit}
}

type wageTypeColumnKey struct {
	ID   int64
	Unit string
}

func wageTypeColumnKeyFor(wageType workforce.PayrollWageType) wageTypeColumnKey {
	return wageTypeColumnKey{ID: wageType.ID, Unit: wageType.Unit}
}

// exportWageTypeColumns is every wage type definition in the export, ordered
// by wage type. Definitions of one wage type retain their first-row order, so
// a renamed wage type keeps the name of its first row.
func exportWageTypeColumns(rows []MonthExportRow) []exportWageTypeColumn {
	seen := map[wageTypeColumnKey]bool{}
	var result []exportWageTypeColumn
	for _, row := range rows {
		if row.PayrollRules == nil {
			continue
		}
		for _, wageType := range row.PayrollRules.WageTypes {
			key := wageTypeColumnKeyFor(wageType)
			if !seen[key] {
				seen[key] = true
				result = append(result, exportWageTypeColumn{WageType: wageType})
			}
		}
	}
	slices.SortStableFunc(result, func(a, b exportWageTypeColumn) int {
		return compareInt64(a.WageType.ID, b.WageType.ID)
	})
	return result
}

var wageTypeUnitColumnLabels = map[string]string{
	workforce.WageTypeUnitHours:  "Stunden",
	workforce.WageTypeUnitDays:   "Tage",
	workforce.WageTypeUnitAmount: "Euro",
}

// wageTypeColumnHeader names a wage type column: "Nachtzuschlag (Stunden)".
func wageTypeColumnHeader(wageType workforce.PayrollWageType) string {
	return wageType.Name + " (" + wageTypeUnitColumnLabels[wageType.Unit] + ")"
}

// formatExportCents renders cents as Euro with a German comma ("12,50").
func formatExportCents(cents int) string {
	sign := ""
	if cents < 0 {
		sign, cents = "-", -cents
	}
	return fmt.Sprintf("%s%d,%02d", sign, cents/100, cents%100)
}

// appendDatevWageTypeLines appends one line per wage type of the row that
// has a DATEV number and a value; unnumbered ones are named by the report.
func appendDatevWageTypeLines(lines []datevLine, row MonthExportRow) []datevLine {
	for _, value := range rowWageTypeValues(row) {
		if value.WageType.Number == "" {
			continue
		}
		line := datevLine{personnelNumber: row.PersonnelNumber, lohnart: value.WageType.Number}
		switch value.WageType.Unit {
		case workforce.WageTypeUnitAmount:
			cents := value.Cents
			line.amountCents = &cents
		case workforce.WageTypeUnitDays:
			days := value.Days
			line.days = &days
		default:
			minutes := value.Minutes
			line.hoursMinutes = &minutes
		}
		lines = append(lines, line)
	}
	return lines
}

// unnumberedWageTypes names the wage types of the export without a DATEV
// number; their lines are missing from the file.
func unnumberedWageTypes(rows []MonthExportRow) []string {
	seen := map[string]bool{}
	var names []string
	for _, row := range rows {
		if row.PayrollRules == nil {
			continue
		}
		for _, wageType := range row.PayrollRules.WageTypes {
			if wageType.Number == "" && !seen[wageType.Name] {
				seen[wageType.Name] = true
				names = append(names, wageType.Name)
			}
		}
	}
	return names
}

func datevLinesCarryAmounts(lines []datevLine) bool {
	for _, line := range lines {
		if line.amountCents != nil {
			return true
		}
	}
	return false
}
