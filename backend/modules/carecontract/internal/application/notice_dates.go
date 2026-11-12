package application

import (
	"time"

	"github.com/moto-ogs/project-phoenix/modules/carecontract"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The dates of a termination and a withdrawal (#4682). They are pure: the
// terms frozen in the contract version decide them, never today's template,
// because the parents agreed to the terms of their contract.

// addMonths moves a day by whole months and keeps it inside the target
// month: 31 January plus one month is 28 (or 29) February.
func addMonths(day calendar.Date, months int) calendar.Date {
	first := time.Date(day.Year(), day.Month()+time.Month(months), 1, 0, 0, 0, 0, time.UTC)
	last := first.AddDate(0, 1, -1).Day()
	return calendar.NewDate(first.Year(), first.Month(), min(day.Day(), last))
}

func monthEnd(day calendar.Date) calendar.Date {
	return addMonths(calendar.NewDate(day.Year(), day.Month(), 1), 1).AddDays(-1)
}

// isNoticeDate tells whether a contract may end on day under noticeTo: every
// month end, the half years (31 January, 31 July) or the end of the school
// year (31 July).
func isNoticeDate(day calendar.Date, noticeTo string) bool {
	if day != monthEnd(day) {
		return false
	}
	switch noticeTo {
	case carecontract.NoticeHalfYear:
		return day.Month() == time.January || day.Month() == time.July
	case carecontract.NoticeSchoolYearEnd:
		return day.Month() == time.July
	default:
		return true
	}
}

// noticeEnd is the earliest day the contract may end on after a
// termination received on receivedOn: the first notice date that leaves the
// notice period. A contract that ends earlier on its own ends then.
func noticeEnd(receivedOn calendar.Date, noticeMonths int, noticeTo string, contractEnd *calendar.Date) calendar.Date {
	earliest := addMonths(receivedOn, noticeMonths)
	day := monthEnd(earliest)
	for !isNoticeDate(day, noticeTo) {
		day = monthEnd(day.AddDays(1))
	}
	if contractEnd != nil && contractEnd.Before(day) {
		return *contractEnd
	}
	return day
}

// noticeDates are the last days of the contract after an ordinary and, when
// the terms provide it, a special termination to the half year received on
// receivedOn. A special termination is offered only where it ends earlier.
func noticeDates(terms carecontract.TemplateInput, receivedOn calendar.Date, contractEnd *calendar.Date) (calendar.Date, *calendar.Date) {
	ordinary := noticeEnd(receivedOn, terms.NoticeMonths, terms.NoticeTo, contractEnd)
	if !terms.SpecialTermination {
		return ordinary, nil
	}
	special := noticeEnd(receivedOn, terms.NoticeMonths, carecontract.NoticeHalfYear, contractEnd)
	if !special.Before(ordinary) {
		return ordinary, nil
	}
	return ordinary, &special
}

// withdrawalUntil is the last day of the withdrawal period, or nil when the
// contract has none. Every contract PDF carries the withdrawal notice since
// #4682 (informedAt, proven delivery or parent download): the period runs 14
// days from the later of conclusion and notice provision (§ 355, § 356 BGB).
// Without that proof it ends 12 months and 14 days after the conclusion
// (§ 356 Abs. 3 BGB), and a late notice never reaches beyond that either. A
// contract signed on paper before has none: it was signed at the office and
// is no distance contract (§ 312c BGB). A last day on a Saturday, a Sunday or
// a statutory holiday of the school's federal state moves to the next
// working day (§ 193 BGB, #6350); holidays holds those of
// withdrawalWindow.
func withdrawalUntil(concluded *carecontract.Concluded, informedAt *time.Time, holidays map[calendar.Date]bool) *calendar.Date {
	if !hasWithdrawalPeriod(concluded, informedAt) {
		return nil
	}
	start := calendar.DateFromTime(concluded.At)
	expiry := nextWorkingDay(addMonths(start, 12).AddDays(carecontract.WithdrawalDays), holidays)
	if informedAt == nil {
		return &expiry
	}
	if informed := calendar.DateFromTime(*informedAt); informed.After(start) {
		start = informed
	}
	last := nextWorkingDay(start.AddDays(carecontract.WithdrawalDays), holidays)
	if last.After(expiry) {
		last = expiry
	}
	return &last
}

// hasWithdrawalPeriod tells whether withdrawalUntil names a day, so that a
// contract without a period reads no holidays.
func hasWithdrawalPeriod(concluded *carecontract.Concluded, informedAt *time.Time) bool {
	return concluded != nil && (informedAt != nil || concluded.Method != carecontract.MethodPaper)
}

// withdrawalWindow spans every day withdrawalUntil may move past for a
// contract concluded at concludedAt: from the conclusion to a week after
// the latest end, 12 months and 14 days later. No run of Saturday, Sunday
// and statutory holidays is longer than four days; a later end of a late
// notice is cut back to that latest end anyway.
func withdrawalWindow(concludedAt time.Time) (calendar.Date, calendar.Date) {
	start := calendar.DateFromTime(concludedAt)
	return start, addMonths(start, 12).AddDays(carecontract.WithdrawalDays + 7)
}

// nextWorkingDay is day, or the next day that is neither a Saturday, a
// Sunday nor one of holidays, the statutory holidays at the school (§ 193
// BGB). Only the end of a period to declare something moves: the end of a
// contract after a termination is no such period and never moves.
func nextWorkingDay(day calendar.Date, holidays map[calendar.Date]bool) calendar.Date {
	for day.Weekday() == time.Saturday || day.Weekday() == time.Sunday || holidays[day] {
		day = day.AddDays(1)
	}
	return day
}
