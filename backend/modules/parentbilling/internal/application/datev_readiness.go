package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// datevExport checks the DATEV settings of a Träger (#4694) by formatting
// a probe batch: the current fiscal year as period and one booking on the
// revenue account. Only the problems of settings are reported.
type datevExport struct {
	settings parentbilling.DatevBillingSettings
	today    func() calendar.Date
	now      func() time.Time
}

// NewDatevExport builds the DATEV booking batch over the settings port the
// root binds.
func NewDatevExport(settings parentbilling.DatevBillingSettings, today func() calendar.Date, now func() time.Time) parentbilling.DatevExport {
	return newDatevExport(settings, today, now)
}

func newDatevExport(settings parentbilling.DatevBillingSettings, today func() calendar.Date, now func() time.Time) *datevExport {
	if settings == nil || today == nil || now == nil {
		panic("parent billing: the DATEV export needs its settings and clocks")
	}
	return &datevExport{settings: settings, today: today, now: now}
}

// datevProbeDocument is the invoice number of the probe booking.
const datevProbeDocument = "PRUEFUNG"

func (d *datevExport) DatevReadiness(ctx context.Context, reviewer parentbilling.HolidayReviewer) (parentbilling.DatevReadiness, error) {
	settings, err := d.settings.DatevSettings(ctx, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.DatevReadiness{}, fmt.Errorf("read DATEV settings: %w", err)
	}
	readiness := datevReadiness{}
	header := DatevBatchHeader{
		ConsultantNumber: readiness.number(settings.ConsultantNumber),
		ClientNumber:     readiness.number(settings.ClientNumber),
		AccountLength:    readiness.number(settings.AccountLength),
		ChartOfAccounts:  settings.ChartOfAccounts.Value,
		Label:            "Pruefung",
		FileNumber:       1,
		CreatedAt:        d.now(),
		Locked:           settings.Locked,
	}
	if month := readiness.number(settings.FiscalYearStart); month >= 1 && month <= 12 {
		header.FiscalYearStart, header.To = datevFiscalYear(d.today(), time.Month(month))
		header.From = header.FiscalYearStart
	} else if month != 0 {
		readiness.invalid = append(readiness.invalid, settings.FiscalYearStart.Key)
	}
	probe := DatevBooking{AmountCents: 1, Side: DatevDebit, Account: datevProbeDebtor(header.AccountLength),
		CounterAccount: settings.RevenueAccount.Value, DocumentDate: header.From, DocumentNumber: datevProbeDocument}
	_, err = FormatDatevBookingBatch(header, []DatevBooking{probe})
	var formatErr *DatevFormatError
	if err != nil && !errors.As(err, &formatErr) {
		return parentbilling.DatevReadiness{}, err
	}
	if formatErr != nil {
		readiness.problems(formatErr.Problems, settings, header.AccountLength >= datevMinAccountLength && header.AccountLength <= datevMaxAccountLength)
	}
	return readiness.result(), nil
}

// datevFiscalYear is the fiscal year starting in month that holds today:
// its first and its last day.
func datevFiscalYear(today calendar.Date, month time.Month) (calendar.Date, calendar.Date) {
	start := calendar.NewDate(today.Year(), month, 1)
	if today.Before(start) {
		start = calendar.NewDate(today.Year()-1, month, 1)
	}
	return start, calendar.NewDate(start.Year()+1, month, 1).AddDays(-1)
}

// datevProbeDebtor is the first debtor for the account length; with an
// invalid length the header problem names the length.
func datevProbeDebtor(length int) string {
	if length < datevMinAccountLength || length > datevMaxAccountLength {
		return "10000"
	}
	return "1" + strings.Repeat("0", length)
}

type datevReadiness struct {
	missing, invalid []string
}

// number reads a numeric setting; empty is 0 (unset), text that is no
// number is marked invalid.
func (r *datevReadiness) number(setting parentbilling.DatevSetting) int {
	if setting.Value == "" {
		return 0
	}
	value, err := strconv.Atoi(setting.Value)
	if err != nil || value <= 0 {
		r.invalid = append(r.invalid, setting.Key)
		return -1
	}
	return value
}

// problems maps the problems of the probe batch to the settings behind
// them. A problem of the probe itself (debtor, date, invoice number) only
// follows from a setting already named; so does a revenue account that
// cannot be checked while the account length is unset or invalid.
func (r *datevReadiness) problems(problems []DatevProblem, settings parentbilling.DatevSettings, lengthValid bool) {
	fields := map[string]parentbilling.DatevSetting{
		DatevFieldConsultantNumber: settings.ConsultantNumber,
		DatevFieldClientNumber:     settings.ClientNumber,
		DatevFieldAccountLength:    settings.AccountLength,
		DatevFieldChartOfAccounts:  settings.ChartOfAccounts,
		DatevFieldFiscalYearStart:  settings.FiscalYearStart,
		DatevFieldCounterAccount:   settings.RevenueAccount,
	}
	for _, problem := range problems {
		setting, ok := fields[problem.Field]
		switch {
		case !ok, problem.Field == DatevFieldCounterAccount && setting.Value != "" && !lengthValid:
			continue
		case setting.Value == "":
			r.missing = append(r.missing, setting.Key)
		default:
			r.invalid = append(r.invalid, setting.Key)
		}
	}
}

func (r *datevReadiness) result() parentbilling.DatevReadiness {
	missing, invalid := datevKeys(r.missing), datevKeys(r.invalid)
	invalid = slices.DeleteFunc(invalid, func(key string) bool { return slices.Contains(missing, key) })
	return parentbilling.DatevReadiness{
		Ready: len(missing) == 0 && len(invalid) == 0, MissingSettings: missing, InvalidSettings: invalid,
	}
}

func datevKeys(keys []string) []string {
	out := slices.Clone(keys)
	slices.Sort(out)
	out = slices.Compact(out)
	if out == nil {
		return []string{}
	}
	return out
}
