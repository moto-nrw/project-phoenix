package application

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/modules/documentrendering/templates"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The runs of the Jahresbescheide (#6219, ADR 0177). A run assesses the
// months of a year from its first month to December: it computes every month
// as the monthly run would (careMonthInput, buildCareMonth, narrowMonth),
// so prices, rule chain, family counts and cost bearer shares are the same,
// and writes per contract a notice of the months (care_notice_assessment.go).
// Like the monthly run it stores nothing before the release, shows the
// preview of today on every read, and its release checks count, total and
// fingerprint again under the lock of the monthly billing.

// Trail action of the letter of a notice.
const runEventLetter = "letter"

// CareNoticeStore persists the notice runs and their notices.
type CareNoticeStore interface {
	ListCareNoticeRuns(ctx context.Context, organizationID int64) ([]parentbilling.CareNoticeRun, error)
	// InsertCareNoticeRun reports a second calculated run of a year as
	// ErrCareNoticeRunOpen.
	InsertCareNoticeRun(ctx context.Context, organizationID, accountID int64, run parentbilling.CareNoticeRun) (parentbilling.CareNoticeRun, error)
	CareNoticeRun(ctx context.Context, organizationID, runID int64, lock bool) (parentbilling.CareNoticeRun, error)
	InsertCareNoticeRunEvent(ctx context.Context, organizationID, runID, accountID int64, action string, details map[string]any) error
	// EffectiveCareNotices reads the notices of the year nobody amended,
	// with their installments and months in the order of the months.
	EffectiveCareNotices(ctx context.Context, organizationID int64, year int) ([]parentbilling.CareNotice, error)
	// CareNoticeRunNotices reads the issued notices of a run with what
	// amends them.
	CareNoticeRunNotices(ctx context.Context, organizationID, runID int64) ([]parentbilling.CareNotice, error)
	// IssueCareNotices stores the numbered notices with their months; a
	// number an invoice or notice of the Träger carries is
	// ErrInvoiceNumberTaken.
	IssueCareNotices(ctx context.Context, organizationID int64, run parentbilling.CareNoticeRun, numberYear int, first int64, notices []parentbilling.CareNotice) error
	MarkCareNoticeRunReleased(ctx context.Context, organizationID, runID int64, issuedOn calendar.Date, count int, totalCents int64) (parentbilling.CareNoticeRun, error)
}

// CareFeeNotices serves parentbilling.CareFeeNotices over the monthly
// computation of CareBilling.
type CareFeeNotices struct {
	billing     *CareBilling
	store       CareNoticeStore
	letterheads parentbilling.BillingLetterheads
}

var _ parentbilling.CareFeeNotices = (*CareFeeNotices)(nil)

// NewCareFeeNotices builds the Jahresbescheide over the monthly billing,
// their store and the letterhead of the Träger, which the root binds.
func NewCareFeeNotices(billing *CareBilling, store CareNoticeStore, letterheads parentbilling.BillingLetterheads) *CareFeeNotices {
	if billing == nil || store == nil || letterheads == nil {
		panic("parent billing: the monthly billing, the notice store and the letterheads are required")
	}
	return &CareFeeNotices{billing: billing, store: store, letterheads: letterheads}
}

// ListCareNoticeRuns lists the runs, the latest year first, and suggests the
// current year from the current month on. It carries no personal data.
func (n *CareFeeNotices) ListCareNoticeRuns(ctx context.Context, organizationID int64) (parentbilling.CareNoticeRunList, error) {
	if organizationID <= 0 {
		return parentbilling.CareNoticeRunList{}, errTraegerRequired
	}
	runs, err := n.store.ListCareNoticeRuns(ctx, organizationID)
	if err != nil {
		return parentbilling.CareNoticeRunList{}, fmt.Errorf("parent billing: list notice runs: %w", err)
	}
	today := n.billing.today()
	return parentbilling.CareNoticeRunList{Runs: runs, SuggestedYear: today.Year(), SuggestedMonth: firstOfMonth(today)}, nil
}

// StartCareNoticeRun starts a calculated run of a year from its first month
// on for its schools; a year has at most one calculated run.
func (n *CareFeeNotices) StartCareNoticeRun(ctx context.Context, reviewer parentbilling.HolidayReviewer, start parentbilling.CareNoticeRunStart) (parentbilling.CareNoticeRun, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.CareNoticeRun{}, err
	}
	if start.Year < 2000 || start.Year > 2100 {
		return parentbilling.CareNoticeRun{}, validation.Errors{"year": errors.New("must be a year between 2000 and 2100")}
	}
	if !isMonthStart(start.FromMonth) || start.FromMonth.Year() != start.Year {
		return parentbilling.CareNoticeRun{}, validation.Errors{"from_month": errors.New("must be the first day of a month of the year")}
	}
	if err := checkNoticeDate(start.Year, start.NoticeDate); err != nil {
		return parentbilling.CareNoticeRun{}, err
	}
	schools, err := n.noticeSchools(ctx, reviewer.OrganizationID, start.SchoolIDs)
	if err != nil {
		return parentbilling.CareNoticeRun{}, err
	}
	run, err := n.store.InsertCareNoticeRun(ctx, reviewer.OrganizationID, reviewer.AccountID, parentbilling.CareNoticeRun{
		Year: start.Year, PeriodFrom: start.FromMonth, PeriodUntil: calendar.NewDate(start.Year, 12, 31),
		SchoolIDs: schools, NoticeDate: start.NoticeDate,
	})
	if err != nil {
		return parentbilling.CareNoticeRun{}, err
	}
	if err := n.store.InsertCareNoticeRunEvent(ctx, reviewer.OrganizationID, run.ID, reviewer.AccountID, runEventCreate,
		map[string]any{"school_ids": run.SchoolIDs}); err != nil {
		return parentbilling.CareNoticeRun{}, fmt.Errorf("parent billing: record notice run: %w", err)
	}
	return run, nil
}

// checkNoticeDate allows a notice date from December of the year before to
// the end of the year.
func checkNoticeDate(year int, noticeDate *calendar.Date) error {
	if noticeDate == nil {
		return nil
	}
	earliest, latest := calendar.NewDate(year-1, 12, 1), calendar.NewDate(year, 12, 31)
	if noticeDate.IsZero() || noticeDate.Before(earliest) || noticeDate.After(latest) {
		return validation.Errors{"notice_date": fmt.Errorf("must lie between %s and %s", earliest, latest)}
	}
	return nil
}

// noticeSchools resolves the schools of a run: the given ones, each a school
// of the Träger, or every school of the Träger.
func (n *CareFeeNotices) noticeSchools(ctx context.Context, organizationID int64, requested []int64) ([]int64, error) {
	schools, err := n.billing.schools.TraegerSchools(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("parent billing: read the schools: %w", err)
	}
	own := make([]int64, 0, len(schools))
	for _, school := range schools {
		own = append(own, school.ID)
	}
	if len(requested) == 0 {
		requested = own
	}
	result := []int64{}
	for _, id := range requested {
		if !slices.Contains(own, id) {
			return nil, validation.Errors{"school_ids": errors.New("names a school that is not one of the Träger")}
		}
		if !slices.Contains(result, id) {
			result = append(result, id)
		}
	}
	if len(result) == 0 {
		return nil, validation.Errors{"school_ids": errors.New("the Träger has no school")}
	}
	slices.Sort(result)
	return result, nil
}

// CareNoticeRun shows a run with the preview of today or its issued
// notices and records the read with the contracts shown.
func (n *CareFeeNotices) CareNoticeRun(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID int64) (parentbilling.CareNoticeRunDetail, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.CareNoticeRunDetail{}, err
	}
	if runID <= 0 {
		return parentbilling.CareNoticeRunDetail{}, parentbilling.ErrCareNoticeRunNotFound
	}
	run, err := n.store.CareNoticeRun(ctx, reviewer.OrganizationID, runID, false)
	if err != nil {
		return parentbilling.CareNoticeRunDetail{}, err
	}
	detail, err := n.detail(ctx, reviewer.OrganizationID, run)
	if err != nil {
		return parentbilling.CareNoticeRunDetail{}, err
	}
	if err := n.store.InsertCareNoticeRunEvent(ctx, reviewer.OrganizationID, run.ID, reviewer.AccountID, runEventRead,
		map[string]any{"contract_ids": noticeContracts(detail.Notices)}); err != nil {
		return parentbilling.CareNoticeRunDetail{}, fmt.Errorf("parent billing: record notice run read: %w", err)
	}
	return detail, nil
}

// ReleaseCareNoticeRun issues the notices of the preview, numbered from the
// number range of the monthly care without a gap, and closes the run. It
// holds the monthly billing of the Träger like a monthly release, so no
// month is billed by both. The caller rolls back when this fails.
func (n *CareFeeNotices) ReleaseCareNoticeRun(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID int64, release parentbilling.CareRelease) (parentbilling.CareNoticeRunDetail, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.CareNoticeRunDetail{}, err
	}
	if runID <= 0 {
		return parentbilling.CareNoticeRunDetail{}, parentbilling.ErrCareNoticeRunNotFound
	}
	if err := n.billing.store.LockCareBilling(ctx, reviewer.OrganizationID); err != nil {
		return parentbilling.CareNoticeRunDetail{}, fmt.Errorf("parent billing: hold the monthly billing: %w", err)
	}
	run, err := n.store.CareNoticeRun(ctx, reviewer.OrganizationID, runID, true)
	if err != nil {
		return parentbilling.CareNoticeRunDetail{}, err
	}
	if run.Status != parentbilling.RunCalculated {
		return parentbilling.CareNoticeRunDetail{}, parentbilling.ErrBillingRunReleased
	}
	preview, err := n.preview(ctx, reviewer.OrganizationID, run)
	if err != nil {
		return parentbilling.CareNoticeRunDetail{}, err
	}
	if len(preview.notices) != release.DocumentCount || preview.total() != release.TotalCents ||
		preview.fingerprint == "" || preview.fingerprint != release.Fingerprint {
		return parentbilling.CareNoticeRunDetail{}, parentbilling.ErrBillingRunChanged
	}
	// A Bescheid without its legal remedy is not issued (ADR 0170 point 4).
	if len(preview.notices) > 0 && preview.legalRemedyMissing {
		return parentbilling.CareNoticeRunDetail{}, parentbilling.ErrLegalRemedyMissing
	}
	issuedOn := preview.issuedOn
	if err := n.issue(ctx, reviewer.OrganizationID, run, issuedOn, preview.notices); err != nil {
		return parentbilling.CareNoticeRunDetail{}, err
	}
	released, err := n.store.MarkCareNoticeRunReleased(ctx, reviewer.OrganizationID, run.ID, issuedOn, len(preview.notices), preview.total())
	if err != nil {
		return parentbilling.CareNoticeRunDetail{}, err
	}
	if err := n.store.InsertCareNoticeRunEvent(ctx, reviewer.OrganizationID, run.ID, reviewer.AccountID, runEventRelease,
		map[string]any{"notice_count": len(preview.notices), "total_cents": preview.total(), "issued_on": issuedOn.String(),
			"contract_ids": noticeContracts(preview.notices)}); err != nil {
		return parentbilling.CareNoticeRunDetail{}, fmt.Errorf("parent billing: record notice release: %w", err)
	}
	return n.detail(ctx, reviewer.OrganizationID, released)
}

// issue numbers the notices from the counter of the monthly care and the
// year of the notice date and stores them.
func (n *CareFeeNotices) issue(ctx context.Context, organizationID int64, run parentbilling.CareNoticeRun, issuedOn calendar.Date,
	notices []parentbilling.CareNotice,
) error {
	if len(notices) == 0 {
		return nil
	}
	pattern, err := numberPatternOf(ctx, n.billing.store, organizationID, parentbilling.ServiceCare)
	if err != nil {
		return err
	}
	year := numberYear(pattern, issuedOn.Year())
	first, err := n.billing.store.ReserveNumbers(ctx, organizationID, parentbilling.ServiceCare, year, len(notices))
	if err != nil {
		return fmt.Errorf("parent billing: reserve notice numbers: %w", err)
	}
	for i := range notices {
		notices[i].Number = formatInvoiceNumber(pattern, issuedOn.Year(), first+int64(i))
		notices[i].IssuedOn = &issuedOn
	}
	return n.store.IssueCareNotices(ctx, organizationID, run, year, first, notices)
}

// CareNoticeLetter shows the notice of a contract in a run with the
// letterhead of the Träger: the draft of a calculated run or the issued
// notice of a released one. The read is recorded with the contract.
func (n *CareFeeNotices) CareNoticeLetter(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID, contractID int64) (parentbilling.CareNoticeLetter, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.CareNoticeLetter{}, err
	}
	if runID <= 0 {
		return parentbilling.CareNoticeLetter{}, parentbilling.ErrCareNoticeRunNotFound
	}
	run, err := n.store.CareNoticeRun(ctx, reviewer.OrganizationID, runID, false)
	if err != nil {
		return parentbilling.CareNoticeLetter{}, err
	}
	detail, err := n.detail(ctx, reviewer.OrganizationID, run)
	if err != nil {
		return parentbilling.CareNoticeLetter{}, err
	}
	index := slices.IndexFunc(detail.Notices, func(notice parentbilling.CareNotice) bool { return notice.ContractID == contractID })
	if index < 0 {
		return parentbilling.CareNoticeLetter{}, parentbilling.ErrCareNoticeNotFound
	}
	letterhead, err := n.letterheads.Letterhead(ctx, reviewer)
	if err != nil {
		return parentbilling.CareNoticeLetter{}, fmt.Errorf("parent billing: read the letterhead: %w", err)
	}
	if err := n.store.InsertCareNoticeRunEvent(ctx, reviewer.OrganizationID, run.ID, reviewer.AccountID, runEventLetter,
		map[string]any{"contract_ids": []int64{contractID}}); err != nil {
		return parentbilling.CareNoticeLetter{}, fmt.Errorf("parent billing: record notice letter: %w", err)
	}
	notice := detail.Notices[index]
	notice.LegalRemedy = filledLegalRemedy(notice, letterhead)
	return parentbilling.CareNoticeLetter{Notice: notice, SenderText: letterhead.SenderText,
		FooterText: letterhead.FooterText, ContactName: letterhead.ContactName, ContactPhone: letterhead.ContactPhone,
		ContactEmail: letterhead.ContactEmail, AccentColor: letterhead.AccentColor, LogoDataURL: logoDataURL(letterhead.Logo),
		BankAccount: noticeBankAccount(letterhead.BankAccount)}, nil
}

func noticeBankAccount(account *parentbilling.LetterheadBankAccount) *parentbilling.CareNoticeBankAccount {
	if account == nil {
		return nil
	}
	return &parentbilling.CareNoticeBankAccount{AccountHolder: account.AccountHolder, IBAN: account.IBAN, BIC: account.BIC,
		BankName: account.BankName}
}

// filledLegalRemedy fills the placeholders of the legal remedy the notice
// copied (ADR 0170) with the values its letter carries, as the letters of the
// monthly care do; a draft without a number keeps "{{bescheidnummer}}" empty.
func filledLegalRemedy(notice parentbilling.CareNotice, letterhead parentbilling.Letterhead) string {
	if strings.TrimSpace(notice.LegalRemedy) == "" {
		return ""
	}
	text := func(value string) templates.Value { return templates.Value{Text: strings.TrimSpace(value)} }
	issued := ""
	if notice.IssuedOn != nil {
		issued = germanDate(*notice.IssuedOn)
	}
	values := map[string]templates.Value{
		"familienname":   text(notice.Recipient.LastName),
		"kinder":         text(fullName(notice.ChildFirstName, notice.ChildLastName)),
		"traeger":        text(firstLine(textLines(letterhead.SenderText))),
		"ansprechperson": text(letterhead.ContactName),
		"zeitraum":       text(germanDate(notice.PeriodFrom) + " bis " + germanDate(notice.PeriodUntil)),
		"bescheidnummer": text(notice.Number),
		"datum":          text(issued),
		"summe":          text(formatEuro(notice.TotalCents)),
	}
	filled := templates.Fill(templates.Template{LegalRemedy: notice.LegalRemedy}, values)
	paragraphs := make([]string, 0, len(filled.LegalRemedy))
	for _, block := range filled.LegalRemedy {
		if line := strings.TrimSpace(strings.Join([]string{block.Heading, block.Text}, "\n")); line != "" {
			paragraphs = append(paragraphs, line)
		}
	}
	return strings.Join(paragraphs, "\n\n")
}

// logoDataURL inlines a logo image; anything that is no image stays out.
func logoDataURL(logo []byte) string {
	if len(logo) == 0 {
		return ""
	}
	mime := http.DetectContentType(logo)
	switch mime {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
		return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(logo)
	}
	return ""
}

// detail is the run with its preview or its issued notices and the number
// range the release uses.
func (n *CareFeeNotices) detail(ctx context.Context, organizationID int64, run parentbilling.CareNoticeRun) (parentbilling.CareNoticeRunDetail, error) {
	detail := parentbilling.CareNoticeRunDetail{CareNoticeRun: run, Notices: []parentbilling.CareNotice{},
		Unpriced: []parentbilling.CareUnpriced{}, Duplicates: []parentbilling.CareDuplicate{}}
	if run.Status == parentbilling.RunReleased {
		notices, err := n.store.CareNoticeRunNotices(ctx, organizationID, run.ID)
		if err != nil {
			return parentbilling.CareNoticeRunDetail{}, err
		}
		detail.Notices = notices
	} else {
		preview, err := n.preview(ctx, organizationID, run)
		if err != nil {
			return parentbilling.CareNoticeRunDetail{}, err
		}
		detail.Notices, detail.Unchanged, detail.Billed = preview.notices, preview.unchanged, preview.billed
		detail.Unpriced, detail.Duplicates = preview.unpriced, preview.duplicates
		detail.Fingerprint, detail.LegalRemedyMissing = preview.fingerprint, preview.legalRemedyMissing
	}
	for _, notice := range detail.Notices {
		detail.PreviewCount++
		detail.PreviewCents += notice.TotalCents
		detail.PreviewDeltaCents += notice.DeltaCents
	}
	pattern, err := numberPatternOf(ctx, n.billing.store, organizationID, parentbilling.ServiceCare)
	switch {
	case errors.Is(err, parentbilling.ErrNumberRangeMissing):
	case err != nil:
		return parentbilling.CareNoticeRunDetail{}, err
	default:
		detail.NumberPattern, detail.NumberRangeSet = pattern, true
	}
	return detail, nil
}

func noticeContracts(notices []parentbilling.CareNotice) []int64 {
	ids := make([]int64, len(notices))
	for i, notice := range notices {
		ids[i] = notice.ContractID
	}
	return ids
}

// legalRemedyOf reads the legal remedy of the template of the fee notice:
// the text the notices print, the key the fingerprint covers and whether it
// is missing (ADR 0170).
func (n *CareFeeNotices) legalRemedyOf(ctx context.Context, organizationID int64) (string, string, bool, error) {
	template, err := n.billing.kinds.templates.DocumentTemplate(ctx, organizationID, parentbilling.DocumentKindFeeNotice)
	if err != nil {
		return "", "", false, fmt.Errorf("parent billing: read the template of the fee notice: %w", err)
	}
	missing := templates.LacksLegalRemedy(templates.KindFeeNotice, letterTemplate(template, templates.KindFeeNotice))
	return template.LegalRemedy, fmt.Sprintf("%d:%s", template.Version, template.LegalRemedy), missing, nil
}
