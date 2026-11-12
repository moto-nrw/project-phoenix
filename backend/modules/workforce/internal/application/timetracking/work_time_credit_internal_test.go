package timetracking

// Work time credit (#6851, #6852): pack-up minutes on a regular checkout and
// the rounding of a completed block's duration. Pure calculations first, then
// which close path stores which rule. int64 literals are fake in-memory IDs.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var creditDay = NewDate(2026, time.November, 9)

func creditAt(hour, minute int) time.Time {
	return time.Date(2026, time.November, 9, hour, minute, 0, 0, Berlin)
}

// creditBlock is a closed block of creditDay with the stored rule.
func creditBlock(fromH, fromM, toH, toM int, rule workforce.WorkSessionCreditRule) *WorkSession {
	checkOut := creditAt(toH, toM)
	session := &WorkSession{
		Model:   Model{ID: 70},
		StaffID: 100, Date: creditDay, Status: WorkSessionStatusPresent,
		CheckInTime: creditAt(fromH, fromM), CheckOutTime: &checkOut, CreatedBy: 100,
	}
	session.setCreditRule(rule)
	return session
}

var (
	mausRule    = workforce.WorkSessionCreditRule{PackUpMinutes: 5, RoundingIntervalMinutes: 15, RoundingMode: workforce.WorkTimeRoundingNearest}
	roundOnly15 = workforce.WorkSessionCreditRule{RoundingIntervalMinutes: 15, RoundingMode: workforce.WorkTimeRoundingNearest}
)

func TestRoundWorkMinutes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		minutes  int
		interval int
		mode     string
		want     int
	}{
		{"off keeps the duration", 67, 0, workforce.WorkTimeRoundingNearest, 67},
		{"nearest below half rounds down", 67, 15, workforce.WorkTimeRoundingNearest, 60},
		{"nearest above half rounds up", 68, 15, workforce.WorkTimeRoundingNearest, 75},
		{"nearest exactly half rounds up", 65, 10, workforce.WorkTimeRoundingNearest, 70},
		{"up rounds any rest up", 61, 15, workforce.WorkTimeRoundingUp, 75},
		{"down drops any rest", 74, 15, workforce.WorkTimeRoundingDown, 60},
		{"a multiple stays", 90, 15, workforce.WorkTimeRoundingUp, 90},
		{"a short block can round to zero", 7, 15, workforce.WorkTimeRoundingNearest, 0},
	} {
		assert.Equal(t, tc.want, roundWorkMinutes(tc.minutes, tc.interval, tc.mode), tc.name)
	}
}

// The order is stamped duration, plus pack-up time, rounding, minus breaks.
func TestSessionCredit_PackUpBeforeRoundingBeforeBreaks(t *testing.T) {
	t.Parallel()

	// 08:00-14:03 is 363 stamped minutes; +5 pack-up is 368; rounded to the
	// nearest 15 that is 375. Without the pack-up time it would be 360.
	session := creditBlock(8, 0, 14, 3, mausRule)
	session.BreakMinutes = 30

	assert.Equal(t, workTimeCredit{PackUpMinutes: 5, RoundingMinutes: 7}, sessionCredit(session))
	assert.Equal(t, 375-30, creditedNetMinutes(session, nil, creditAt(20, 0)))
	assert.Equal(t, 363-30, netMinutesWithBreaks(session, nil, creditAt(20, 0)), "stamped net time is unchanged")
}

func TestSessionCredit_RoundingDirections(t *testing.T) {
	t.Parallel()
	for mode, want := range map[string]int{
		workforce.WorkTimeRoundingNearest: 375 - 368,
		workforce.WorkTimeRoundingUp:      375 - 368,
		workforce.WorkTimeRoundingDown:    360 - 368,
	} {
		rule := mausRule
		rule.RoundingMode = mode
		credit := sessionCredit(creditBlock(8, 0, 14, 3, rule))
		assert.Equal(t, 5, credit.PackUpMinutes, mode)
		assert.Equal(t, want, credit.RoundingMinutes, mode)
	}
}

func TestSessionCredit_NoRuleNoOpenBlock(t *testing.T) {
	t.Parallel()

	assert.Zero(t, sessionCredit(creditBlock(8, 0, 14, 3, workforce.WorkSessionCreditRule{})),
		"a block closed without a rule (every block before #6851) keeps its stamped time")

	open := creditBlock(8, 0, 14, 3, mausRule)
	open.CheckOutTime = nil
	assert.Zero(t, sessionCredit(open), "a running block is credited nothing")
	assert.Equal(t, 120, creditedNetMinutes(open, nil, creditAt(10, 0)))
}

// An automatic checkout stores the rounding only: no pack-up minutes.
func TestSessionCredit_AutoCheckoutRoundsWithoutPackUp(t *testing.T) {
	t.Parallel()

	session := creditBlock(8, 0, 15, 53, roundOnly15) // 473 -> 480
	session.AutoCheckedOut = true
	assert.Equal(t, workTimeCredit{RoundingMinutes: 7}, sessionCredit(session))
}

// A correction moves the stamp; the credit is derived again from the new
// stamps and the stored rule, never added on top of an earlier credit.
func TestSessionCredit_CorrectionRecomputesWithoutDoubleCounting(t *testing.T) {
	t.Parallel()

	session := creditBlock(8, 0, 14, 3, mausRule)
	corrected := creditAt(14, 20) // 380 + 5 = 385 -> 390
	session.CheckOutTime = &corrected

	assert.Equal(t, workTimeCredit{PackUpMinutes: 5, RoundingMinutes: 5}, sessionCredit(session))
	assert.Equal(t, 390, creditedNetMinutes(session, nil, creditAt(20, 0)))
	assert.Equal(t, 390, creditedNetMinutes(session, nil, creditAt(21, 0)), "reading again changes nothing")
}

// Several blocks of a day each earn their own credit, booked on the day.
func TestCreditedNetMinutesByDate_SeveralBlocksPerDay(t *testing.T) {
	t.Parallel()

	morning := creditBlock(7, 0, 8, 2, mausRule)      // 62 + 5 = 67 -> 60
	afternoon := creditBlock(11, 30, 16, 0, mausRule) // 270 + 5 = 275 -> 270
	afternoon.ID = 71

	total := 0
	for _, session := range []*WorkSession{morning, afternoon} {
		for day, minutes := range creditedNetMinutesByDate(session, nil, *session.CheckOutTime, creditDay, creditDay) {
			require.Equal(t, creditDay, day)
			total += minutes
		}
	}
	assert.Equal(t, 60+270, total)
}

func TestCreditedNetMinutesByDate_CreditLandsOnTheCheckoutDay(t *testing.T) {
	t.Parallel()

	nextDay := creditDay.AddDays(1)
	checkOut := nextDay.BerlinMidnight().Add(32 * time.Minute) // 22:00-00:32 = 152 + 5 = 157 -> 150
	session := creditBlock(22, 0, 23, 0, mausRule)
	session.CheckOutTime = &checkOut

	byDate := creditedNetMinutesByDate(session, nil, checkOut, creditDay, nextDay)
	assert.Equal(t, 120, byDate[creditDay], "the evening keeps its stamped minutes")
	assert.Equal(t, 32-2, byDate[nextDay], "the credit is booked with the checkout")

	onlyFirst := creditedNetMinutesByDate(session, nil, checkOut, creditDay, creditDay)
	assert.Equal(t, map[Date]int{creditDay: 120}, onlyFirst, "a range without the checkout day carries no credit")
}

func TestCreditedNetMinutesByDate_LiveLimitIsNotACheckout(t *testing.T) {
	t.Parallel()

	session := creditBlock(8, 0, 14, 3, mausRule)
	byDate := creditedNetMinutesByDate(session, nil, creditAt(12, 0), creditDay, creditDay)
	assert.Equal(t, 240, byDate[creditDay])
}

func TestExportRowNotes_ShowsTheCredit(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "Frühdienst; Wegpackzeit +5 Min.; Rundung -8 Min.",
		exportRowNotes("Frühdienst", &SessionResponse{PackUpMinutes: 5, RoundingMinutes: -8}))
	assert.Equal(t, "Rundung +7 Min.", exportRowNotes("", &SessionResponse{RoundingMinutes: 7}))
	assert.Empty(t, exportRowNotes("", &SessionResponse{}))
}

func TestSessionHistoryResponses_CarryTheCredit(t *testing.T) {
	t.Parallel()

	session := creditBlock(8, 0, 14, 3, mausRule)
	responses := sessionHistoryResponses([]*WorkSession{session}, nil, nil, nil, creditAt(20, 0))
	require.Len(t, responses, 1)
	assert.Equal(t, 375, responses[0].NetMinutes)
	assert.Equal(t, 5, responses[0].PackUpMinutes)
	assert.Equal(t, 7, responses[0].RoundingMinutes)
}

// --- which close path stores which rule ---

func creditCheckoutService(t *testing.T, rule workforce.WorkSessionCreditRule) (*workSessionService, *wsMockWorkSessionRepository, *wsMockWorkSessionBreakRepository) {
	t.Helper()
	svc, sessionRepo, breakRepo, _, _ := wsCreateTestService()
	svc.settings = &wsMockSettingsResolver{credit: rule}
	open := &WorkSession{Model: Model{ID: 70}, StaffID: 100, Date: TodayDate(), CheckInTime: time.Now().Add(-4 * time.Hour)}
	sessionRepo.getCurrentByStaffIDFunc = func(context.Context, int64) (*WorkSession, error) { return open, nil }
	sessionRepo.findByIDFunc = func(_ context.Context, id any) (*WorkSession, error) {
		closed := *open
		now := time.Now()
		closed.CheckOutTime = &now
		return &closed, nil
	}
	breakRepo.getActiveBySessionIDFunc = func(context.Context, int64) (*WorkSessionBreak, error) { return nil, nil }
	breakRepo.createFunc = func(_ context.Context, entity *WorkSessionBreak) error {
		entity.ID = 80
		return nil
	}
	return svc, sessionRepo, breakRepo
}

func TestCheckOut_StoresPackUpAndRounding(t *testing.T) {
	t.Parallel()
	svc, sessionRepo, _ := creditCheckoutService(t, mausRule)

	_, err := svc.CheckOut(context.Background(), 100, "")
	require.NoError(t, err)
	assert.Equal(t, []workforce.WorkSessionCreditRule{mausRule}, sessionRepo.closedCredits)
}

func TestCheckOut_SettingsOffStoresNothing(t *testing.T) {
	t.Parallel()
	svc, sessionRepo, _ := creditCheckoutService(t, workforce.WorkSessionCreditRule{})

	_, err := svc.CheckOut(context.Background(), 100, "")
	require.NoError(t, err)
	assert.Equal(t, []workforce.WorkSessionCreditRule{{}}, sessionRepo.closedCredits)
}

func TestCheckOut_UnreadableSettingsCloseNothing(t *testing.T) {
	t.Parallel()
	svc, sessionRepo, _ := creditCheckoutService(t, mausRule)
	svc.settings = &wsMockSettingsResolver{creditErr: errors.New("settings down")}

	_, err := svc.CheckOut(context.Background(), 100, "")
	require.Error(t, err)
	assert.Empty(t, sessionRepo.closedCredits)
}

func TestStartBreak_IsNoCheckout(t *testing.T) {
	t.Parallel()
	svc, sessionRepo, _ := creditCheckoutService(t, mausRule)

	_, err := svc.StartBreak(context.Background(), 100, nil)
	require.NoError(t, err)
	assert.Empty(t, sessionRepo.closedCredits, "a break start closes no block and earns no pack-up time")
}

func TestAutoCheckout_StoresRoundingWithoutPackUp(t *testing.T) {
	t.Parallel()

	yesterday := TodayDate().AddDays(-1)
	session := &WorkSession{StaffID: 7, Date: yesterday, CheckInTime: yesterday.BerlinMidnight().Add(8 * time.Hour)}
	session.ID = 42
	service, sessionRepo, _, auditRepo, _ := autoCheckoutFixture([]*WorkSession{session}, []*TimeTrackingShift{shiftFor(7, yesterday, 8, 16)})
	service.settings = &wsMockSettingsResolver{credit: mausRule}
	auditRepo.createBatchFunc = func(context.Context, []*WorkSessionEdit) error { return nil }

	count, err := service.AutoCheckoutDueSessions(context.Background(), 15*time.Minute)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	assert.Equal(t, []workforce.WorkSessionCreditRule{roundOnly15}, sessionRepo.closedCredits)
}

func TestCreateSessionAsAdmin_StoresRoundingWithoutPackUp(t *testing.T) {
	t.Parallel()
	svc, sessionRepo, _, auditRepo, _ := wsCreateTestService()
	svc.settings = &wsMockSettingsResolver{credit: mausRule}
	var created *WorkSession
	sessionRepo.createFunc = func(_ context.Context, entity *WorkSession) error {
		entity.ID = 55
		created = entity
		return nil
	}
	auditRepo.createBatchFunc = func(context.Context, []*WorkSessionEdit) error { return nil }

	_, err := svc.CreateSessionAsAdmin(context.Background(), 10, 100, AdminCreateSessionRequest{
		Date: creditAt(0, 0), CheckInTime: creditAt(8, 0), CheckOutTime: creditAt(12, 0), Status: WorkSessionStatusPresent, Notes: "Nachtrag",
	})
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, roundOnly15, created.creditRule())
}

func creditAdminUpdate(t *testing.T, stored *WorkSession, settings *wsMockSettingsResolver, checkOut time.Time) *WorkSession {
	t.Helper()
	svc, sessionRepo, _, auditRepo, _ := wsCreateTestService()
	svc.settings = settings
	sessionRepo.findByIDFunc = func(context.Context, any) (*WorkSession, error) { return stored, nil }
	var updated *WorkSession
	sessionRepo.updateFunc = func(_ context.Context, entity *WorkSession) error {
		updated = entity
		return nil
	}
	auditRepo.createBatchFunc = func(context.Context, []*WorkSessionEdit) error { return nil }

	notes := "Korrektur"
	_, err := svc.UpdateSessionAsAdmin(context.Background(), 10, stored.StaffID, stored.ID,
		SessionUpdateRequest{CheckOutTime: &checkOut, Notes: &notes})
	require.NoError(t, err)
	require.NotNil(t, updated)
	return updated
}

func TestUpdateSession_CorrectionKeepsTheStoredRule(t *testing.T) {
	t.Parallel()
	// The school has switched both off since; the block keeps its rule.
	updated := creditAdminUpdate(t, creditBlock(8, 0, 14, 3, mausRule), &wsMockSettingsResolver{}, creditAt(14, 20))
	assert.Equal(t, mausRule, updated.creditRule())
}

func TestUpdateSession_ClosingAnOpenBlockRoundsWithoutPackUp(t *testing.T) {
	t.Parallel()
	open := creditBlock(8, 0, 14, 3, workforce.WorkSessionCreditRule{})
	open.CheckOutTime = nil
	updated := creditAdminUpdate(t, open, &wsMockSettingsResolver{credit: mausRule}, creditAt(12, 0))
	assert.Equal(t, roundOnly15, updated.creditRule())
}

func TestCreditedNetMinutesByDate_CarriesRoundingDeductionBeforeFiltering(t *testing.T) {
	t.Parallel()
	nextDay := creditDay.AddDays(1)
	end := nextDay.BerlinMidnight().Add(2 * time.Minute)
	session := creditBlock(23, 8, 23, 9, workforce.WorkSessionCreditRule{
		RoundingIntervalMinutes: 15, RoundingMode: workforce.WorkTimeRoundingDown,
	})
	session.CheckOutTime = &end
	assert.Equal(t, 45, creditedNetMinutes(session, nil, end))
	assert.Equal(t, map[Date]int{creditDay: 45}, creditedNetMinutesByDate(session, nil, end, creditDay, nextDay))
	assert.Equal(t, map[Date]int{creditDay: 45}, creditedNetMinutesByDate(session, nil, end, creditDay, creditDay))
	assert.Empty(t, creditedNetMinutesByDate(session, nil, end, nextDay, nextDay))
}

func TestCreditedNetMinutes_ExcessBreakDeductsCreditBeforeFloor(t *testing.T) {
	t.Parallel()
	for _, breakMinutes := range []int{62, 90} {
		session := creditBlock(23, 0, 23, 1, workforce.WorkSessionCreditRule{PackUpMinutes: 5})
		end := creditDay.AddDays(1).BerlinMidnight()
		session.CheckOutTime = &end
		session.BreakMinutes = breakMinutes
		want := max(0, 65-breakMinutes)
		assert.Equal(t, want, creditedNetMinutes(session, nil, end))
		byDate := creditedNetMinutesByDate(session, nil, end, creditDay, creditDay.AddDays(1))
		assert.Equal(t, want, byDate[creditDay])
		assert.Zero(t, byDate[creditDay.AddDays(1)])
	}
}

func TestBuildWeeklySummaries_StatutoryLimitUsesStampedMinutes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name            string
		minutes, packUp int
		over            bool
	}{
		{name: "rounding down cannot hide excess", minutes: 484, over: true},
		{name: "credit cannot create statutory excess", minutes: 480, packUp: 15, over: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			monday := creditDay.AddDays(-7)
			var sessions []*SessionResponse
			for offset := range 6 {
				day := monday.AddDays(offset)
				end := day.BerlinMidnight().Add(time.Duration(8*60+tc.minutes) * time.Minute)
				block := creditBlock(8, 0, 16, 0, workforce.WorkSessionCreditRule{
					PackUpMinutes: tc.packUp, RoundingIntervalMinutes: 15, RoundingMode: workforce.WorkTimeRoundingDown,
				})
				block.CheckInTime = day.BerlinMidnight().Add(8 * time.Hour)
				block.CheckOutTime = &end
				sessions = append(sessions, &SessionResponse{WorkSession: block})
			}
			svc := &workSessionService{}
			summaries := svc.buildWeeklySummaries(sessions, nil, monday, monday.AddDays(6))
			require.Len(t, summaries, 1)
			assert.Equal(t, 48*60+6*tc.packUp, summaries[0].TotalNetMinutes)
			assert.Equal(t, tc.over, summaries[0].IsOverWeeklyMax)
		})
	}
}
