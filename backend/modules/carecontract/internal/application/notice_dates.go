package application

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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

// monthEnd is the last day of the month of day.
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
// #4682 (informedAt, set at the conclusion): the period then runs 14 days
// from the conclusion (§ 355 Abs. 2 BGB). A contract concluded before
// without it has its period end 12 months and 14 days after the conclusion
// (§ 356 Abs. 3 BGB), except one signed on paper, which was signed at the
// office and is no distance contract (§ 312c BGB).
func withdrawalUntil(concluded *carecontract.Concluded, informedAt *time.Time) *calendar.Date {
	if concluded == nil {
		return nil
	}
	start := calendar.DateFromTime(concluded.At)
	if informedAt != nil {
		if informed := calendar.DateFromTime(*informedAt); informed.After(start) {
			start = informed
		}
		last := start.AddDays(carecontract.WithdrawalDays)
		return &last
	}
	if concluded.Method == carecontract.MethodPaper {
		return nil
	}
	last := addMonths(start, 12).AddDays(carecontract.WithdrawalDays)
	return &last
}
