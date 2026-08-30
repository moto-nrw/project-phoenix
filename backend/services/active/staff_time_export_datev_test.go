package active_test

import (
	"bytes"
	"context"
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/services/active"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// DATEV writer tests (#1417 2b, final part).
//
// The export month is FIXED (June 2026, before the fixture's account anchor,
// so it computes as a standalone month) — that keeps both golden files
// byte-stable regardless of the day the suite runs. Every value in the files
// is additionally pinned against GetMonthSummary of the same person, so the
// DATEV lines cannot drift from the Monatskarte.

var updateDatevGoldens = flag.Bool("update-datev-goldens", false, "rewrite the DATEV golden files")

// datevFullConfig configures every category: hours for the pure minute sums
// and Urlaub, days for Krank and Fortbildung — both unit paths in one file.
func datevFullConfig() map[string]string {
	return map[string]string{
		configModel.KeyPayrollLohnartRegelarbeit:       "100",
		configModel.KeyPayrollLohnartPlusStunden:       "110",
		configModel.KeyPayrollLohnartAuszahlung:        "120",
		configModel.KeyPayrollLohnartFreizeitausgleich: "130",
		configModel.KeyPayrollLohnartKrank:             "200",
		configModel.KeyPayrollLohnartUrlaub:            "210",
		configModel.KeyPayrollLohnartFortbildung:       "220",
		configModel.KeyPayrollEinheitKrank:             configModel.PayrollUnitDays,
		configModel.KeyPayrollEinheitUrlaub:            configModel.PayrollUnitHours,
		configModel.KeyPayrollEinheitFortbildung:       configModel.PayrollUnitDays,
		configModel.KeyPayrollDatevBeraternummer:       "1234567",
		configModel.KeyPayrollDatevMandantennummer:     "54321",
	}
}

func (f *overviewFixture) newDatevExportService(values map[string]string) active.StaffTimeExportService {
	settings := &configtest.Mock{
		ResolveStringFn: func(_ context.Context, key string) (string, error) {
			return values[key], nil
		},
	}
	payrollStatus := configSvc.NewPayrollStatusService(settings, testpkg.PersonnelNumberCounter(f.repos.Staff))
	return active.NewStaffTimeExportService(
		f.svc,
		f.newWorkSessionService(),
		f.repos.Staff,
		f.repos.DataAccessLog,
		payrollStatus,
		nil,
	)
}

func (f *overviewFixture) setPersonnelNumber(t *testing.T, staffID int64, number string) {
	t.Helper()
	_, err := f.db.NewUpdate().
		Table("users.staff").
		Set("personnel_number = ?", number).
		Where("id = ?", staffID).
		Exec(f.ctx)
	require.NoError(t, err)
}

// newDatevFixture builds two staff members with deterministic June 2026 data.
// Both carry personnel numbers so exports pass the mandatory preflight;
// staff[0] also has data for every booking category.
func newDatevFixture(t *testing.T) *overviewFixture {
	f := newOverviewFixture(t, 2)
	f.setPersonnelNumber(t, f.staff[0].ID, "1001")
	f.setPersonnelNumber(t, f.staff[1].ID, "1002")

	// June 2026 bookings for staff[0]: 2×8h work, 1 sick day, 2 vacation
	// days, 1 training day, one payout and one comp-time booking.
	f.addSession(t, f.staff[0].ID, timezone.NewDate(2026, time.June, 1), 8*time.Hour)
	f.addSession(t, f.staff[0].ID, timezone.NewDate(2026, time.June, 2), 8*time.Hour)
	f.addAbsence(t, f.staff[0].ID, activeModels.AbsenceTypeSick, activeModels.AbsenceStatusReported,
		timezone.NewDate(2026, time.June, 3), timezone.NewDate(2026, time.June, 3))
	f.addAbsence(t, f.staff[0].ID, activeModels.AbsenceTypeVacation, activeModels.AbsenceStatusApproved,
		timezone.NewDate(2026, time.June, 4), timezone.NewDate(2026, time.June, 5))
	f.addAbsence(t, f.staff[0].ID, activeModels.AbsenceTypeTraining, activeModels.AbsenceStatusApproved,
		timezone.NewDate(2026, time.June, 8), timezone.NewDate(2026, time.June, 8))
	for _, adj := range []struct {
		typ   string
		delta int
	}{
		{activeModels.BalanceAdjustmentTypePayout, -600},
		{activeModels.BalanceAdjustmentTypeCompTime, -120},
	} {
		adjustment := &activeModels.StaffBalanceAdjustment{
			StaffID:       f.staff[0].ID,
			Type:          adj.typ,
			MinutesDelta:  adj.delta,
			EffectiveDate: timezone.NewDate(2026, time.June, 10),
			Note:          "Buchung",
			DecidedBy:     f.staff[0].ID,
			DecidedAt:     time.Now(),
		}
		adjustment.SetTenantID(f.tenantID)
		require.NoError(t, f.repos.StaffBalanceAdjust.Create(f.ctx, adjustment))
	}
	return f
}

func datevRequest(format string) active.TimeExportRequest {
	return active.TimeExportRequest{Year: 2026, Month: 6, Format: format}
}

// checkDatevGolden compares CONTENT with LF-normalized goldens: the repo
// ignores .gitattributes and core.autocrlf=input would silently rewrite a
// CRLF golden on commit. The CRLF wire requirement is pinned separately by
// an explicit assertion on the produced bytes.
func checkDatevGolden(t *testing.T, name string, data []byte) {
	t.Helper()
	normalized := strings.ReplaceAll(string(data), "\r\n", "\n")
	path := filepath.Join("testdata", name)
	if *updateDatevGoldens {
		require.NoError(t, os.MkdirAll("testdata", 0o755))
		require.NoError(t, os.WriteFile(path, []byte(normalized), 0o644))
		return
	}
	expected, err := os.ReadFile(path)
	require.NoError(t, err, "golden %s missing — regenerate with -update-datev-goldens", name)
	assert.Equal(t, string(expected), normalized,
		"golden diff for %s — if intentional, regenerate with -update-datev-goldens and justify in the PR", name)
}

// TestDatevExport_GoldenFiles pins both file formats byte for byte,
// including the CRLF line ends the LODAS handbook requires.
func TestDatevExport_GoldenFiles(t *testing.T) {
	t.Parallel()

	f := newDatevFixture(t)
	actorID := f.newActorAccount(t)
	f.cleanupAccessLogs(t)
	svc := f.newDatevExportService(datevFullConfig())

	lodas, err := svc.Export(f.ctx, datevRequest(active.ExportFormatDatevLodas), actorID, "admin")
	require.NoError(t, err)
	assert.Equal(t, "datev_lodas_2026-06.txt", lodas.Filename)
	assert.Equal(t, "text/plain; charset=windows-1252", lodas.ContentType)
	assert.True(t, bytes.Contains(lodas.Data, []byte("\r\n")), "LODAS requires CR/LF")
	checkDatevGolden(t, "datev_lodas.golden", lodas.Data)

	lug, err := svc.Export(f.ctx, datevRequest(active.ExportFormatDatevLug), actorID, "admin")
	require.NoError(t, err)
	assert.Equal(t, "datev_lug_2026-06.txt", lug.Filename)
	checkDatevGolden(t, "datev_lug.golden", lug.Data)
}

// TestDatevExport_PinsAgainstMonthSummary re-derives every exported value
// from the Monatskarte of the same person — the file cannot carry a number
// the card does not show.
func TestDatevExport_PinsAgainstMonthSummary(t *testing.T) {
	t.Parallel()

	f := newDatevFixture(t)
	actorID := f.newActorAccount(t)
	f.cleanupAccessLogs(t)
	svc := f.newDatevExportService(datevFullConfig())

	summary, err := f.monthSvc.GetMonthSummary(f.ctx, f.staff[0].ID, 2026, 6)
	require.NoError(t, err)

	file, err := svc.Export(f.ctx, datevRequest(active.ExportFormatDatevLug), actorID, "admin")
	require.NoError(t, err)

	// Parse the LuG lines: Personalnummer;;;Lohnart;Stunden;Tage;...
	hoursByLohnart := map[string]string{}
	daysByLohnart := map[string]string{}
	lines := strings.Split(strings.TrimRight(string(file.Data), "\r\n"), "\r\n")
	require.Greater(t, len(lines), 1)
	for _, line := range lines[1:] {
		fields := strings.Split(line, ";")
		require.Len(t, fields, 11)
		assert.Equal(t, "1001", fields[0], "only staff[0] carries a personnel number")
		if fields[4] != "" {
			hoursByLohnart[fields[3]] = fields[4]
		}
		if fields[5] != "" {
			daysByLohnart[fields[3]] = fields[5]
		}
	}

	toHours := func(minutes int) string {
		return strings.Replace(activeMinutesToDecimal(minutes), ".", ",", 1)
	}
	assert.Equal(t, toHours(summary.ActualMinutes), hoursByLohnart["100"], "Regelarbeit == Ist")
	assert.Equal(t, toHours(600), hoursByLohnart["120"], "Auszahlung == sign-inverted payout booking")
	assert.Equal(t, toHours(120), hoursByLohnart["130"], "Freizeitausgleich == sign-inverted comp-time booking")
	assert.Equal(t, toHours(summary.CreditedVacationMinutes), hoursByLohnart["210"], "Urlaub in hours == credited vacation")
	assert.Equal(t, "1", daysByLohnart["200"], "Krank in days == sick days")
	assert.Equal(t, "1", daysByLohnart["220"], "Fortbildung in days == training days")
	assert.Equal(t, float64(1), summary.SickDays)
	assert.Equal(t, float64(1), summary.TrainingDays)
	// June is deeply negative (full-month Soll vs 2 worked days): no
	// Plus-Stunden line may exist.
	_, plusHours := hoursByLohnart["110"]
	assert.False(t, plusHours, "negative month balance must not book Plus-Stunden")
	require.Negative(t, summary.BalanceMinutes)
}

// TestDatevExport_RefusesStaffWithoutPersonnelNumber: the report identifies
// the gap, but no file or audit row is produced until every staff member has
// a personnel number.
func TestDatevExport_RefusesStaffWithoutPersonnelNumber(t *testing.T) {
	t.Parallel()

	f := newDatevFixture(t)
	f.setPersonnelNumber(t, f.staff[1].ID, "")
	actorID := f.newActorAccount(t)
	f.cleanupAccessLogs(t)
	svc := f.newDatevExportService(datevFullConfig())

	report, err := svc.DatevReport(f.ctx, datevRequest(active.ExportFormatDatevLug))
	require.NoError(t, err)
	assert.Equal(t, 1, report.StaffExported)
	require.Len(t, report.StaffSkipped, 1)
	assert.Equal(t, "keine Personalnummer", report.StaffSkipped[0].Reason)
	assert.Empty(t, report.UnconfiguredCategories)
	assert.True(t, report.OpenMonth, "June was never closed — the report must say so")

	file, err := svc.Export(f.ctx, datevRequest(active.ExportFormatDatevLug), actorID, "admin")
	require.ErrorIs(t, err, active.ErrPayrollConfigIncomplete)
	assert.Nil(t, file)

	var logs []*auditModels.DataAccessLog
	require.NoError(t, f.db.NewSelect().
		Model(&logs).
		ModelTableExpr(`audit.data_access_log AS "data_access_log"`).
		Where("tenant_id = ?", f.tenantID).
		Scan(context.Background()))
	assert.Empty(t, logs)
}

// TestDatevExport_RefusesIncompleteConfiguration: no file from missing LODAS
// header identifiers or a fully unconfigured Lohnart mapping.
func TestDatevExport_RefusesIncompleteConfiguration(t *testing.T) {
	t.Parallel()

	f := newDatevFixture(t)
	actorID := f.newActorAccount(t)
	f.cleanupAccessLogs(t)

	// LODAS without Berater-/Mandantennummer → refused; LuG does not need them.
	config := datevFullConfig()
	delete(config, configModel.KeyPayrollDatevBeraternummer)
	svc := f.newDatevExportService(config)
	_, err := svc.Export(f.ctx, datevRequest(active.ExportFormatDatevLodas), actorID, "admin")
	require.ErrorIs(t, err, active.ErrPayrollConfigIncomplete)
	_, err = svc.Export(f.ctx, datevRequest(active.ExportFormatDatevLug), actorID, "admin")
	require.NoError(t, err, "Lohn und Gehalt does not need the LODAS header")

	// No configured category at all → refused for both.
	svc = f.newDatevExportService(map[string]string{
		configModel.KeyPayrollDatevBeraternummer:   "1234567",
		configModel.KeyPayrollDatevMandantennummer: "54321",
	})
	_, err = svc.Export(f.ctx, datevRequest(active.ExportFormatDatevLodas), actorID, "admin")
	require.ErrorIs(t, err, active.ErrPayrollConfigIncomplete)

	// A category whose number is set but whose required unit is missing does
	// not count as configured and exports no line.
	config = datevFullConfig()
	delete(config, configModel.KeyPayrollEinheitKrank)
	svc = f.newDatevExportService(config)
	report, err := svc.DatevReport(f.ctx, datevRequest(active.ExportFormatDatevLug))
	require.NoError(t, err)
	assert.Contains(t, report.UnconfiguredCategories, "Krank")
}

// TestDatevExport_RequiresSingleMonth: payroll is monthly; year spans and day
// granularity are rejected for the DATEV formats.
func TestDatevExport_RequiresSingleMonth(t *testing.T) {
	t.Parallel()

	f := newOverviewFixture(t, 1)
	actorID := f.newActorAccount(t)
	f.cleanupAccessLogs(t)
	svc := f.newDatevExportService(datevFullConfig())

	req := active.TimeExportRequest{Year: 2026, Format: active.ExportFormatDatevLodas}
	_, err := svc.Export(f.ctx, req, actorID, "admin")
	require.ErrorIs(t, err, active.ErrTimeExportInvalid)

	req = datevRequest(active.ExportFormatDatevLug)
	req.Granularity = active.ExportGranularityDay
	_, err = svc.Export(f.ctx, req, actorID, "admin")
	require.ErrorIs(t, err, active.ErrTimeExportInvalid)
}

func TestDatevReport_RejectsNonDatevFormats(t *testing.T) {
	t.Parallel()

	svc := active.NewStaffTimeExportService(nil, nil, nil, nil, nil, nil)

	for _, format := range []string{"", active.ExportFormatCSV, active.ExportFormatXLSX} {
		_, err := svc.DatevReport(context.Background(), active.TimeExportRequest{
			Year:   2026,
			Month:  6,
			Format: format,
		})
		require.ErrorIs(t, err, active.ErrTimeExportInvalid, "format %q", format)
	}
}

// TestDatevExport_NoFileWithoutAudit: same contract as CSV/XLSX (#2005).
func TestDatevExport_NoFileWithoutAudit(t *testing.T) {
	t.Parallel()

	f := newDatevFixture(t)
	actorID := f.newActorAccount(t)
	settings := &configtest.Mock{
		ResolveStringFn: func(_ context.Context, key string) (string, error) {
			return datevFullConfig()[key], nil
		},
	}
	svc := active.NewStaffTimeExportService(
		f.svc, f.newWorkSessionService(), f.repos.Staff, failingAccessLogRepo{},
		configSvc.NewPayrollStatusService(settings, testpkg.PersonnelNumberCounter(f.repos.Staff)), nil,
	)

	file, err := svc.Export(f.ctx, datevRequest(active.ExportFormatDatevLodas), actorID, "admin")
	require.Error(t, err)
	assert.Nil(t, file)
}

// activeMinutesToDecimal mirrors the writers' decimal-hour rendering with a
// dot so the test can flip it to the German comma explicitly.
func activeMinutesToDecimal(minutes int) string {
	return strconv.FormatFloat(float64(minutes)/60.0, 'f', 2, 64)
}
