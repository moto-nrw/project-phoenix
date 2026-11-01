package timetracking

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// exportPayroll adds the school's own wage types to the month export rows
// (#4807): the rule set each row books with, the credit of the school's
// absence types and the surcharge minutes. It reads the current rules and the
// statutory holidays of the export period once.
type exportPayroll struct {
	current  *workforce.PayrollRuleSet
	holidays map[calendar.Date]bool
	now      time.Time
}

func (s *staffOverviewService) loadExportPayroll(ctx context.Context, first, last monthKey) (*exportPayroll, error) {
	if s.payrollRules == nil {
		return nil, nil
	}
	current, err := s.payrollRules.CurrentPayrollRuleSet(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load payroll rules: %w", err)
	}
	payroll := &exportPayroll{current: current, holidays: map[calendar.Date]bool{}, now: calendar.Now()}
	if s.statutoryHolidays != nil {
		if payroll.holidays, err = s.statutoryHolidays.HolidayDates(ctx, first.firstDay(), last.lastDay()); err != nil {
			return nil, fmt.Errorf("failed to load statutory holidays: %w", err)
		}
	}
	return payroll, nil
}

// apply fills the wage type fields of one row. A closed month books with the
// rules frozen at its close; a close from before #4807 froze none, so the
// month had no own wage types and books none now either.
func (p *exportPayroll) apply(row *MonthExportRow, summary *MonthSummary, prefetch *monthPrefetch, key monthKey) error {
	if p == nil {
		return nil
	}
	rules := p.current
	if summary.IsClosed {
		frozen, err := frozenPayrollRules(prefetch.snapshots[row.StaffID], key)
		if err != nil {
			return fmt.Errorf("staff %d %04d-%02d: %w", row.StaffID, key.Year, key.Month, err)
		}
		rules = frozen
	}
	if rules == nil || len(rules.WageTypes) == 0 {
		return nil
	}
	row.PayrollRules = rules
	row.AbsenceTypeCredits = summary.CreditedByAbsenceType
	if len(rules.SurchargeRules) > 0 {
		row.SurchargeMinutes = SurchargeMinutes(prefetch.sessions[row.StaffID], prefetch.breaks, p.holidays,
			rules.SurchargeRules, key.firstDay(), key.lastDay(), p.now)
	}
	return nil
}

// frozenPayrollRules decodes the rule set of the active snapshot of the
// month, nil when the close froze none.
func frozenPayrollRules(snapshots []*MonthSnapshot, key monthKey) (*workforce.PayrollRuleSet, error) {
	for _, snapshot := range snapshots {
		if !snapshot.IsActive() || snapshot.Year != key.Year || snapshot.Month != key.Month {
			continue
		}
		if len(snapshot.PayrollRules) == 0 {
			return nil, nil
		}
		var rules workforce.PayrollRuleSet
		if err := json.Unmarshal(snapshot.PayrollRules, &rules); err != nil {
			return nil, fmt.Errorf("decode frozen payroll rules: %w", err)
		}
		return &rules, nil
	}
	return nil, nil
}
