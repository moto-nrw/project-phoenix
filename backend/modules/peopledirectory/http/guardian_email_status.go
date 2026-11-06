package users

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
)

// ErrGuardianHasNoEmail refuses a change to the mail state of a guardian without
// an address.
var ErrGuardianHasNoEmail = errors.New("guardian has no email address on file")

// GuardianEmailState is what the school knows about mail to one guardian's
// address (#5934). MailOutcome names the last invitation mail and stays empty
// before one exists. AddressMark is why the school's list holds the address.
// SendBlock says why a new invitation would change nothing.
type GuardianEmailState struct {
	MailOutcome string
	AddressMark string
	SendBlock   string
	OutcomeAt   *time.Time
}

// guardianEmailStatusResponse is one guardian's line of the student's status.
type guardianEmailStatusResponse struct {
	GuardianProfileID int64      `json:"guardian_profile_id"`
	MailOutcome       string     `json:"mail_outcome,omitempty"`
	AddressMark       string     `json:"address_mark,omitempty"`
	SendBlock         string     `json:"send_block,omitempty"`
	OutcomeAt         *time.Time `json:"outcome_at,omitempty"`
}

// getStudentGuardianEmailStatus serves GET /students/{studentId}/email-status.
// It reads the guardians once and their addresses in one read each.
func (rs *GuardianResource) getStudentGuardianEmailStatus(w http.ResponseWriter, r *http.Request) {
	studentID, ok := rs.parseStudentID(w, r)
	if !ok {
		return
	}
	if _, active := rs.activeStudent(r.Context(), studentID); !active {
		rs.failMessage(w, r, FailureNotFound, msgStudentNotFound)
		return
	}
	rows, err := rs.directory.ListStudentGuardians(r.Context(), studentID)
	if err != nil {
		rs.moduleFailure(w, r, err)
		return
	}
	addresses := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.Guardian.Email != nil {
			addresses = append(addresses, *row.Guardian.Email)
		}
	}
	states, err := rs.runtime.GuardianEmailStates(r.Context(), addresses)
	if err != nil {
		rs.fail(w, r, FailureInternal, err)
		return
	}
	responses := make([]guardianEmailStatusResponse, 0, len(rows))
	for _, row := range rows {
		line := guardianEmailStatusResponse{GuardianProfileID: row.Guardian.ID}
		if row.Guardian.Email != nil {
			state := states[strings.ToLower(strings.TrimSpace(*row.Guardian.Email))]
			line.MailOutcome, line.AddressMark, line.SendBlock, line.OutcomeAt = state.MailOutcome, state.AddressMark, state.SendBlock, state.OutcomeAt
		}
		responses = append(responses, line)
	}
	rs.succeed(w, r, http.StatusOK, responses, "Guardian email status retrieved successfully")
}

// stopGuardianEmails serves POST /{id}/email-block: the school stops mail to this
// guardian's address.
func (rs *GuardianResource) stopGuardianEmails(w http.ResponseWriter, r *http.Request) {
	rs.changeGuardianEmailBlock(w, r, rs.runtime.StopGuardianEmails, "Guardian emails stopped")
}

// liftGuardianEmailBlock serves DELETE /{id}/email-block: someone checked the
// address, or the school takes its stop back.
func (rs *GuardianResource) liftGuardianEmailBlock(w http.ResponseWriter, r *http.Request) {
	rs.changeGuardianEmailBlock(w, r, rs.runtime.LiftGuardianEmailBlock, "Guardian email block lifted")
}

func (rs *GuardianResource) changeGuardianEmailBlock(w http.ResponseWriter, r *http.Request, change func(ctx context.Context, guardianID int64) error, message string) {
	id, ok := rs.parseGuardianID(w, r)
	if !ok {
		return
	}
	if err := change(r.Context(), id); err != nil {
		if errors.Is(err, ErrGuardianHasNoEmail) {
			rs.fail(w, r, FailureInvalidRequest, err)
			return
		}
		rs.guardianLookupFailure(w, r, err)
		return
	}
	rs.succeed(w, r, http.StatusOK, nil, message)
}
