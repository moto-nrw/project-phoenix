package timetracking_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/settings/settingstest"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/application/timetracking"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Eigene Lohnarten und Zuschläge in the DATEV export (#4807). The month is
// June 2026 like the existing goldens; staff[1] works one night, one Sunday
// and one holiday, staff[0] takes a day of a school absence type booked onto
// its own wage type.

// payrollHolidays declares Thursday 4 June 2026 (Fronleichnam in NRW) the
// statutory holiday of the export month.
type payrollHolidays struct{}

func (payrollHolidays) HolidayDates(_ context.Context, from, to calendar.Date) (map[calendar.Date]bool, error) {
	holiday := calendar.NewDate(2026, time.June, 4)
	if holiday.Before(from) || holiday.After(to) {
		return map[calendar.Date]bool{}, nil
	}
	return map[calendar.Date]bool{holiday: true}, nil
}

func berlinAt(date calendar.Date, hour, minute int) time.Time {
	return time.Date(date.Year(), date.Month(), date.Day(), hour, minute, 0, 0, calendar.Berlin)
}

func (f *overviewFixture) addSessionBetween(t *testing.T, staffID int64, checkIn, checkOut time.Time) *timetracking.WorkSession {
	t.Helper()
	session := &timetracking.WorkSession{
		StaffID: staffID, Date: calendar.DateFromTime(checkIn),
		Status: workforce.WorkSessionStatusPresent, Source: workforce.WorkSessionSourceApp,
		CheckInTime: checkIn, CheckOutTime: &checkOut, CreatedBy: staffID,
	}
	session.SetTenantID(f.tenantID)
	createSessionFixture(t, f.workforce, f.ctx, session)
	return session
}

func (f *overviewFixture) insertRow(t *testing.T, query string, args ...any) int64 {
	t.Helper()
	var id int64
	require.NoError(t, f.db.NewRaw(query, args...).Scan(f.ctx, &id))
	return id
}

// addPayrollRules gives the school four wage types (one without a number),
// one rule of each kind and a school absence type booked onto a day wage
// type, and books the work and the absence that use them.
func addPayrollRules(t *testing.T, f *overviewFixture) {
	t.Helper()
	wageType := func(name, number, unit string) int64 {
		return f.insertRow(t, `INSERT INTO active.payroll_wage_types (tenant_id, name, number, unit) VALUES (?, ?, ?, ?) RETURNING id`,
			f.tenantID, name, number, unit)
	}
	night := wageType("Nachtzuschlag", "310", workforce.WageTypeUnitHours)
	sunday := wageType("Sonntagszuschlag", "320", workforce.WageTypeUnitAmount)
	holiday := wageType("Feiertagszuschlag", "330", workforce.WageTypeUnitHours)
	special := wageType("Sonderurlaub", "340", workforce.WageTypeUnitDays)
	wageType("Rufbereitschaft", "", workforce.WageTypeUnitHours)
	f.insertRow(t, `INSERT INTO active.payroll_surcharge_rules (tenant_id, kind, window_start_minute, window_end_minute, rate_type, rate_hundredths, wage_type_id)
		VALUES (?, 'nacht', 1200, 360, 'prozent', 2500, ?) RETURNING id`, f.tenantID, night)
	f.insertRow(t, `INSERT INTO active.payroll_surcharge_rules (tenant_id, kind, rate_type, rate_hundredths, wage_type_id)
		VALUES (?, 'sonntag', 'betrag', 250, ?) RETURNING id`, f.tenantID, sunday)
	f.insertRow(t, `INSERT INTO active.payroll_surcharge_rules (tenant_id, kind, rate_type, rate_hundredths, wage_type_id)
		VALUES (?, 'feiertag', 'prozent', 12500, ?) RETURNING id`, f.tenantID, holiday)
	absenceType := f.insertRow(t, `INSERT INTO active.staff_absence_types (tenant_id, name, base_type, is_active, wage_type_id)
		VALUES (?, 'Sonderurlaub', ?, true, ?) RETURNING id`, f.tenantID, workforce.AbsenceTypeOther, special)

	// Night: Tuesday 9 June 20:00 to Wednesday 02:00 with half an hour of
	// break at midnight; 330 net minutes in the 20:00 to 06:00 window.
	tuesday := calendar.NewDate(2026, time.June, 9)
	nightSession := f.addSessionBetween(t, f.staff[1], berlinAt(tuesday, 20, 0), berlinAt(tuesday.AddDays(1), 2, 0))
	breakEnd := berlinAt(tuesday.AddDays(1), 0, 30)
	createBreakFixture(t, f.workforce, f.ctx, &timetracking.WorkSessionBreak{
		SessionID: nightSession.ID, StartedAt: berlinAt(tuesday.AddDays(1), 0, 0), EndedAt: &breakEnd, DurationMinutes: 30,
	})
	// Sunday 14 June 10:00 to 14:00: four hours at 2,50 € are 10,00 €.
	f.addSessionBetween(t, f.staff[1], berlinAt(calendar.NewDate(2026, time.June, 14), 10, 0), berlinAt(calendar.NewDate(2026, time.June, 14), 14, 0))
	// Holiday Thursday 4 June 09:00 to 12:00: three hours.
	f.addSessionBetween(t, f.staff[1], berlinAt(calendar.NewDate(2026, time.June, 4), 9, 0), berlinAt(calendar.NewDate(2026, time.June, 4), 12, 0))

	// One day of Sonderurlaub on Thursday 11 June.
	absence := &timetracking.StaffAbsence{
		StaffID: f.staff[0], AbsenceType: workforce.AbsenceTypeOther, AbsenceTypeID: &absenceType,
		Status: workforce.AbsenceStatusApproved, CreatedBy: f.staff[0],
		DateStart: calendar.NewDate(2026, time.June, 11), DateEnd: calendar.NewDate(2026, time.June, 11),
	}
	absence.SetTenantID(f.tenantID)
	command := timetracking.StaffAbsenceToCapability(absence)
	_, err := f.reposAbsences.StaffAbsence.CreateStaffAbsence(f.ctx, &command)
	require.NoError(t, err)
}

// newPayrollRulesExportService builds the export over an overview that reads
// the school's wage types and the fixed holiday.
func (f *overviewFixture) newPayrollRulesExportService(t *testing.T) timetracking.StaffTimeExportService {
	t.Helper()
	settings := wtmIntSettings{accountStart: calendar.NewDate(f.today.Year(), f.today.Month(), 1).String()}
	overview := f.newOverviewService(settings, timetracking.WithOverviewPayrollRules(f.workforce, payrollHolidays{}))
	return timetracking.NewStaffTimeExportService(
		overview,
		f.newWorkSessionService(),
		owners.TimeExportStaff(f.repos.Staff),
		owners.NewTimeTrackingDataAccessAudit(owners.NewTestAuditPersistence(testpkg.MustDatabase(f.db), nil).DataAccessLog),
		api.NewPayrollExportSettings(settingstest.New(datevFullConfig()), f.countPersonnelNumbers),
		nil,
		owners.RenderTimeTrackingWorkbook,
	)
}

// TestDatevExport_PayrollRuleGoldenFiles pins both DATEV formats with
// night, Sunday and holiday work and a mapped absence type.
func TestDatevExport_PayrollRuleGoldenFiles(t *testing.T) {
	t.Parallel()

	f := newDatevFixture(t)
	f.setPersonnelNumber(t, f.staff[1], "1002")
	addPayrollRules(t, f)
	actorID := f.newActorAccount(t)
	svc := f.newPayrollRulesExportService(t)

	lodas, err := testpkg.TenantQuery3(t, f.ctx, testpkg.SetupTestDB(t), svc.Export, datevRequest(timetracking.ExportFormatDatevLodas), actorID, "admin")
	require.NoError(t, err)
	checkDatevGolden(t, "datev_lodas_payroll_rules.golden", lodas.Data)
	lug, err := testpkg.TenantQuery3(t, f.ctx, testpkg.SetupTestDB(t), svc.Export, datevRequest(timetracking.ExportFormatDatevLug), actorID, "admin")
	require.NoError(t, err)
	checkDatevGolden(t, "datev_lug_payroll_rules.golden", lug.Data)

	// Worked out by hand: the lines the goldens carry for the new wage types.
	lugText := string(lug.Data)
	for _, line := range []string{
		"1002;;;310;5,50;;;;;;",  // 330 night minutes
		"1002;;;320;;;10,00;;;;", // 4 h × 2,50 € in the Wert field
		"1002;;;330;3,00;;;;;;",  // 180 holiday minutes
		"1001;;;340;;1;;;;;",     // one day of Sonderurlaub
	} {
		assert.Contains(t, lugText, line)
	}
	assert.Contains(t, string(lodas.Data), "3;u_lod_bwd_buchung_standard;pnr#bwd;abrechnung_zeitraum#bwd;bs_wert_butab#bwd;bs_nr#bwd;la_eigene#bwd;")
	assert.Contains(t, string(lodas.Data), "3;1002;01.06.2026;10,00;02;320;")

	report, err := testpkg.TenantQuery1(t, f.ctx, testpkg.SetupTestDB(t), svc.DatevReport, datevRequest(timetracking.ExportFormatDatevLug))
	require.NoError(t, err)
	assert.Contains(t, report.UnconfiguredCategories, "Rufbereitschaft", "the preflight names a wage type without a number")
}

// TestDatevExport_SevenCategoriesUnchangedWithoutOwnWageTypes: with the rule
// reader wired but no wage type, both files equal the goldens of #1417.
func TestDatevExport_SevenCategoriesUnchangedWithoutOwnWageTypes(t *testing.T) {
	t.Parallel()

	f := newDatevFixture(t)
	actorID := f.newActorAccount(t)
	svc := f.newPayrollRulesExportService(t)

	lodas, err := testpkg.TenantQuery3(t, f.ctx, testpkg.SetupTestDB(t), svc.Export, datevRequest(timetracking.ExportFormatDatevLodas), actorID, "admin")
	require.NoError(t, err)
	checkDatevGolden(t, "datev_lodas.golden", lodas.Data)
	lug, err := testpkg.TenantQuery3(t, f.ctx, testpkg.SetupTestDB(t), svc.Export, datevRequest(timetracking.ExportFormatDatevLug), actorID, "admin")
	require.NoError(t, err)
	checkDatevGolden(t, "datev_lug.golden", lug.Data)
}

// TestMonthExport_CSVCarriesTheWageTypeColumns: CSV shows the same values as
// the DATEV lines, one column per wage type.
func TestMonthExport_CSVCarriesTheWageTypeColumns(t *testing.T) {
	t.Parallel()

	f := newDatevFixture(t)
	addPayrollRules(t, f)
	actorID := f.newActorAccount(t)
	svc := f.newPayrollRulesExportService(t)

	request := timetracking.TimeExportRequest{Year: 2026, Month: 6, Format: timetracking.ExportFormatCSV, TimeFormat: timetracking.ExportTimeDecimal}
	file, err := testpkg.TenantQuery3(t, f.ctx, testpkg.SetupTestDB(t), svc.Export, request, actorID, "admin")
	require.NoError(t, err)
	lines := strings.Split(strings.TrimPrefix(strings.ReplaceAll(string(file.Data), "\r\n", "\n"), "\ufeff"), "\n")
	header := strings.Split(lines[0], ";")
	assert.Equal(t, []string{"Nachtzuschlag (Stunden)", "Sonntagszuschlag (Euro)", "Feiertagszuschlag (Stunden)", "Sonderurlaub (Tage)", "Rufbereitschaft (Stunden)"},
		header[len(header)-5:])
	var staffOne, staffZero, staffZeroRow []string
	for _, line := range lines[1:] {
		cells := strings.Split(line, ";")
		if len(cells) < len(header) {
			continue
		}
		switch {
		case cells[len(cells)-4] == "10,00":
			staffOne = cells[len(cells)-5:]
		case cells[len(cells)-2] == "1":
			staffZero = cells[len(cells)-5:]
			staffZeroRow = cells
		}
	}
	assert.Equal(t, []string{"5,50", "10,00", "3,00", "", ""}, staffOne)
	assert.Equal(t, []string{"", "", "", "1", ""}, staffZero)
	otherColumn := -1
	for i, label := range header {
		if label == "Gutschrift Sonstige" {
			otherColumn = i
			break
		}
	}
	require.NotEqual(t, -1, otherColumn)
	require.Len(t, staffZeroRow, len(header))
	assert.Equal(t, "0,00", staffZeroRow[otherColumn], "a mapped custom absence must not also be exported as Sonstige")
}
