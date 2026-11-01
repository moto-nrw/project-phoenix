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

// mappedCategoryCredit is the part of a standard category that an absence
// type booked onto its own wage type takes away: without a mapping the
// credit stays in its category, with one it books only once.
func mappedCategoryCredit(row MonthExportRow, baseType string) (int, float64) {
	rules := row.PayrollRules
	if rules == nil {
		return 0, 0
	}
	minutes, days := 0, 0.0
	for absenceTypeID, credit := range row.AbsenceTypeCredits {
		wageTypeID, mapped := rules.AbsenceTypeWageTypes[absenceTypeID]
		if _, active := rules.WageType(wageTypeID); mapped && active && credit.BaseType == baseType {
			minutes += credit.Minutes
			days += credit.Days
		}
	}
	return minutes, days
}

// exportWageTypes is every wage type a row of the export books with, in
// wage type order; a renamed wage type keeps the name of its first row.
func exportWageTypes(rows []MonthExportRow) []workforce.PayrollWageType {
	seen := map[int64]bool{}
	var result []workforce.PayrollWageType
	for _, row := range rows {
		if row.PayrollRules == nil {
			continue
		}
		for _, wageType := range row.PayrollRules.WageTypes {
			if !seen[wageType.ID] {
				seen[wageType.ID] = true
				result = append(result, wageType)
			}
		}
	}
	slices.SortFunc(result, func(a, b workforce.PayrollWageType) int { return compareInt64(a.ID, b.ID) })
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
	names := []string{}
	for _, wageType := range exportWageTypes(rows) {
		if wageType.Number == "" {
			names = append(names, wageType.Name)
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
