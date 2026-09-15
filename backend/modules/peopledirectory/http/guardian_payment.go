package users

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// Guardian payment endpoints (#2608). The whole section sits behind
// guardians:financial, not users:update: maintaining the directory and
// handling bank data are different jobs. Every read is access-logged by the
// owner; the adapter never sees an unmasked IBAN except in the reveal
// response and the export file.

// GuardianPaymentMaskedResponse is the default bank read of one guardian.
type GuardianPaymentMaskedResponse struct {
	GuardianID    string  `json:"guardian_id"`
	IBANMasked    *string `json:"iban_masked"`
	AccountHolder *string `json:"account_holder"`
}

// GuardianPaymentRevealResponse carries the unmasked IBAN after the audited
// reveal.
type GuardianPaymentRevealResponse struct {
	GuardianID    string  `json:"guardian_id"`
	IBAN          *string `json:"iban"`
	AccountHolder *string `json:"account_holder"`
}

// GuardianPaymentRequest binds the bank PUT. An empty string clears a field.
type GuardianPaymentRequest struct {
	IBAN          *string `json:"iban"`
	AccountHolder *string `json:"account_holder"`
	Note          string  `json:"note"`
}

func (r *GuardianPaymentRequest) Bind(_ *http.Request) error { return nil }

// StudentPayerRequest binds the per-child payer assignment; a nil
// guardian_id clears the assignment.
type StudentPayerRequest struct {
	GuardianID *string `json:"guardian_id"`
}

func (r *StudentPayerRequest) Bind(_ *http.Request) error { return nil }

// PaymentOverviewRow is one line of the Bankverbindungen list.
type PaymentOverviewRow struct {
	StudentID        string  `json:"student_id"`
	StudentName      string  `json:"student_name"`
	SchoolClass      string  `json:"school_class"`
	GuardianID       *string `json:"guardian_id"`
	GuardianName     string  `json:"guardian_name"`
	RelationshipType string  `json:"relationship_type"`
	AccountHolder    string  `json:"account_holder"`
	IBANMasked       string  `json:"iban_masked"`
}

// Export formats the renderer understands.
const (
	exportFormatPDF  = "pdf"
	exportFormatDOCX = "docx"
	exportFormatXLSX = "xlsx"
)

// PaymentExportRequest binds the export POST. The format is rejected HERE,
// before the rows are loaded: loading them writes an access-log entry, and
// an export that never happened must not leave that trace behind.
type PaymentExportRequest struct {
	Format string `json:"format"`
}

func (r *PaymentExportRequest) Bind(_ *http.Request) error {
	format := strings.ToLower(strings.TrimSpace(r.Format))
	if format == "" {
		format = exportFormatPDF
	}
	switch format {
	case exportFormatPDF, exportFormatDOCX, exportFormatXLSX:
		r.Format = format
		return nil
	default:
		return fmt.Errorf("unsupported export format %q (use pdf, docx or xlsx)", r.Format)
	}
}

// paymentActor pulls the acting account from the token for the GDPR access
// log; the role is the comma-joined role list.
func (rs *GuardianResource) paymentActor(w http.ResponseWriter, r *http.Request) (peopledirectory.GuardianPaymentActor, bool) {
	accountID := rs.runtime.ActorID(r)
	if accountID <= 0 {
		rs.failMessage(w, r, FailureUnauthorized, "invalid token: no account id")
		return peopledirectory.GuardianPaymentActor{}, false
	}
	return peopledirectory.GuardianPaymentActor{AccountID: accountID, Role: rs.runtime.ActorRole(r)}, true
}

// paymentErrorMessages renders each input error as the German sentence the
// school office reads in the toast.
var paymentErrorMessages = []struct {
	sentinel error
	message  string
}{
	{peopledirectory.ErrGuardianIBANInvalid, "Die IBAN ist nicht gültig. Bitte prüfen Sie die Eingabe."},
	{peopledirectory.ErrGuardianAccountHolderTooLong, "Der Name des Kontoinhabers ist zu lang."},
	{peopledirectory.ErrGuardianNotLinkedToStudent, "Diese Person ist bei diesem Kind nicht als erziehungsberechtigt eingetragen."},
	{peopledirectory.ErrGuardianStudentRequired, "Das Kind konnte nicht zugeordnet werden."},
}

// renderPaymentError maps the owner sentinels to their status and to a
// message the reader can act on.
func (rs *GuardianResource) renderPaymentError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, peopledirectory.ErrGuardianNotFound) || errors.Is(err, peopledirectory.ErrStudentNotFound) {
		rs.fail(w, r, FailureNotFound, err)
		return
	}
	for _, rule := range paymentErrorMessages {
		if errors.Is(err, rule.sentinel) {
			rs.failMessage(w, r, FailureInvalidRequest, rule.message)
			return
		}
	}
	if errors.Is(err, peopledirectory.ErrGuardianPaymentInvalid) {
		rs.failMessage(w, r, FailureInvalidRequest, "Die Eingabe ist nicht gültig. Bitte prüfen Sie die Felder.")
		return
	}
	rs.fail(w, r, FailureInternal, err)
}

func (rs *GuardianResource) parsePaymentGuardianID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	return rs.parseIDParam(w, r, "id", msgInvalidPaymentGuardian)
}

func (rs *GuardianResource) getGuardianPayment(w http.ResponseWriter, r *http.Request) {
	id, ok := rs.parsePaymentGuardianID(w, r)
	if !ok {
		return
	}
	actor, ok := rs.paymentActor(w, r)
	if !ok {
		return
	}
	masked, err := rs.directory.GuardianPaymentMasked(r.Context(), id, actor)
	if err != nil {
		rs.renderPaymentError(w, r, err)
		return
	}
	rs.succeed(w, r, http.StatusOK, &GuardianPaymentMaskedResponse{
		GuardianID: strconv.FormatInt(masked.GuardianProfileID, 10), IBANMasked: masked.IBAN, AccountHolder: masked.AccountHolder,
	}, "Payment data retrieved successfully")
}

// revealGuardianPayment is a POST on purpose: serving the unmasked IBAN is
// an audited action, not a cacheable read.
func (rs *GuardianResource) revealGuardianPayment(w http.ResponseWriter, r *http.Request) {
	id, ok := rs.parsePaymentGuardianID(w, r)
	if !ok {
		return
	}
	actor, ok := rs.paymentActor(w, r)
	if !ok {
		return
	}
	plain, err := rs.directory.RevealGuardianPayment(r.Context(), id, actor)
	if err != nil {
		rs.renderPaymentError(w, r, err)
		return
	}
	rs.succeed(w, r, http.StatusOK, &GuardianPaymentRevealResponse{
		GuardianID: strconv.FormatInt(plain.GuardianProfileID, 10), IBAN: plain.IBAN, AccountHolder: plain.AccountHolder,
	}, "Payment data revealed successfully")
}

func (rs *GuardianResource) updateGuardianPayment(w http.ResponseWriter, r *http.Request) {
	id, ok := rs.parsePaymentGuardianID(w, r)
	if !ok {
		return
	}
	req := &GuardianPaymentRequest{}
	if err := render.Bind(r, req); err != nil {
		rs.fail(w, r, FailureInvalidRequest, err)
		return
	}
	actor, ok := rs.paymentActor(w, r)
	if !ok {
		return
	}
	if err := rs.directory.UpdateGuardianPayment(r.Context(), id, peopledirectory.GuardianPaymentInput{
		IBAN: req.IBAN, AccountHolder: req.AccountHolder, Note: req.Note, ActorAccountID: actor.AccountID,
	}); err != nil {
		rs.renderPaymentError(w, r, err)
		return
	}
	rs.succeed(w, r, http.StatusOK, nil, "Payment data updated successfully")
}

func (rs *GuardianResource) setStudentPayer(w http.ResponseWriter, r *http.Request) {
	studentID, ok := rs.parseStudentID(w, r)
	if !ok {
		return
	}
	req := &StudentPayerRequest{}
	if err := render.Bind(r, req); err != nil {
		rs.fail(w, r, FailureInvalidRequest, err)
		return
	}
	actor, ok := rs.paymentActor(w, r)
	if !ok {
		return
	}
	// guardians:financial says the caller may see bank data; whether they may
	// change this child's guardians is the same active-student and verified
	// staff check every relationship write runs.
	if canModify, err := rs.canModifyStudent(r, studentID); !canModify {
		rs.fail(w, r, FailureForbidden, err)
		return
	}
	var guardianID *int64
	if req.GuardianID != nil && strings.TrimSpace(*req.GuardianID) != "" {
		parsed, err := strconv.ParseInt(strings.TrimSpace(*req.GuardianID), 10, 64)
		if err != nil || parsed <= 0 {
			rs.failMessage(w, r, FailureInvalidRequest, msgInvalidPaymentGuardian)
			return
		}
		guardianID = &parsed
	}
	if err := rs.directory.SetStudentPayer(r.Context(), peopledirectory.StudentPayer{
		StudentID: studentID, GuardianProfileID: guardianID, ActorAccountID: actor.AccountID,
	}); err != nil {
		rs.renderPaymentError(w, r, err)
		return
	}
	rs.succeed(w, r, http.StatusOK, nil, "Payer updated successfully")
}

func (rs *GuardianResource) listPaymentOverview(w http.ResponseWriter, r *http.Request) {
	actor, ok := rs.paymentActor(w, r)
	if !ok {
		return
	}
	rows, err := rs.directory.ListPaymentOverview(r.Context(), actor)
	if err != nil {
		rs.renderPaymentError(w, r, err)
		return
	}
	responses := make([]PaymentOverviewRow, 0, len(rows))
	for _, row := range rows {
		out := PaymentOverviewRow{
			StudentID: strconv.FormatInt(row.StudentID, 10), StudentName: row.StudentName, SchoolClass: row.SchoolClass,
			GuardianName: row.GuardianName, RelationshipType: row.RelationshipType,
			AccountHolder: row.AccountHolder, IBANMasked: row.IBANMasked,
		}
		if row.GuardianProfileID != nil {
			id := strconv.FormatInt(*row.GuardianProfileID, 10)
			out.GuardianID = &id
		}
		responses = append(responses, out)
	}
	rs.succeed(w, r, http.StatusOK, responses, "Payment overview retrieved successfully")
}

func (rs *GuardianResource) exportPaymentOverview(w http.ResponseWriter, r *http.Request) {
	req := &PaymentExportRequest{}
	if err := render.Bind(r, req); err != nil {
		rs.fail(w, r, FailureInvalidRequest, err)
		return
	}
	actor, ok := rs.paymentActor(w, r)
	if !ok {
		return
	}
	rows, err := rs.directory.ListPaymentExportRows(r.Context(), actor, req.Format)
	if err != nil {
		rs.renderPaymentError(w, r, err)
		return
	}
	file, err := rs.runtime.RenderPaymentExport(rows, req.Format)
	if err != nil {
		// The renderer reports an unsupported layout as bad input, exactly as
		// the legacy handler did.
		rs.fail(w, r, FailureInvalidRequest, err)
		return
	}
	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.Filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(file.Data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Data)
	rs.runtime.ObserveResponse(http.StatusOK, "none")
}
