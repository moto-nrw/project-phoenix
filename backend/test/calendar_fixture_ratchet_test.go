package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// calendarFixtureClockExceptions contains only tests whose purpose requires
// the system clock. Every exact function key needs its own reviewed reason.
var calendarFixtureClockExceptions = map[string]string{
	"services/enrollment/offering_change_request_service_test.go:TestOfferingChangeRequestService_ListPending_ReportsTheSelectableDateRange": "Verifies the queue boundary derived from the current date.",
}

func TestCalendarFixtureClockRatchet(t *testing.T) {
	t.Parallel()

	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Skipf("Could not find backend root: %v", err)
	}
	findings, err := scanCalendarFixtureClockRisks(backendRoot)
	if err == nil {
		findings, err = applyCalendarClockExceptions(findings, calendarFixtureClockExceptions)
	}
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) == 0 {
		return
	}
	t.Errorf("Calendar fixture wall-clock ratchet failed (%d finding(s)):\n\n%s\n\n"+
		"These fixtures can cross a Berlin date or ISO-week boundary depending on when CI runs. "+
		"Use timezone.NewDate(...), BerlinMidnight(), or time.Date(...) with a fixed instant. "+
		"If the behavior must observe the live clock, inject it or add the exact file:test key to "+
		"calendarFixtureClockExceptions with a reviewed, non-empty reason.",
		len(findings), strings.Join(formatCalendarClockFindings(findings), "\n"))
}

func TestCalendarFixtureRatchetDetectsEnrollmentPattern(t *testing.T) {
	t.Parallel()

	root := writeCalendarFixtureSource(t, "enrollment/history_test.go", `package enrollment

import (
	stdtime "time"
	tz "github.com/moto-nrw/project-phoenix/internal/timezone"
)

func TestHistoryPeriod(t *testing.T) {
	t.Parallel()
	base := stdtime.Now().UTC().Add(-2 * stdtime.Hour)
	today := tz.DateFromTime(base).String()
	_ = today
}
`)

	findings, err := scanCalendarFixtureClockRisks(root)
	if err != nil {
		t.Fatal(err)
	}
	requireCalendarFinding(t, findings, "enrollment/history_test.go:11", "TestHistoryPeriod")
}

func TestCalendarFixtureRatchetDetectsWorkSessionPattern(t *testing.T) {
	t.Parallel()

	root := writeCalendarFixtureSource(t, "active/work_session_test.go", `package active

import (
	"time"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
)
type liveClock struct{}
func (liveClock) Now() time.Time { return time.Now() }

func TestHistorySummary(t *testing.T) {
	t.Parallel()
	from := timezone.TodayDate().AddDays(-7)
	to := timezone.TodayDate()
	checkIn := time.Now().Add(-8 * time.Hour)
	checkOut := time.Now().Add(-2 * time.Hour)
	session := WorkSession{CheckInTime: checkIn, CheckOutTime: &checkOut}
	updated := WorkSession{}
	updated.CheckInTime = time.Now()
	history := GetHistory(session, from, to)
	require.Len(t, history.WeeklySummaries, 1)
	updatedHistory := GetHistory(updated, from, to)
	_ = updatedHistory.WeeklySummaries
	other := GetHistory(NewWorkSession(fixtureNow()), from, to)
	_ = other.WeeklySummaries
	structured := GetHistory(fixtureSession(), from, to)
	_ = structured.WeeklySummaries
	methodHistory := GetHistory(WorkSession{CheckInTime: liveClock{}.Now()}, from, to)
	_ = methodHistory.WeeklySummaries
}
`)
	writeLiveInstantHelper(t, root)

	findings, err := scanCalendarFixtureClockRisks(root)
	if err != nil {
		t.Fatal(err)
	}
	requireCalendarFinding(t, findings,
		"TestHistorySummary",
		"live calendar date shifted into a range",
		"live instant feeds an ISO-week expectation",
	)
}

func writeLiveInstantHelper(t *testing.T, root string) {
	t.Helper()

	writeCalendarFixtureSourceAt(t, root, "active/clock_helpers_test.go", `package active
import "time"
func fixtureNow() time.Time {
	now := time.Now()
	return now
}
func fixtureSession() WorkSession {
	session := WorkSession{}
	session.CheckInTime = time.Now()
	return session
}
`)
}

func TestCalendarFixtureRatchetDetectsLiveDateRange(t *testing.T) {
	t.Parallel()

	root := writeCalendarFixtureSource(t, "sample/history_test.go", `package sample
import (
	"testing"
	tz "github.com/moto-nrw/project-phoenix/internal/timezone"
)
type DateRange struct { From, To tz.Date }
func TestHistoryRange(t *testing.T) {
	t.Parallel()
	from := tz.TodayDate().AddDays(-7)
	to := tz.TodayDate()
	fixed := tz.NewDate(2026, 8, 30)
	_ = GetHistory(from, fixed)
	_ = FindByDateRange(fixed, to)
	_ = FindByDateRange(DateRange{From: from, To: fixed})
	_ = List(from, to)
	dateRange := DateRange{From: fixed, To: fixed}
	dateRange.From = tz.TodayDate()
	_ = List(dateRange)
	alias := dateRange
	_ = List(alias)
	_ = List(&DateRange{From: tz.TodayDate(), To: fixed})
}
func BenchmarkHistoryRange(b *testing.B) { _ = GetHistory(tz.TodayDate(), tz.NewDate(2026, 8, 30)) }
func FuzzHistoryRange(f *testing.F) { _ = FindByDateRange(tz.TodayDate(), tz.NewDate(2026, 8, 30)) }
func ExampleHistoryRange() { _ = GetHistory(tz.TodayDate(), tz.NewDate(2026, 8, 30)) }
`)

	findings, err := scanCalendarFixtureClockRisks(root)
	if err != nil {
		t.Fatal(err)
	}
	requireCalendarFinding(t, findings,
		"TestHistoryRange",
		"BenchmarkHistoryRange",
		"FuzzHistoryRange",
		"ExampleHistoryRange",
		"live clock defines a calendar range",
	)
}

func TestCalendarFixtureRatchetFollowsLiveDateHelperIntoAssertion(t *testing.T) {
	t.Parallel()

	root := writeCalendarFixtureSource(t, "sample/helper_test.go", `package sample
import (
	"testing"
	assertpkg "github.com/stretchr/testify/assert"
	tz "github.com/moto-nrw/project-phoenix/internal/timezone"
)
type fixtureClock struct{}
func fixtureDate() tz.Date {
	return crossFileDate()
}
func (fixtureClock) today() tz.Date { return tz.TodayDate() }
func TestCalendarExpectation(t *testing.T) {
	t.Parallel()
	got := struct{ Date tz.Date }{}
	assertpkg.Equal(t, fixtureDate(), got.Date)
	assertpkg.False(t, got.Date.After(fixtureClock{}.today()))
	if got.Date != fixtureDate() { t.Fail() }
}
`)
	writeCalendarFixtureSourceAt(t, root, "sample/date_helpers_test.go", `package sample
import tz "github.com/moto-nrw/project-phoenix/internal/timezone"
func crossFileDate() tz.Date {
	today := tz.TodayDate()
	return today
}
`)

	findings, err := scanCalendarFixtureClockRisks(root)
	if err != nil {
		t.Fatal(err)
	}
	requireCalendarFinding(t, findings, "TestCalendarExpectation", "live calendar date used as an expectation")
}

func TestCalendarFixtureRatchetRequiresReviewedExceptionReason(t *testing.T) {
	t.Parallel()

	root := writeCalendarFixtureSource(t, "sample/range_test.go", `package sample
import tz "github.com/moto-nrw/project-phoenix/internal/timezone"
func TestLiveRange(t *testing.T) {
	t.Parallel()
	from := tz.TodayDate().AddDays(-7)
	_ = from.Weekday()
}
`)
	key := "sample/range_test.go:TestLiveRange"

	findings, err := scanCalendarFixtureClockRisks(root)
	if err != nil {
		t.Fatal(err)
	}
	assertCalendarExceptionAccepted(t, findings, key)
	assertCalendarExceptionRejected(t, findings, map[string]string{key: ""}, "non-empty reason")
	assertCalendarExceptionRejected(t, findings,
		map[string]string{"sample/range_test.go:TestOther": "typo"}, "no matching finding")
}

func assertCalendarExceptionAccepted(t *testing.T, findings []calendarClockFinding, key string) {
	t.Helper()

	remaining, err := applyCalendarClockExceptions(findings, map[string]string{
		key: "the production contract is explicitly relative to the current Berlin day",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 0 {
		t.Fatalf("reviewed exception did not suppress its exact test: %v", remaining)
	}
}

func assertCalendarExceptionRejected(t *testing.T, findings []calendarClockFinding, exceptions map[string]string, want string) {
	t.Helper()

	_, err := applyCalendarClockExceptions(findings, exceptions)
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("exception error %v does not contain %q", err, want)
	}
}

func TestCalendarFixtureRatchetIgnoresFixedAndNonCodePatterns(t *testing.T) {
	t.Parallel()

	safeRoot := writeCalendarFixtureSource(t, "sample/fixed_test.go", `package sample
import (
	"time"
	"testing"
	tz "github.com/moto-nrw/project-phoenix/internal/timezone"
)
type fakeClock struct{}
type suite struct{}
func (fakeClock) Now() time.Time { return time.Time{} }
func freshnessSession() WorkSession {
	return WorkSession{CheckInTime: time.Date(2026, 8, 19, 8, 0, 0, 0, tz.Berlin), UpdatedAt: time.Now()}
}

func (suite) TestMethod(t *testing.T) {
	t.Parallel()
	_ = tz.TodayDate().Weekday()
}

func Testament(t *testing.T) {
	t.Parallel()
	_ = tz.TodayDate().Weekday()
}
func TestFixedFixtures(t *testing.T) {
	t.Parallel()
	base := tz.NewDate(2026, 8, 19).BerlinMidnight().Add(12 * time.Hour)
	from := tz.NewDate(2026, 8, 16)
	to := tz.NewDate(2026, 8, 22)
	checkIn := time.Date(2026, 8, 19, 8, 0, 0, 0, tz.Berlin)
	elapsedStart := time.Now()
	history := struct{ WeeklySummaries []int }{}
	freshHistory := GetHistory(WorkSession{CheckInTime: checkIn, UpdatedAt: time.Now()})
	helperHistory := GetHistory(freshnessSession())
	time := fakeClock{}
	_ = []any{base, from, to, checkIn, elapsedStart, history.WeeklySummaries, freshHistory.WeeklySummaries, helperHistory.WeeklySummaries, time.Now(), "time.Now().Add(-2h)"}
	// timezone.TodayDate().AddDays(-7) is documentation, not syntax.
}
`)
	assertNoCalendarFindings(t, safeRoot)

	productionRoot := writeCalendarFixtureSource(t, "sample/production.go", `package sample
import tz "github.com/moto-nrw/project-phoenix/internal/timezone"
func production() { _ = tz.TodayDate().AddDays(-7).Weekday() }
`)
	assertNoCalendarFindings(t, productionRoot)
}

func TestCalendarFixtureRatchetIgnoresUnixNanoUsedForFixtureUniqueness(t *testing.T) {
	t.Parallel()

	root := writeCalendarFixtureSource(t, "schedule/fixture_test.go", `package schedule

import (
	"fmt"
	"time"
	tz "github.com/moto-nrw/project-phoenix/internal/timezone"
)

type fixture struct {
	name string
	date time.Time
}

func newFixture() *fixture {
	suffix := time.Now().UnixNano()
	return &fixture{
		name: fmt.Sprintf("fixture-%d", suffix),
		date: time.Date(2030, 8, 26, 12, 0, 0, 0, time.UTC),
	}
}

func TestFixedDateFromUniquelyNamedFixture(t *testing.T) {
	t.Parallel()
	got := tz.DateFromTime(newFixture().date)
	if got != tz.NewDate(2030, 8, 26) {
		t.Fatal(got)
	}
}
`)

	findings, err := scanCalendarFixtureClockRisks(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("UnixNano fixture suffix produced false calendar finding: %#v", findings)
	}
}

func TestCalendarFixtureRatchetDetectsUnixNanoUsedForCalendarOffset(t *testing.T) {
	t.Parallel()

	root := writeCalendarFixtureSource(t, "schedule/fixture_test.go", `package schedule

import (
	"testing"
	"time"
	tz "github.com/moto-nrw/project-phoenix/internal/timezone"
)

func TestWeeklyFixtureWithUnixNanoOffset(t *testing.T) {
	t.Parallel()
	offset := time.Now().UnixNano()
	date := tz.NewDate(2030, 8, 26).AddDays(int(offset % 100))
	history := GetHistory(date)
	_ = history.WeeklySummaries
}
`)

	findings, err := scanCalendarFixtureClockRisks(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].function != "TestWeeklyFixtureWithUnixNanoOffset" {
		t.Fatalf("UnixNano calendar offset findings = %#v, want one weekly fixture finding", findings)
	}
}

func TestCalendarFixtureRatchetDetectsInstantReconstructedFromLiveUnixScalar(t *testing.T) {
	t.Parallel()

	root := writeCalendarFixtureSource(t, "schedule/fixture_test.go", `package schedule

import (
	"testing"
	"time"
	tz "github.com/moto-nrw/project-phoenix/internal/timezone"
)

func TestDateReconstructedFromLiveUnixScalar(t *testing.T) {
	t.Parallel()
	seconds := time.Now().Unix()
	date := tz.DateFromTime(time.Unix(seconds, 0))
	_ = date
}
`)

	findings, err := scanCalendarFixtureClockRisks(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].function != "TestDateReconstructedFromLiveUnixScalar" {
		t.Fatalf("reconstructed Unix instant findings = %#v, want one calendar conversion finding", findings)
	}
}

func TestCalendarFixtureRatchetDetectsReconstructedInstantInWeeklyFixture(t *testing.T) {
	t.Parallel()

	root := writeCalendarFixtureSource(t, "schedule/fixture_test.go", `package schedule

import (
	"testing"
	"time"
)

func TestWeeklyFixtureReconstructedFromLiveUnixScalar(t *testing.T) {
	t.Parallel()
	session := WorkSession{CheckInTime: time.Unix(time.Now().Unix(), 0)}
	history := GetHistory(session)
	_ = history.WeeklySummaries
}
`)

	findings, err := scanCalendarFixtureClockRisks(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].function != "TestWeeklyFixtureReconstructedFromLiveUnixScalar" {
		t.Fatalf("weekly reconstructed Unix instant findings = %#v, want one weekly fixture finding", findings)
	}
}

func TestCalendarFixtureRatchetFollowsUnixScalarHelpersAcrossFiles(t *testing.T) {
	t.Parallel()

	root := writeCalendarFixtureSource(t, "schedule/fixture_test.go", `package schedule

import (
	"testing"
	tz "github.com/moto-nrw/project-phoenix/internal/timezone"
)

func TestDateOffsetFromLiveScalarHelper(t *testing.T) {
	t.Parallel()
	offset := liveOffset()
	date := tz.NewDate(2030, 8, 26).AddDays(int(offset % 100))
	_ = date
}
`)
	writeCalendarFixtureSourceAt(t, root, "schedule/helpers_test.go", `package schedule

import "time"

func liveOffset() int64 { return time.Now().UnixNano() }
`)

	findings, err := scanCalendarFixtureClockRisks(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].function != "TestDateOffsetFromLiveScalarHelper" {
		t.Fatalf("cross-file scalar helper findings = %#v, want one calendar offset finding", findings)
	}
}

func TestCalendarFixtureRatchetKeepsScopesAndNowMethodsSeparate(t *testing.T) {
	t.Parallel()

	root := writeCalendarFixtureSource(t, "sample/scopes_test.go", `package sample
import (
	"testing"
	"time"
)
type liveClock struct{}
type fakeClock struct{}
func (liveClock) Now() time.Time { return time.Now() }
func (fakeClock) Now() time.Time { return time.Time{} }
func TestElapsedAndWeeklyScopes(t *testing.T) {
	t.Parallel()
	t.Run("elapsed", func(t *testing.T) {
		t.Parallel()
		result := measure(time.Now())
		_ = result
	})
	t.Run("weekly", func(t *testing.T) {
		t.Parallel()
		result := GetHistory(fakeClock{}.Now())
		_ = result.WeeklySummaries
	})
}
`)
	assertNoCalendarFindings(t, root)
}

func TestCalendarFixtureRatchetQualifiesNowMethodsByReceiver(t *testing.T) {
	t.Parallel()

	root := writeCalendarFixtureSource(t, "sample/methods_test.go", `package sample
import (
	"testing"
	"time"
)
type liveClock struct{}
type fakeClock struct{}
func (liveClock) Now() time.Time { return time.Now() }
func (fakeClock) Now() time.Time { return time.Time{} }
func currentISOWeekday() int { return int(time.Now().Weekday()) }
func delegatedISOWeekday() int { return currentISOWeekday() }
func weekdayFromFixtureDate() int { return fixtureDate().Weekday() }
func TestLiveMethod(t *testing.T) {
	t.Parallel()
	history := GetHistory(WorkSession{CheckInTime: liveClock{}.Now()})
	_ = history.WeeklySummaries
}
func TestFakeMethod(t *testing.T) {
	t.Parallel()
	history := GetHistory(WorkSession{CheckInTime: fakeClock{}.Now()})
	_ = history.WeeklySummaries
}
func TestFactoryMethod(t *testing.T) {
	t.Parallel()
	history := GetHistory(WorkSession{CheckInTime: factoryTime()})
	_ = history.WeeklySummaries
}
func TestExplicitReceiverTypeConverges(t *testing.T) {
	t.Parallel()
	var clock Clock
	clock = liveClock{}
	history := GetHistory(WorkSession{CheckInTime: clock.Now()})
	_ = history.WeeklySummaries
}
func TestRepeatedInterfaceAssignmentsUseLastConcreteClock(t *testing.T) {
	t.Parallel()
	var clock Clock
	clock = liveClock{}
	clock = fakeClock{}
	history := GetHistory(WorkSession{CheckInTime: clock.Now()})
	_ = history.WeeklySummaries
}
func TestAnonymousRange(t *testing.T) {
	t.Parallel()
	_ = List(struct{ From, To Date }{From: TodayDate(), To: fixedDate})
}
func TestLiveWeekdayFixture(t *testing.T) {
	t.Parallel()
	_ = map[string]int{"weekday": currentISOWeekday()}
}
func TestIndirectLiveWeekdayFixture(t *testing.T) {
	t.Parallel()
	_ = map[string]int{"weekday": delegatedISOWeekday()}
}
func TestWeekdayFromLiveDateHelper(t *testing.T) {
	t.Parallel()
	_ = map[string]int{"weekday": weekdayFromFixtureDate()}
}
`)
	writeCalendarFixtureSourceAt(t, root, "sample/factory_test.go", `package sample
import "time"
type Clock interface { Now() time.Time }
func newLiveClock() liveClock { return liveClock{} }
func factoryTime() time.Time { return newLiveClock().Now() }
`)
	writeCalendarFixtureSourceAt(t, root, "sample/live_date_test.go", `package sample
import (
	. "time"
	. "github.com/stretchr/testify/assert"
	assertpkg "github.com/stretchr/testify/assert"
	tz "github.com/moto-nrw/project-phoenix/internal/timezone"
)
type Date = tz.Date
var fixedDate = tz.NewDate(2026, 8, 30)
func TodayDate() Date { return tz.TodayDate() }
func fixtureNow() Time { return Now() }
func liveDate() Date { return tz.DateFromTime(fixtureNow()) }
func fixtureDate() Date { return tz.TodayDate() }
func List(value any) any { return value }
func TestLiveDateConversionHelper(t *testing.T) {
	t.Parallel()
	assertpkg.Equal(t, liveDate(), fixedDate)
}
func normalize(date Date) Date { return date }
func TestWrappedLiveDate(t *testing.T) {
	t.Parallel()
	date := normalize(TodayDate())
	Equal(t, date, fixedDate)
}
func TestDotImportedNow(t *testing.T) {
	t.Parallel()
	history := GetHistory(WorkSession{CheckInTime: Now()})
	_ = history.WeeklySummaries
}
`)
	findings, err := scanCalendarFixtureClockRisks(root)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(formatCalendarClockFindings(findings), "\n")
	if !strings.Contains(joined, "TestLiveMethod") || !strings.Contains(joined, "TestFactoryMethod") ||
		!strings.Contains(joined, "TestExplicitReceiverTypeConverges") || !strings.Contains(joined, "TestDotImportedNow") ||
		!strings.Contains(joined, "TestLiveDateConversionHelper") || !strings.Contains(joined, "TestAnonymousRange") ||
		!strings.Contains(joined, "TestLiveWeekdayFixture") || !strings.Contains(joined, "TestIndirectLiveWeekdayFixture") ||
		!strings.Contains(joined, "TestWeekdayFromLiveDateHelper") || !strings.Contains(joined, "TestWrappedLiveDate") || strings.Contains(joined, "TestFakeMethod") ||
		strings.Contains(joined, "TestRepeatedInterfaceAssignmentsUseLastConcreteClock") {
		t.Fatalf("receiver-qualified findings were %q", joined)
	}
}

func assertNoCalendarFindings(t *testing.T, root string) {
	t.Helper()

	findings, err := scanCalendarFixtureClockRisks(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("safe source triggered findings: %v", formatCalendarClockFindings(findings))
	}
}

func writeCalendarFixtureSource(t *testing.T, rel, source string) string {
	t.Helper()

	root := t.TempDir()
	writeCalendarFixtureSourceAt(t, root, rel, source)
	return root
}

func writeCalendarFixtureSourceAt(t *testing.T, root, rel, source string) {
	t.Helper()

	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
}

func requireCalendarFinding(t *testing.T, findings []calendarClockFinding, wants ...string) {
	t.Helper()

	joined := strings.Join(formatCalendarClockFindings(findings), "\n")
	for _, want := range wants {
		if !strings.Contains(joined, want) {
			t.Errorf("findings %q do not contain %q", joined, want)
		}
	}
}
