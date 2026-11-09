package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Review of the holiday registrations by the Träger-Büro (#4663, ADR 0114).
// The caller runs every operation in the Träger transaction of the
// reviewer's Träger. A registration is frozen after the registration
// deadline of its period unless the Träger-Büro reopened it, and at once when
// the Träger-Büro froze it. A frozen registration can still be confirmed;
// cancelling and rebooking need it reopened. Every read of personal data and
// every change is written to the audit trail in the same transaction.

// Values of enrollment.holiday_registrations.freeze_override.
const (
	holidayFreezeFrozen   = "frozen"
	holidayFreezeReopened = "reopened"
)

// HolidayReviewStore persists the review of the registrations of a Träger.
type HolidayReviewStore interface {
	// ReadHolidayReview reads the registrations of a period, or the one
	// registrationID names, with children and bookings; a booking names
	// only its week and time model. It reads no health information.
	// The second result is the freeze override of each registration.
	ReadHolidayReview(ctx context.Context, organizationID, periodID, registrationID int64) ([]enrollment.HolidayRegistrationDetail, map[int64]string, error)
	// ReadHolidaySealedHealth reads the sealed health information of one
	// registration of the period; found is false outside it (#4722).
	ReadHolidaySealedHealth(ctx context.Context, organizationID, periodID, registrationID int64) (enrollment.HolidaySealedRegistrationHealth, bool, error)
	ReadHolidayContacts(ctx context.Context, organizationID, registrationID int64) ([]enrollment.HolidayRegistrationContact, error)
	// ReadHolidayAnswers reads the form version and the answers of the
	// registrations of a period, or of the one registrationID names, by
	// registration (#5198).
	ReadHolidayAnswers(ctx context.Context, organizationID, periodID, registrationID int64) (map[int64]enrollment.HolidayRegistrationAnswers, error)
	ReadHolidayHistory(ctx context.Context, organizationID, registrationID int64) ([]enrollment.HolidayRegistrationEvent, error)
	// ReadHolidayProofs reads the claimed proofs of a registration without
	// their bytes, oldest first (#5238).
	ReadHolidayProofs(ctx context.Context, organizationID, registrationID int64) ([]enrollment.HolidayRegistrationProof, error)
	// ReadHolidayProof reads one claimed proof of a registration of the
	// period with its bytes.
	ReadHolidayProof(ctx context.Context, organizationID, periodID, registrationID, proofID int64) (enrollment.HolidayProofFile, bool, error)
	// InsertClaimedHolidayProof stores a proof of the registration the
	// Träger-Büro uploads and returns its ID (#5627).
	InsertClaimedHolidayProof(ctx context.Context, organizationID, periodID, registrationID int64, proof enrollment.HolidayProofUpload) (int64, error)
	// LockHolidayRegistration reads the status, the freeze override and the
	// debtor number of a registration and holds its row.
	LockHolidayRegistration(ctx context.Context, organizationID, periodID, registrationID int64) (status, override, debtorNumber string, found bool, err error)
	SetHolidayRegistrationStatus(ctx context.Context, organizationID, registrationID int64, status string) error
	// SetHolidayDebtorNumber writes the debtor number of a registration;
	// an empty one stores none (#5621).
	SetHolidayDebtorNumber(ctx context.Context, organizationID, registrationID int64, debtorNumber string) error
	SetHolidayRegistrationFreeze(ctx context.Context, organizationID, registrationID int64, override string) error
	// ReplaceHolidayChildBookings makes bookings the child's bookings and
	// returns the weeks whose places changed: those it left and those it
	// newly books, which wait for their placement (#4699).
	ReplaceHolidayChildBookings(ctx context.Context, organizationID, periodID, childID int64, bookings []enrollment.HolidayBookingInput) ([]int64, error)
	// WriteHolidayRegistrationDetails writes the details of a locked
	// registration (#5629): both guardians with the address, the family's
	// answers, the contacts, and the details and answers of the changed
	// children only: details.Children, with childIDs and schoolChanged in
	// the same order. A school is written only when it changed, so a child
	// keeps a school the Träger has deleted since. The answers carry no
	// health information; the bookings, the consents and the sealed health
	// information stay as they are.
	WriteHolidayRegistrationDetails(ctx context.Context, organizationID, registrationID int64, details enrollment.HolidayRegistrationInput, childIDs []int64, schoolChanged []bool) error
	// AppendHolidayAudit writes one entry; details travel as JSON.
	AppendHolidayAudit(ctx context.Context, organizationID, periodID, registrationID int64, action string, details any, actorAccountID int64) error
}

// HolidayRegistrationReview serves enrollment.HolidayRegistrationReview.
type HolidayRegistrationReview struct {
	store     HolidayReviewStore
	periods   HolidayPeriodStore
	forms     HolidayFormStore
	health    enrollment.HolidayHealthCipher
	today     func() calendar.Date
	placement holidayPlacement
}

var _ enrollment.HolidayRegistrationReview = (*HolidayRegistrationReview)(nil)

// NewHolidayRegistrationReview builds the review; forms reads the form
// versions the answers belong to (#5198), health opens the health
// information (#4722). A nil today uses the Berlin calendar day.
func NewHolidayRegistrationReview(store HolidayReviewStore, periods HolidayPeriodStore, forms HolidayFormStore, health enrollment.HolidayHealthCipher, today func() calendar.Date) *HolidayRegistrationReview {
	if store == nil || periods == nil || forms == nil {
		panic("enrollment: the holiday review store, the period store and the form store are required")
	}
	if health == nil {
		panic(errHolidayHealthCipherRequired)
	}
	if today == nil {
		today = calendar.TodayDate
	}
	return &HolidayRegistrationReview{store: store, periods: periods, forms: forms, health: health, today: today}
}

// errHolidayPlacesRequired reports a cancellation or rebooking of a review
// built without the places: the waiting lists would not move up.
var errHolidayPlacesRequired = errors.New("enrollment: cancelling or rebooking a holiday registration needs the places")

// WithHolidayPlaces lets a cancellation and a rebooking give the places they
// free to the waiting lists (#4699); traeger and mail tell the parents of a
// booking that moved up. Without places the review refuses both.
func (h *HolidayRegistrationReview) WithHolidayPlaces(places HolidayPlaceStore, traeger enrollment.HolidayTraegerDirectory, mail enrollment.TraegerMailOutbox) *HolidayRegistrationReview {
	h.placement = holidayPlacement{store: places, traeger: traeger, mail: mail}
	return h
}

// moveUp gives the places of the weeks to their waiting lists after a
// change of the locked registration.
func (h *HolidayRegistrationReview) moveUp(ctx context.Context, reviewer enrollment.HolidayReviewer, period enrollment.HolidayPeriod, weekIDs []int64) ([]enrollment.HolidayPromotion, error) {
	if h.placement.store == nil {
		return nil, errHolidayPlacesRequired
	}
	_, promoted, err := h.placement.place(ctx, reviewer.OrganizationID, period, weekIDs, 0, reviewer.AccountID)
	return promoted, err
}

// ListHolidayRegistrations lists the registrations of the filter with the
// status counts of the whole period, and records the read. The list names
// no postal address (#5625).
func (h *HolidayRegistrationReview) ListHolidayRegistrations(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID int64, filter enrollment.HolidayRegistrationFilter) (enrollment.HolidayRegistrationList, error) {
	all, matching, err := h.listPeriod(ctx, reviewer, periodID, filter)
	if err != nil {
		return enrollment.HolidayRegistrationList{}, err
	}
	list := enrollment.HolidayRegistrationList{
		Registrations: make([]enrollment.HolidayRegistrationSummary, 0, len(matching)), Total: len(all), StatusCounts: map[string]int{},
	}
	for _, registration := range all {
		list.StatusCounts[registration.Status]++
	}
	for _, registration := range matching {
		list.Registrations = append(list.Registrations, registration.HolidayRegistrationSummary)
	}
	return list, nil
}

// ListHolidayRegistrationRecipients lists the registrations of the filter
// with the postal address of the parent who registered, for the invoices of
// the Elternabrechnung (#5625), and records the read like the list.
func (h *HolidayRegistrationReview) ListHolidayRegistrationRecipients(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID int64, filter enrollment.HolidayRegistrationFilter) ([]enrollment.HolidayRegistrationRecipient, error) {
	_, matching, err := h.listPeriod(ctx, reviewer, periodID, filter)
	if err != nil {
		return nil, err
	}
	recipients := make([]enrollment.HolidayRegistrationRecipient, len(matching))
	for i, registration := range matching {
		recipients[i] = enrollment.HolidayRegistrationRecipient{
			HolidayRegistrationSummary: registration.HolidayRegistrationSummary, GuardianAddress: registration.GuardianAddress,
			HasBenefitsProof: registration.BenefitsProofAttached,
		}
	}
	return recipients, nil
}

// listPeriod reads every registration of the period and those of the
// filter, and records the read in the trail.
func (h *HolidayRegistrationReview) listPeriod(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID int64, filter enrollment.HolidayRegistrationFilter) ([]enrollment.HolidayRegistrationDetail, []enrollment.HolidayRegistrationDetail, error) {
	period, all, err := h.readPeriod(ctx, reviewer, periodID, filter)
	if err != nil {
		return nil, nil, err
	}
	matching := []enrollment.HolidayRegistrationDetail{}
	for _, registration := range all {
		if holidayRegistrationMatches(registration.HolidayRegistrationSummary, filter) {
			matching = append(matching, registration)
		}
	}
	if err := h.audit(ctx, reviewer, period.ID, 0, enrollment.HolidayAuditList, map[string]any{
		"filter": holidayFilterDetails(filter), "registrations": len(matching),
	}); err != nil {
		return nil, nil, err
	}
	return all, matching, nil
}

// ExportHolidayRegistrations lists one row per booking of the filter with
// the answers to the questions without health information, and records the
// export.
func (h *HolidayRegistrationReview) ExportHolidayRegistrations(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID int64, filter enrollment.HolidayRegistrationFilter, format string) (enrollment.HolidayRegistrationExport, error) {
	period, all, err := h.readPeriod(ctx, reviewer, periodID, filter)
	if err != nil {
		return enrollment.HolidayRegistrationExport{}, err
	}
	rows := holidayExportRows(all, filter)
	columns, err := h.exportAnswers(ctx, reviewer.OrganizationID, period.ID, rows)
	if err != nil {
		return enrollment.HolidayRegistrationExport{}, err
	}
	if err := h.audit(ctx, reviewer, period.ID, 0, enrollment.HolidayAuditExport, map[string]any{
		"filter": holidayFilterDetails(filter), "format": format, "rows": len(rows),
	}); err != nil {
		return enrollment.HolidayRegistrationExport{}, err
	}
	return enrollment.HolidayRegistrationExport{Period: period, Rows: rows, Columns: columns}, nil
}

// exportAnswers reads the answers of the exported registrations and
// all form versions of the period, and gives each row the answers
// to the export's columns. No answer to a question with health information
// reaches a row.
func (h *HolidayRegistrationReview) exportAnswers(ctx context.Context, organizationID, periodID int64, rows []enrollment.HolidayRegistrationExportRow) ([]enrollment.HolidayFormField, error) {
	if len(rows) == 0 {
		return []enrollment.HolidayFormField{}, nil
	}
	answers, err := h.store.ReadHolidayAnswers(ctx, organizationID, periodID, 0)
	if err != nil {
		return nil, fmt.Errorf("enrollment: read holiday registration answers: %w", err)
	}
	versions := []int{}
	for _, row := range rows {
		if version := answers[row.Registration.ID].FormVersion; version > 0 && !slices.Contains(versions, version) {
			versions = append(versions, version)
		}
	}
	forms, err := h.forms.HolidayPeriodForms(ctx, organizationID, periodID, nil)
	if err != nil {
		return nil, fmt.Errorf("enrollment: read holiday form versions: %w", err)
	}
	columns := holidayExportColumns(forms, versions)
	for i := range rows {
		registration := answers[rows[i].Registration.ID]
		rows[i].Answers = holidayRowAnswers(columns, registration, rows[i].Child.ID)
		rows[i].AnswerFields = holidayExportAnswerFields(forms[registration.FormVersion], rows[i].Answers)
	}
	return columns, nil
}

// holidayExportAnswerFields keeps the submitted definitions for the answers
// already admitted by the export's health filter.
func holidayExportAnswerFields(form enrollment.HolidayPeriodForm, answers enrollment.HolidayAnswers) []enrollment.HolidayFormField {
	fields := []enrollment.HolidayFormField{}
	for _, field := range form.Fields {
		if _, ok := answers[field.Key]; ok {
			fields = append(fields, field)
		}
	}
	return fields
}

// holidayRowAnswers picks the family's and the child's answers to the
// export's columns.
func holidayRowAnswers(columns []enrollment.HolidayFormField, registration enrollment.HolidayRegistrationAnswers, childID int64) enrollment.HolidayAnswers {
	child := holidayChildCustomData(registration, childID)
	out := enrollment.HolidayAnswers{}
	for _, column := range columns {
		// Answers were checked against the registration's kept version and
		// belong to exactly one scope. A newer version may move the question
		// between family and child without changing those stored answers.
		value, ok := registration.CustomData[column.Key]
		if !ok {
			value, ok = child[column.Key]
		}
		if ok {
			out[column.Key] = value
		}
	}
	return out
}

func holidayChildCustomData(answers enrollment.HolidayRegistrationAnswers, childID int64) enrollment.HolidayAnswers {
	for _, child := range answers.Children {
		if child.ChildID == childID {
			return child.CustomData
		}
	}
	return nil
}

// HolidayRegistration returns one registration with its contacts and
// history, without its health information, and records the read.
func (h *HolidayRegistrationReview) HolidayRegistration(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID, registrationID int64) (enrollment.HolidayRegistrationDetail, error) {
	if err := holidayReviewKey(reviewer, periodID, registrationID); err != nil {
		return enrollment.HolidayRegistrationDetail{}, err
	}
	period, err := h.period(ctx, reviewer.OrganizationID, periodID, false)
	if err != nil {
		return enrollment.HolidayRegistrationDetail{}, err
	}
	detail, err := h.detail(ctx, reviewer, period, registrationID)
	if err != nil {
		return enrollment.HolidayRegistrationDetail{}, err
	}
	// The read is recorded once the registration is known to belong to the
	// period; the entry commits with the answer in the same transaction.
	if err := h.audit(ctx, reviewer, periodID, registrationID, enrollment.HolidayAuditRead, nil); err != nil {
		return enrollment.HolidayRegistrationDetail{}, err
	}
	return detail, nil
}

// OpenHolidayRegistrationProof returns one proof of benefits of a
// registration of the period (#5238) and records the read in the same
// transaction; without the entry no byte is answered.
func (h *HolidayRegistrationReview) OpenHolidayRegistrationProof(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID, registrationID, proofID int64) (enrollment.HolidayProofFile, error) {
	if err := holidayReviewKey(reviewer, periodID, registrationID); err != nil {
		return enrollment.HolidayProofFile{}, err
	}
	if proofID <= 0 {
		return enrollment.HolidayProofFile{}, enrollment.ErrHolidayProofNotFound
	}
	if _, err := h.period(ctx, reviewer.OrganizationID, periodID, false); err != nil {
		return enrollment.HolidayProofFile{}, err
	}
	file, found, err := h.store.ReadHolidayProof(ctx, reviewer.OrganizationID, periodID, registrationID, proofID)
	if err != nil {
		return enrollment.HolidayProofFile{}, fmt.Errorf("enrollment: read holiday registration proof: %w", err)
	}
	if !found {
		return enrollment.HolidayProofFile{}, enrollment.ErrHolidayProofNotFound
	}
	if err := h.audit(ctx, reviewer, periodID, registrationID, enrollment.HolidayAuditProof, map[string]any{"proof_id": proofID}); err != nil {
		return enrollment.HolidayProofFile{}, err
	}
	return file, nil
}

// ChangeHolidayRegistration cancels, freezes or reopens a registration.
// ConfirmHolidayRegistration confirms one (#5621).
func (h *HolidayRegistrationReview) ChangeHolidayRegistration(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID, registrationID int64, action string) (enrollment.HolidayRegistrationDetail, error) {
	var promoted []enrollment.HolidayPromotion
	cancel := func(ctx context.Context, _ int64, period enrollment.HolidayPeriod, current holidayRegistrationState) (any, error) {
		details, moved, err := h.cancel(ctx, reviewer, period, current)
		promoted = moved
		return details, err
	}
	apply := map[string]func(context.Context, int64, enrollment.HolidayPeriod, holidayRegistrationState) (any, error){
		enrollment.HolidayAuditCancel: cancel, enrollment.HolidayAuditFreeze: h.freeze, enrollment.HolidayAuditReopen: h.reopen,
	}[action]
	if apply == nil {
		return enrollment.HolidayRegistrationDetail{}, fmt.Errorf("enrollment: unknown holiday registration action %q", action)
	}
	detail, err := h.change(ctx, reviewer, periodID, registrationID, func(period enrollment.HolidayPeriod, current holidayRegistrationState) (string, any, error) {
		details, err := apply(ctx, reviewer.OrganizationID, period, current)
		return action, details, err
	})
	detail.Promotions = promoted
	return detail, err
}

// cancel cancels a registration that is not frozen and gives the places of
// its bookings to the waiting lists of their weeks (#4699).
func (h *HolidayRegistrationReview) cancel(ctx context.Context, reviewer enrollment.HolidayReviewer, period enrollment.HolidayPeriod, current holidayRegistrationState) (any, []enrollment.HolidayPromotion, error) {
	if err := h.requireChangeable(period, current); err != nil {
		return nil, nil, err
	}
	if err := h.store.SetHolidayRegistrationStatus(ctx, reviewer.OrganizationID, current.ID, enrollment.HolidayRegistrationCancelled); err != nil {
		return nil, nil, err
	}
	read, _, err := h.store.ReadHolidayReview(ctx, reviewer.OrganizationID, period.ID, current.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("enrollment: read holiday registration: %w", err)
	}
	promoted, err := h.moveUp(ctx, reviewer, period, holidayBookedWeeks(read))
	if err != nil {
		return nil, nil, err
	}
	return enrollment.HolidayRegistrationEventDetails{From: current.Status}, promoted, nil
}

// freeze freezes an open registration at once.
func (h *HolidayRegistrationReview) freeze(ctx context.Context, organizationID int64, period enrollment.HolidayPeriod, current holidayRegistrationState) (any, error) {
	if current.Status == enrollment.HolidayRegistrationCancelled || h.frozen(period, current.FreezeOverride) {
		return nil, enrollment.ErrHolidayRegistrationStateConflict
	}
	return nil, h.store.SetHolidayRegistrationFreeze(ctx, organizationID, current.ID, holidayFreezeFrozen)
}

// reopen opens a frozen registration again.
func (h *HolidayRegistrationReview) reopen(ctx context.Context, organizationID int64, period enrollment.HolidayPeriod, current holidayRegistrationState) (any, error) {
	if current.Status == enrollment.HolidayRegistrationCancelled || !h.frozen(period, current.FreezeOverride) {
		return nil, enrollment.ErrHolidayRegistrationStateConflict
	}
	return nil, h.store.SetHolidayRegistrationFreeze(ctx, organizationID, current.ID, holidayFreezeReopened)
}

// RebookHolidayChild replaces the bookings of one child of a registration
// that is not frozen. The history keeps the bookings before and after.
func (h *HolidayRegistrationReview) RebookHolidayChild(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID, registrationID, childID int64, bookings []enrollment.HolidayBookingInput) (enrollment.HolidayRegistrationDetail, error) {
	var promoted []enrollment.HolidayPromotion
	detail, err := h.change(ctx, reviewer, periodID, registrationID, func(period enrollment.HolidayPeriod, current holidayRegistrationState) (string, any, error) {
		details, changed, err := h.rebook(ctx, reviewer.OrganizationID, period, current, childID, bookings)
		if err != nil {
			return "", nil, err
		}
		// A week the child left frees its place, a new week waits for one;
		// the places are given before the trail records the rebooking.
		promoted, err = h.moveUp(ctx, reviewer, period, changed)
		return enrollment.HolidayAuditRebook, details, err
	})
	detail.Promotions = promoted
	return detail, err
}

func (h *HolidayRegistrationReview) rebook(ctx context.Context, organizationID int64, period enrollment.HolidayPeriod, current holidayRegistrationState, childID int64, bookings []enrollment.HolidayBookingInput) (any, []int64, error) {
	if err := h.requireChangeable(period, current); err != nil {
		return nil, nil, err
	}
	read, _, err := h.store.ReadHolidayReview(ctx, organizationID, period.ID, current.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("enrollment: read holiday registration: %w", err)
	}
	child, found := holidayChild(read, childID)
	if !found {
		return nil, nil, enrollment.ErrHolidayRegistrationEntryNotFound
	}
	before := make([]enrollment.HolidayBookingChange, len(child.Bookings))
	kept := make([]enrollment.HolidayBookingInput, len(child.Bookings))
	for i, booking := range child.Bookings {
		before[i] = enrollment.HolidayBookingChange{
			WeekID: booking.WeekID, TimeModelID: booking.TimeModelID, Waitlisted: booking.Placement == enrollment.HolidayPlacementWaitlisted,
		}
		kept[i] = enrollment.HolidayBookingInput{WeekID: booking.WeekID, TimeModelID: booking.TimeModelID}
	}
	if err := validateHolidayBookings(bookings, period, kept); err != nil {
		return nil, nil, validation.Errors{"bookings": err}
	}
	after := make([]enrollment.HolidayBookingChange, len(bookings))
	for i, booking := range bookings {
		after[i] = enrollment.HolidayBookingChange{WeekID: booking.WeekID, TimeModelID: booking.TimeModelID}
	}
	changed, err := h.store.ReplaceHolidayChildBookings(ctx, organizationID, period.ID, childID, bookings)
	if err != nil {
		return nil, nil, err
	}
	return enrollment.HolidayRegistrationEventDetails{ChildID: childID, Before: before, After: after}, changed, nil
}

// holidayChild finds a child among the read registrations.
func holidayChild(read []enrollment.HolidayRegistrationDetail, childID int64) (enrollment.HolidayRegistrationChild, bool) {
	for _, registration := range read {
		for _, child := range registration.Children {
			if child.ID == childID {
				return child, true
			}
		}
	}
	return enrollment.HolidayRegistrationChild{}, false
}

// change locks the period and the registration, runs apply and records the
// change it names; an empty action records nothing, for an edit that
// changed nothing (#5629). The period lock orders the change after a running
// change of the period's weeks, the registration lock one change of a
// registration after the other.
func (h *HolidayRegistrationReview) change(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID, registrationID int64,
	apply func(enrollment.HolidayPeriod, holidayRegistrationState) (string, any, error),
) (enrollment.HolidayRegistrationDetail, error) {
	if err := holidayReviewKey(reviewer, periodID, registrationID); err != nil {
		return enrollment.HolidayRegistrationDetail{}, err
	}
	period, err := h.period(ctx, reviewer.OrganizationID, periodID, true)
	if err != nil {
		return enrollment.HolidayRegistrationDetail{}, err
	}
	status, override, debtorNumber, found, err := h.store.LockHolidayRegistration(ctx, reviewer.OrganizationID, periodID, registrationID)
	if err != nil {
		return enrollment.HolidayRegistrationDetail{}, fmt.Errorf("enrollment: lock holiday registration: %w", err)
	}
	if !found {
		return enrollment.HolidayRegistrationDetail{}, enrollment.ErrHolidayRegistrationEntryNotFound
	}
	action, details, err := apply(period, holidayRegistrationState{
		ID: registrationID, Status: status, FreezeOverride: override, DebtorNumber: debtorNumber,
	})
	if err != nil {
		return enrollment.HolidayRegistrationDetail{}, err
	}
	if action != "" {
		if err := h.audit(ctx, reviewer, periodID, registrationID, action, details); err != nil {
			return enrollment.HolidayRegistrationDetail{}, err
		}
	}
	return h.detail(ctx, reviewer, period, registrationID)
}

// requireChangeable admits a cancellation or a rebooking: the registration
// is not cancelled and not frozen.
func (h *HolidayRegistrationReview) requireChangeable(period enrollment.HolidayPeriod, current holidayRegistrationState) error {
	if current.Status == enrollment.HolidayRegistrationCancelled {
		return enrollment.ErrHolidayRegistrationStateConflict
	}
	if h.frozen(period, current.FreezeOverride) {
		return enrollment.ErrHolidayRegistrationFrozen
	}
	return nil
}

// holidayRegistrationState is what a change decides on.
type holidayRegistrationState struct {
	ID             int64
	Status         string
	FreezeOverride string
	DebtorNumber   string
}

// frozen reports whether a registration is frozen: frozen on purpose, or
// after the registration deadline unless reopened.
func (h *HolidayRegistrationReview) frozen(period enrollment.HolidayPeriod, override string) bool {
	if override != "" {
		return override == holidayFreezeFrozen
	}
	return h.today().After(period.RegistrationClosesOn)
}

func (h *HolidayRegistrationReview) period(ctx context.Context, organizationID, periodID int64, lock bool) (enrollment.HolidayPeriod, error) {
	period, found, err := h.periods.FindHolidayPeriod(ctx, organizationID, periodID, lock)
	if err != nil {
		return enrollment.HolidayPeriod{}, fmt.Errorf("enrollment: read holiday period: %w", err)
	}
	if !found {
		return enrollment.HolidayPeriod{}, enrollment.ErrHolidayPeriodNotFound
	}
	return period, nil
}

// readPeriod checks the filter and reads every registration of the period
// without health information.
func (h *HolidayRegistrationReview) readPeriod(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID int64, filter enrollment.HolidayRegistrationFilter) (enrollment.HolidayPeriod, []enrollment.HolidayRegistrationDetail, error) {
	if err := holidayReviewKey(reviewer, periodID, 1); err != nil {
		return enrollment.HolidayPeriod{}, nil, err
	}
	if err := validateHolidayRegistrationFilter(filter); err != nil {
		return enrollment.HolidayPeriod{}, nil, err
	}
	period, err := h.period(ctx, reviewer.OrganizationID, periodID, false)
	if err != nil {
		return enrollment.HolidayPeriod{}, nil, err
	}
	read, overrides, err := h.store.ReadHolidayReview(ctx, reviewer.OrganizationID, periodID, 0)
	if err != nil {
		return enrollment.HolidayPeriod{}, nil, fmt.Errorf("enrollment: read holiday registrations: %w", err)
	}
	h.complete(period, read, overrides)
	return period, read, nil
}

func (h *HolidayRegistrationReview) detail(ctx context.Context, reviewer enrollment.HolidayReviewer, period enrollment.HolidayPeriod, registrationID int64) (enrollment.HolidayRegistrationDetail, error) {
	read, overrides, err := h.store.ReadHolidayReview(ctx, reviewer.OrganizationID, period.ID, registrationID)
	if err != nil {
		return enrollment.HolidayRegistrationDetail{}, fmt.Errorf("enrollment: read holiday registration: %w", err)
	}
	if len(read) == 0 {
		return enrollment.HolidayRegistrationDetail{}, enrollment.ErrHolidayRegistrationEntryNotFound
	}
	h.complete(period, read, overrides)
	detail := read[0]
	if texts, ok := enrollment.HolidayConsentTextsOf(detail.ConsentVersion); ok {
		detail.ConsentTexts = &texts
	}
	if detail.Contacts, err = h.store.ReadHolidayContacts(ctx, reviewer.OrganizationID, registrationID); err != nil {
		return enrollment.HolidayRegistrationDetail{}, fmt.Errorf("enrollment: read holiday registration contacts: %w", err)
	}
	if detail.History, err = h.store.ReadHolidayHistory(ctx, reviewer.OrganizationID, registrationID); err != nil {
		return enrollment.HolidayRegistrationDetail{}, fmt.Errorf("enrollment: read holiday registration history: %w", err)
	}
	if detail.BenefitsProofs, err = h.store.ReadHolidayProofs(ctx, reviewer.OrganizationID, registrationID); err != nil {
		return enrollment.HolidayRegistrationDetail{}, fmt.Errorf("enrollment: read holiday registration proofs: %w", err)
	}
	if err := h.detailAnswers(ctx, reviewer.OrganizationID, period.ID, &detail); err != nil {
		return enrollment.HolidayRegistrationDetail{}, err
	}
	return detail, nil
}

// detailAnswers adds the version of the form the registration was submitted
// with and every answer without health information; the health read adds
// those (#4722).
func (h *HolidayRegistrationReview) detailAnswers(ctx context.Context, organizationID, periodID int64, detail *enrollment.HolidayRegistrationDetail) error {
	answers, err := h.store.ReadHolidayAnswers(ctx, organizationID, periodID, detail.ID)
	if err != nil {
		return fmt.Errorf("enrollment: read holiday registration answers: %w", err)
	}
	registration, ok := answers[detail.ID]
	if !ok {
		return enrollment.ErrHolidayRegistrationEntryNotFound
	}
	forms, err := h.forms.HolidayPeriodForms(ctx, organizationID, periodID, []int{registration.FormVersion})
	if err != nil {
		return fmt.Errorf("enrollment: read holiday form version: %w", err)
	}
	form, ok := forms[registration.FormVersion]
	if !ok {
		return fmt.Errorf("enrollment: holiday registration %d keeps the missing form version %d", detail.ID, registration.FormVersion)
	}
	detail.Form = &form
	detail.LegalAcceptances = registration.LegalAcceptances
	detail.CustomData = registration.CustomData
	detail.ChildCustomData = registration.Children
	return nil
}

// holidayWeekModel is a time model with its week and its position in the
// period.
type holidayWeekModel struct {
	week  enrollment.HolidayWeek
	model enrollment.HolidayTimeModel
	order int
}

// complete names the week, host school and time model of every booking,
// orders the bookings as the period orders its weeks, and derives whether a
// registration is frozen. A cancelled registration is neither frozen nor a
// duplicate hint.
func (h *HolidayRegistrationReview) complete(period enrollment.HolidayPeriod, read []enrollment.HolidayRegistrationDetail, overrides map[int64]string) {
	models := map[int64]holidayWeekModel{}
	for _, week := range period.Weeks {
		for _, model := range week.TimeModels {
			models[model.ID] = holidayWeekModel{week: week, model: model, order: len(models)}
		}
	}
	for r := range read {
		registration := &read[r]
		cancelled := registration.Status == enrollment.HolidayRegistrationCancelled
		registration.Frozen = !cancelled && h.frozen(period, overrides[registration.ID])
		if registration.Children == nil {
			registration.Children = []enrollment.HolidayRegistrationChild{}
		}
		for c := range registration.Children {
			completeHolidayChild(&registration.Children[c], models, cancelled)
		}
	}
}

func completeHolidayChild(child *enrollment.HolidayRegistrationChild, models map[int64]holidayWeekModel, cancelled bool) {
	if cancelled {
		child.DuplicateRegistrationIDs = []int64{}
	}
	if child.Bookings == nil {
		child.Bookings = []enrollment.HolidayRegistrationBooking{}
	}
	for b := range child.Bookings {
		booking := &child.Bookings[b]
		known := models[booking.TimeModelID]
		booking.WeekStartsOn, booking.WeekEndsOn = known.week.StartsOn, known.week.EndsOn
		booking.HostSchoolID, booking.TimeModelName, booking.PriceCents = known.week.HostSchoolID, known.model.Name, known.model.PriceCents
	}
	slices.SortFunc(child.Bookings, func(a, b enrollment.HolidayRegistrationBooking) int {
		return models[a.TimeModelID].order - models[b.TimeModelID].order
	})
}

func (h *HolidayRegistrationReview) audit(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID, registrationID int64, action string, details any) error {
	if err := h.store.AppendHolidayAudit(ctx, reviewer.OrganizationID, periodID, registrationID, action, details, reviewer.AccountID); err != nil {
		return fmt.Errorf("enrollment: record holiday registration %s: %w", action, err)
	}
	return nil
}

func holidayReviewKey(reviewer enrollment.HolidayReviewer, periodID, registrationID int64) error {
	if reviewer.OrganizationID <= 0 || reviewer.AccountID <= 0 {
		return errHolidayPeriodRequired
	}
	if periodID <= 0 {
		return enrollment.ErrHolidayPeriodNotFound
	}
	if registrationID <= 0 {
		return enrollment.ErrHolidayRegistrationEntryNotFound
	}
	return nil
}

func validateHolidayRegistrationFilter(filter enrollment.HolidayRegistrationFilter) error {
	switch filter.Status {
	case "", enrollment.HolidayRegistrationReceived, enrollment.HolidayRegistrationConfirmed, enrollment.HolidayRegistrationCancelled:
	default:
		return fmt.Errorf("%w: unknown status %q", enrollment.ErrInvalidHolidayRegistrationFilter, filter.Status)
	}
	if filter.School != "" && filter.School != enrollment.HolidayRegistrationGuestSchool {
		if id, err := strconv.ParseInt(filter.School, 10, 64); err != nil || id <= 0 {
			return fmt.Errorf("%w: school must be a school ID or %q", enrollment.ErrInvalidHolidayRegistrationFilter, enrollment.HolidayRegistrationGuestSchool)
		}
	}
	if filter.WeekID < 0 {
		return fmt.Errorf("%w: week_id must be a positive ID", enrollment.ErrInvalidHolidayRegistrationFilter)
	}
	return nil
}

// holidayExportRows lists one row per child and booking of the filter.
func holidayExportRows(all []enrollment.HolidayRegistrationDetail, filter enrollment.HolidayRegistrationFilter) []enrollment.HolidayRegistrationExportRow {
	rows := []enrollment.HolidayRegistrationExportRow{}
	for _, registration := range all {
		if filter.Status != "" && registration.Status != filter.Status {
			continue
		}
		for _, child := range registration.Children {
			rows = appendHolidayChildRows(rows, registration, child, filter)
		}
	}
	return rows
}

func appendHolidayChildRows(rows []enrollment.HolidayRegistrationExportRow, registration enrollment.HolidayRegistrationDetail, child enrollment.HolidayRegistrationChild, filter enrollment.HolidayRegistrationFilter) []enrollment.HolidayRegistrationExportRow {
	if !holidayChildMatches(child, filter) {
		return rows
	}
	for _, booking := range child.Bookings {
		if holidayBookingMatches(booking, filter) {
			rows = append(rows, enrollment.HolidayRegistrationExportRow{
				Registration: registration.HolidayRegistrationSummary, GuardianAddress: registration.GuardianAddress,
				SecondGuardian: registration.SecondGuardian, Child: child, Booking: booking,
			})
		}
	}
	return rows
}

// holidayRegistrationMatches admits a registration of the status with a
// child that matches the school and has a booking of the week and time
// model.
func holidayRegistrationMatches(registration enrollment.HolidayRegistrationSummary, filter enrollment.HolidayRegistrationFilter) bool {
	if filter.Status != "" && registration.Status != filter.Status {
		return false
	}
	return slices.ContainsFunc(registration.Children, func(child enrollment.HolidayRegistrationChild) bool {
		return holidayChildMatches(child, filter) && slices.ContainsFunc(child.Bookings, func(booking enrollment.HolidayRegistrationBooking) bool {
			return holidayBookingMatches(booking, filter)
		})
	})
}

func holidayChildMatches(child enrollment.HolidayRegistrationChild, filter enrollment.HolidayRegistrationFilter) bool {
	switch filter.School {
	case "":
		return true
	case enrollment.HolidayRegistrationGuestSchool:
		return child.HomeSchoolID == nil
	default:
		return child.HomeSchoolID != nil && strconv.FormatInt(*child.HomeSchoolID, 10) == filter.School
	}
}

func holidayBookingMatches(booking enrollment.HolidayRegistrationBooking, filter enrollment.HolidayRegistrationFilter) bool {
	if filter.WeekID != 0 && booking.WeekID != filter.WeekID {
		return false
	}
	return filter.TimeModel == "" || strings.EqualFold(strings.TrimSpace(booking.TimeModelName), strings.TrimSpace(filter.TimeModel))
}

// holidayFilterDetails names the filter in the audit trail; empty values
// stay out.
func holidayFilterDetails(filter enrollment.HolidayRegistrationFilter) map[string]any {
	details := map[string]any{}
	if filter.Status != "" {
		details["status"] = filter.Status
	}
	if filter.School != "" {
		details["school"] = filter.School
	}
	if filter.WeekID != 0 {
		details["week_id"] = filter.WeekID
	}
	if filter.TimeModel != "" {
		details["time_model"] = filter.TimeModel
	}
	return details
}
