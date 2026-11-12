package users

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/peopledirectory"
)

// ErrGuardianHasNoEmail refuses to lift a block for a guardian without an address.
var ErrGuardianHasNoEmail = errors.New("guardian has no email address on file")

// GuardianEmailState is what the school knows about mail to one guardian's
// address (#5934). MailOutcome names the last invitation mail and stays empty
// before one exists. AddressMark is why the school's list holds the address.
// SendBlock says why a new invitation would change nothing. The stop is the
// person's and comes from the guardian itself.
type GuardianEmailState struct {
	MailOutcome string
	AddressMark string
	SendBlock   string
	// ProviderBlock is the provider's block now; only a mail it withheld asks.
	ProviderBlock string
	// AllHoldersStopped: nobody without a stop holds the address.
	AllHoldersStopped bool
	OutcomeAt         *time.Time
}

const sendBlockStopped = "stopped"

// guardianEmailStatusResponse is one guardian's line of the student's status.
type guardianEmailStatusResponse struct {
	GuardianProfileID int64  `json:"guardian_profile_id"`
	MailOutcome       string `json:"mail_outcome,omitempty"`
	AddressMark       string `json:"address_mark,omitempty"`
	EmailsStopped     bool   `json:"emails_stopped,omitempty"`
	// StopShared: another holder of the address carries no stop, so mail to the
	// address still goes out.
	StopShared    bool       `json:"stop_shared,omitempty"`
	ProviderBlock string     `json:"provider_block,omitempty"`
	SendBlock     string     `json:"send_block,omitempty"`
	OutcomeAt     *time.Time `json:"outcome_at,omitempty"`
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
	addresses, stopped := guardianEmailAddresses(rows)
	states, err := rs.runtime.GuardianEmailStates(r.Context(), addresses, stopped)
	if err != nil {
		rs.fail(w, r, FailureInternal, err)
		return
	}
	responses := make([]guardianEmailStatusResponse, 0, len(rows))
	for _, row := range rows {
		line := guardianEmailStatusResponse{GuardianProfileID: row.Guardian.ID, EmailsStopped: row.Guardian.EmailsStopped}
		if row.Guardian.Email != nil {
			state := states[strings.ToLower(strings.TrimSpace(*row.Guardian.Email))]
			line.MailOutcome, line.AddressMark, line.SendBlock, line.OutcomeAt = state.MailOutcome, state.AddressMark, state.SendBlock, state.OutcomeAt
			line.ProviderBlock = state.ProviderBlock
			line.StopShared = line.EmailsStopped && !state.AllHoldersStopped
		}
		if line.EmailsStopped {
			line.SendBlock = sendBlockStopped
		}
		responses = append(responses, line)
	}
	rs.succeed(w, r, http.StatusOK, responses, "Guardian email status retrieved successfully")
}

// guardianEmailAddresses names the addresses of the guardians, and those of the
// guardians with a stop.
func guardianEmailAddresses(rows []peopledirectory.GuardianWithLink) (addresses, stopped []string) {
	for _, row := range rows {
		if row.Guardian.Email == nil {
			continue
		}
		addresses = append(addresses, *row.Guardian.Email)
		if row.Guardian.EmailsStopped {
			stopped = append(stopped, *row.Guardian.Email)
		}
	}
	return addresses, stopped
}

// stopGuardianEmails serves POST /{id}/email-stop: the school stops mail to this
// person. The stop stays with the person, whatever address they hold later.
func (rs *GuardianResource) stopGuardianEmails(w http.ResponseWriter, r *http.Request) {
	rs.setGuardianEmailsStopped(w, r, true, "Guardian emails stopped")
}

// resumeGuardianEmails serves DELETE /{id}/email-stop.
func (rs *GuardianResource) resumeGuardianEmails(w http.ResponseWriter, r *http.Request) {
	rs.setGuardianEmailsStopped(w, r, false, "Guardian emails resumed")
}

func (rs *GuardianResource) setGuardianEmailsStopped(w http.ResponseWriter, r *http.Request, stopped bool, message string) {
	id, ok := rs.parseGuardianID(w, r)
	if !ok {
		return
	}
	if canModify, err := rs.canModifyGuardian(r, id); !canModify {
		rs.fail(w, r, FailureForbidden, err)
		return
	}
	if err := rs.runtime.SetGuardianEmailsStopped(r, id, stopped); err != nil {
		rs.guardianLookupFailure(w, r, err)
		return
	}
	rs.succeed(w, r, http.StatusOK, nil, message)
}

// liftGuardianEmailBlock serves DELETE /{id}/email-block: someone checked the
// address on file. The route has no request transaction because lifting waits for
// the mail provider, so the check reads in a short transaction of its own. The
// address never comes from the request.
func (rs *GuardianResource) liftGuardianEmailBlock(w http.ResponseWriter, r *http.Request) {
	id, ok := rs.parseGuardianID(w, r)
	if !ok {
		return
	}
	var address string
	var forbidden bool
	err := rs.runtime.InTenantTx(r.Context(), func(txCtx context.Context) error {
		txRequest := r.WithContext(txCtx)
		canModify, reason := rs.canModifyGuardian(txRequest, id)
		if !canModify {
			forbidden = true
			return reason
		}
		guardian, err := rs.directory.FindGuardian(txCtx, id)
		if err == nil && guardian.Email != nil {
			address = strings.TrimSpace(*guardian.Email)
		}
		return err
	})
	switch {
	case forbidden:
		rs.fail(w, r, FailureForbidden, err)
	case err != nil:
		rs.guardianLookupFailure(w, r, err)
	case address == "":
		rs.fail(w, r, FailureInvalidRequest, ErrGuardianHasNoEmail)
	default:
		rs.finishGuardianEmailLift(w, r, id, address)
	}
}

func (rs *GuardianResource) finishGuardianEmailLift(w http.ResponseWriter, r *http.Request, id int64, address string) {
	if err := rs.runtime.LiftGuardianEmailBlock(r, id, address); err != nil {
		rs.guardianLookupFailure(w, r, err)
		return
	}
	rs.succeed(w, r, http.StatusOK, nil, "Guardian email block lifted")
}
