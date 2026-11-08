package application

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The assessment of a Jahresbescheid (#6219, ADR 0177). It takes the
// months the monthly computation billed per contract (the drafts of each
// month, narrowed to the schools of the run) and the effective notice of
// each contract, and writes per contract a fee notice: one line per basis
// of calculation (module, reduction, cost bearer share) and span of months
// with one monthly amount, and one installment per month. It reads nothing
// and computes no price of its own: every cent comes from the month.

// noticeAssessment is what the notices of a run are assessed from.
type noticeAssessment struct {
	// from is the first month of the run; first notices start there.
	from calendar.Date
	// months are the months computed, ascending; a change notice may reach
	// back before from to the months of the notice it amends.
	months []calendar.Date
	// drafts are per month the drafts of the run's schools.
	drafts map[calendar.Date][]parentbilling.CareInvoice
	// previous is the effective notice of each contract of the run's schools.
	previous map[int64]parentbilling.CareNotice
	issuedOn calendar.Date
}

// noticeItem is one basis of calculation of one month: its key, its texts
// and what it adds to the month, negative for a reduction or a share.
type noticeItem struct {
	key, kind, label, detail string
	offering                 *int64
	cents                    int64
}

// assessNotices writes the drafts of the notices and counts the contracts
// whose effective notice still holds.
func assessNotices(in noticeAssessment) ([]parentbilling.CareNotice, int) {
	byContract, order := draftsByContract(in)
	notices := []parentbilling.CareNotice{}
	unchanged := 0
	for _, contractID := range order {
		previous, amends := in.previous[contractID]
		notice, changed := assessContract(in, byContract[contractID], previous, amends)
		switch {
		case changed:
			notices = append(notices, notice)
		case amends:
			unchanged++
		}
	}
	slices.SortFunc(notices, func(a, b parentbilling.CareNotice) int {
		return firstNonZero(strings.Compare(a.SchoolName, b.SchoolName), strings.Compare(a.ChildLastName, b.ChildLastName),
			strings.Compare(a.ChildFirstName, b.ChildFirstName), compareOrdered(a.ContractID, b.ContractID))
	})
	return notices, unchanged
}

// draftsByContract groups the drafts by contract and month; a month before
// the run counts only where the effective notice assessed it. Every
// contract with an effective notice has an entry, also without a draft.
func draftsByContract(in noticeAssessment) (map[int64]map[calendar.Date]parentbilling.CareInvoice, []int64) {
	byContract := map[int64]map[calendar.Date]parentbilling.CareInvoice{}
	order := []int64{}
	ensure := func(contractID int64) {
		if byContract[contractID] == nil {
			byContract[contractID] = map[calendar.Date]parentbilling.CareInvoice{}
			order = append(order, contractID)
		}
	}
	for _, month := range in.months {
		for _, draft := range in.drafts[month] {
			if month.Before(in.from) && !previousCovers(in.previous[draft.ContractID], month) {
				continue
			}
			ensure(draft.ContractID)
			byContract[draft.ContractID][month] = draft
		}
	}
	for contractID := range in.previous {
		ensure(contractID)
	}
	slices.Sort(order)
	return byContract, order
}

// previousCovers reports whether the notice assessed the month.
func previousCovers(notice parentbilling.CareNotice, month calendar.Date) bool {
	return slices.ContainsFunc(notice.Installments, func(installment parentbilling.CareNoticeInstallment) bool {
		return installment.Month == month
	})
}

// assessContract writes the notice of one contract; changed is false when
// nothing is to assess or the effective notice assessed the same.
func assessContract(in noticeAssessment, drafts map[calendar.Date]parentbilling.CareInvoice,
	previous parentbilling.CareNotice, amends bool,
) (parentbilling.CareNotice, bool) {
	previousMonths, previousAmounts := previousByMonth(previous, amends)
	months := noticeMonths(in.months, drafts, previousAmounts)
	if len(months) == 0 {
		return parentbilling.CareNotice{}, false
	}
	before := map[calendar.Date][]noticeItem{}
	after := map[calendar.Date][]noticeItem{}
	for _, month := range months {
		if invoice, ok := previousMonths[month]; ok {
			before[month] = noticeItems(invoice)
		}
		if invoice, ok := drafts[month]; ok {
			after[month] = noticeItems(invoice)
		}
	}
	if amends && sameAssessment(months, before, after, previousAmounts, drafts) {
		return parentbilling.CareNotice{}, false
	}
	notice := noticeHead(drafts, months, previous, amends)
	notice.Lines = noticeLines(months, before, after)
	for _, month := range months {
		installment := noticeInstallment(month, previousAmounts[month], drafts[month].TotalCents, amends, in.issuedOn)
		keepPreviousDue(&installment, previous)
		notice.Installments = append(notice.Installments, installment)
		notice.PreviousCents += installment.PreviousCents
		notice.TotalCents += installment.AmountCents
		notice.Months = append(notice.Months, noticeMonth(notice, drafts, month))
	}
	notice.DeltaCents = notice.TotalCents - notice.PreviousCents
	notice.PeriodFrom, notice.PeriodUntil = months[0], monthEnd(months[len(months)-1])
	notice.Year = months[0].Year()
	return notice, true
}

// previousByMonth is what the effective notice assessed per month: the
// month as billed and its rate. The months of a notice follow its
// installments one by one.
func previousByMonth(previous parentbilling.CareNotice, amends bool) (map[calendar.Date]parentbilling.CareInvoice, map[calendar.Date]int64) {
	months := map[calendar.Date]parentbilling.CareInvoice{}
	amounts := map[calendar.Date]int64{}
	if !amends {
		return months, amounts
	}
	for i, installment := range previous.Installments {
		amounts[installment.Month] = installment.AmountCents
		if i < len(previous.Months) {
			months[installment.Month] = previous.Months[i]
		}
	}
	return months, amounts
}

// noticeMonth is what the month of a notice bills: its draft, or nothing
// for a month the change takes back, which stays assessed, so no monthly
// run bills it either.
func noticeMonth(notice parentbilling.CareNotice, drafts map[calendar.Date]parentbilling.CareInvoice, month calendar.Date) parentbilling.CareInvoice {
	if draft, ok := drafts[month]; ok {
		return draft
	}
	return parentbilling.CareInvoice{ContractID: notice.ContractID, SchoolID: notice.SchoolID, Recipient: notice.Recipient,
		FamilyEmail: parentbilling.FamilyKey(notice.Recipient.Email), Service: parentbilling.ServiceCare,
		Positions: []parentbilling.CarePosition{}}
}

// noticeMonths are the months the notice assesses: those with a draft from
// the run's first month on (and before it where the amended notice reached),
// and every month the amended notice assessed, including zero amounts.
func noticeMonths(computed []calendar.Date, drafts map[calendar.Date]parentbilling.CareInvoice, previous map[calendar.Date]int64) []calendar.Date {
	months := []calendar.Date{}
	for _, month := range computed {
		_, drafted := drafts[month]
		_, assessed := previous[month]
		if drafted || assessed {
			months = append(months, month)
		}
	}
	return months
}

// sameAssessment reports whether the months bill what the effective notice
// assessed, basis by basis.
func sameAssessment(months []calendar.Date, before, after map[calendar.Date][]noticeItem,
	previous map[calendar.Date]int64, drafts map[calendar.Date]parentbilling.CareInvoice,
) bool {
	for _, month := range months {
		if previous[month] != drafts[month].TotalCents || !slices.Equal(itemCents(before[month]), itemCents(after[month])) {
			return false
		}
	}
	return true
}

func itemCents(items []noticeItem) []string {
	result := make([]string, len(items))
	for i, item := range items {
		result[i] = item.key + "=" + strconv.FormatInt(item.cents, 10)
	}
	slices.Sort(result)
	return result
}

// noticeHead copies child, school and recipient from the latest month the
// contract bills now, or from the amended notice when it bills nothing any
// more.
func noticeHead(drafts map[calendar.Date]parentbilling.CareInvoice, months []calendar.Date,
	previous parentbilling.CareNotice, amends bool,
) parentbilling.CareNotice {
	notice := parentbilling.CareNotice{Kind: parentbilling.NoticeKindInitial, Lines: []parentbilling.CareNoticeLine{},
		Installments: []parentbilling.CareNoticeInstallment{}, Months: []parentbilling.CareInvoice{}}
	if amends {
		notice.Kind = parentbilling.NoticeKindChange
		notice.Amends = &parentbilling.CareNoticeReference{ID: previous.ID, Number: previous.Number, IssuedOn: previous.IssuedOn}
		notice.ContractID, notice.SchoolID, notice.SchoolName = previous.ContractID, previous.SchoolID, previous.SchoolName
		notice.ChildFirstName, notice.ChildLastName, notice.StudentID = previous.ChildFirstName, previous.ChildLastName, previous.StudentID
		notice.Recipient, notice.FamilyEmail = previous.Recipient, previous.FamilyEmail
	}
	for i := len(months) - 1; i >= 0; i-- {
		draft, ok := drafts[months[i]]
		if !ok {
			continue
		}
		notice.ContractID, notice.SchoolID, notice.SchoolName = draft.ContractID, draft.SchoolID, draft.SchoolName
		notice.ChildFirstName, notice.ChildLastName, notice.StudentID = draft.ChildFirstName, draft.ChildLastName, draft.StudentID
		notice.Recipient, notice.FamilyEmail = draft.Recipient, draft.FamilyEmail
		break
	}
	return notice
}

// noticeItems are the bases of calculation of a month: each module position,
// each reduction and each cost bearer share.
func noticeItems(invoice parentbilling.CareInvoice) []noticeItem {
	items := []noticeItem{}
	for _, position := range invoice.Positions {
		offering := position.CareOfferingID
		items = append(items, noticeItem{key: noticeModuleKey(position), kind: parentbilling.NoticeLineModule,
			label: position.ModuleName, detail: moduleDetail(position), offering: &offering, cents: position.AmountCents})
	}
	for _, reduction := range invoice.Reductions {
		items = append(items, noticeItem{key: "r:" + reduction.Kind + ":" + strconv.FormatInt(reduction.RuleID, 10),
			kind: parentbilling.NoticeLineReduction, label: reduction.Name, cents: -reduction.AmountCents})
	}
	for _, share := range invoice.CostShares {
		items = append(items, noticeItem{key: "c:" + strconv.FormatInt(share.CostBearerID, 10),
			kind: parentbilling.NoticeLineCostShare, label: share.CostBearerName, cents: -share.AmountCents})
	}
	return items
}

// noticeModuleKey names the basis of a module: the module with its weekdays, or
// with its scale step.
func noticeModuleKey(position parentbilling.CarePosition) string {
	if position.DaysPerWeek != nil {
		return fmt.Sprintf("m:%d:d%d", position.CareOfferingID, *position.DaysPerWeek)
	}
	return fmt.Sprintf("m:%d:%s", position.CareOfferingID, strings.Join(positionWeekdays(position), ","))
}

func positionWeekdays(position parentbilling.CarePosition) []string {
	days := make([]string, len(position.Weekdays))
	for i, weekday := range position.Weekdays {
		days[i] = weekday.Weekday
	}
	slices.SortFunc(days, func(a, b string) int { return compareOrdered(weekdayOrder[a], weekdayOrder[b]) })
	return days
}

// moduleDetail is the scope of a module as the notice prints it, e.g.
// "an 1 Tag/Woche (Mi)".
func moduleDetail(position parentbilling.CarePosition) string {
	days := positionWeekdays(position)
	count := len(days)
	if position.DaysPerWeek != nil {
		count = *position.DaysPerWeek
	}
	unit := "Tagen"
	if count == 1 {
		unit = "Tag"
	}
	names := make([]string, len(days))
	for i, day := range days {
		names[i] = germanWeekdays[day]
	}
	detail := fmt.Sprintf("an %d %s/Woche", count, unit)
	if len(names) > 0 {
		detail += " (" + strings.Join(names, ", ") + ")"
	}
	return detail
}

// noticeLines groups the items of the months by basis: a line per basis and
// span of consecutive months with the same amount before and now.
func noticeLines(months []calendar.Date, before, after map[calendar.Date][]noticeItem) []parentbilling.CareNoticeLine {
	keys, texts := noticeKeys(months, before, after)
	lines := []parentbilling.CareNoticeLine{}
	for _, key := range keys {
		var open *parentbilling.CareNoticeLine
		var last calendar.Date
		for _, month := range months {
			was, now := itemCentsOf(before[month], key), itemCentsOf(after[month], key)
			if was == 0 && now == 0 {
				open = closeLine(&lines, open)
				continue
			}
			if open != nil && open.MonthlyPreviousCents == was && open.MonthlyCents == now && monthEnd(last).AddDays(1) == month {
				open.Months++
				open.PeriodUntil = monthEnd(month)
				last = month
				continue
			}
			closeLine(&lines, open)
			text := texts[key]
			open = &parentbilling.CareNoticeLine{Kind: text.kind, Label: text.label, Detail: text.detail, CareOfferingID: text.offering,
				PeriodFrom: month, PeriodUntil: monthEnd(month), Months: 1, Quantity: 1, MonthlyPreviousCents: was, MonthlyCents: now}
			last = month
		}
		closeLine(&lines, open)
	}
	return lines
}

// closeLine adds the open line with its annual amounts and closes it.
func closeLine(lines *[]parentbilling.CareNoticeLine, open *parentbilling.CareNoticeLine) *parentbilling.CareNoticeLine {
	if open == nil {
		return nil
	}
	months := int64(open.Months)
	open.AnnualPreviousCents, open.AnnualCents = open.MonthlyPreviousCents*months, open.MonthlyCents*months
	open.DeltaCents = open.AnnualCents - open.AnnualPreviousCents
	*lines = append(*lines, *open)
	return nil
}

// noticeKeys lists the bases in the order modules, reductions, shares, each
// module with all its bases and as it first appears, with the texts of its
// latest month.
func noticeKeys(months []calendar.Date, before, after map[calendar.Date][]noticeItem) ([]string, map[string]noticeItem) {
	rank := map[string]int{parentbilling.NoticeLineModule: 0, parentbilling.NoticeLineReduction: 1, parentbilling.NoticeLineCostShare: 2}
	keys := []string{}
	texts := map[string]noticeItem{}
	for _, side := range []map[calendar.Date][]noticeItem{before, after} {
		for _, month := range months {
			for _, item := range side[month] {
				if _, seen := texts[item.key]; !seen {
					keys = append(keys, item.key)
				}
				texts[item.key] = item
			}
		}
	}
	offering := func(key string) int64 {
		if texts[key].offering == nil {
			return 0
		}
		return *texts[key].offering
	}
	// A module keeps its lines together, in the order its modules appear.
	first := map[int64]int{}
	for i, key := range keys {
		if _, seen := first[offering(key)]; !seen {
			first[offering(key)] = i
		}
	}
	slices.SortStableFunc(keys, func(a, b string) int {
		return firstNonZero(compareOrdered(rank[texts[a].kind], rank[texts[b].kind]), compareOrdered(first[offering(a)], first[offering(b)]))
	})
	return keys, texts
}

func itemCentsOf(items []noticeItem, key string) int64 {
	var cents int64
	for _, item := range items {
		if item.key == key {
			cents += item.cents
		}
	}
	return cents
}

// keepPreviousDue keeps the due date of a past rate the change does not
// raise: what the amended notice set for that month stays set.
func keepPreviousDue(installment *parentbilling.CareNoticeInstallment, previous parentbilling.CareNotice) {
	if installment.DeltaCents > 0 || installment.ImmediatelyDue {
		return
	}
	for _, before := range previous.Installments {
		if before.Month == installment.Month && before.DueOn != installment.DueOn {
			installment.DueOn, installment.ImmediatelyDue = before.DueOn, before.ImmediatelyDue
		}
	}
}

// noticeInstallment is the rate of a month: due on its first day, or at
// once on the notice date when that day has passed. On a change notice only
// a raise of a past month is due at once; a past month the change does not
// raise kept its due date.
func noticeInstallment(month calendar.Date, previous, amount int64, amends bool, issuedOn calendar.Date) parentbilling.CareNoticeInstallment {
	installment := parentbilling.CareNoticeInstallment{Month: month, DueOn: month, PreviousCents: previous, AmountCents: amount,
		DeltaCents: amount - previous}
	if month.Before(issuedOn) && (installment.DeltaCents > 0 || (!amends && amount > 0)) {
		installment.DueOn, installment.ImmediatelyDue = issuedOn, true
	}
	return installment
}
