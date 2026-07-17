package enrollment

import (
	"context"
	"errors"
	"strconv"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

// manualEnrollmentDecider is the narrow slice of DecisionService the
// manual-approved-enrollment flow needs to approve the freshly submitted
// request inside the same transaction.
type manualEnrollmentDecider interface {
	Decide(ctx context.Context, input DecideInput) (*DecideOutcome, error)
}

// ManualApprovedEnrollmentInput carries the already-normalized submission
// plus the privileged-flow parameters. Request must already have PhaseID,
// tenant and guardian fields set (built from the wire request); the policy
// flags are applied by CreateManualApprovedEnrollment.
type ManualApprovedEnrollmentInput struct {
	Request          SubmitRequest
	Reason           string
	SendNotification bool
	ActorID          int64
}

// ManualApprovedEnrollmentResult is the outcome of a manual approved
// enrollment: the created request, the approved child, its status URL and
// any pending guardian invite for post-decision dispatch.
type ManualApprovedEnrollmentResult struct {
	Request       *enrollmentModels.Request
	Child         *enrollmentModels.RequestChild
	StatusURL     string
	PendingInvite *PendingGuardianInvite
}

func (s *requestService) CreateManualApprovedEnrollment(ctx context.Context, input ManualApprovedEnrollmentInput) (*ManualApprovedEnrollmentResult, error) {
	if s.ManualDecider == nil {
		return nil, errors.New("manual enrollment decider is not configured")
	}
	req := input.Request
	req.SkipRateLimit = true
	req.AllowClosedPhase = true
	req.SuppressSubmissionEmails = true
	req.ExternalConsentConfirmed = true
	req.SubmissionSource = enrollmentModels.RequestSourceAdminManual
	req.SourceMetadata = map[string]any{
		"external_consent_confirmed": true,
		"admin_reason":               input.Reason,
		"send_notification":          input.SendNotification,
		"actor_account_id":           input.ActorID,
	}

	submitted, err := s.Submit(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(submitted.Children) != 1 {
		return nil, errManualEnrollmentChildCount(len(submitted.Children))
	}
	decided, err := s.ManualDecider.Decide(ctx, DecideInput{
		RequestID:                  submitted.Request.ID,
		ChildID:                    submitted.Children[0].ID,
		Status:                     DecisionApproved,
		Reason:                     input.Reason,
		ReviewedBy:                 input.ActorID,
		SuppressParentEmail:        !input.SendNotification,
		SuppressGuardianInvitation: !input.SendNotification,
	})
	if err != nil {
		return nil, err
	}
	return &ManualApprovedEnrollmentResult{
		Request:       submitted.Request,
		Child:         decided.Child,
		StatusURL:     submitted.StatusURL,
		PendingInvite: decided.PendingInvite,
	}, nil
}

func errManualEnrollmentChildCount(count int) error {
	return errors.New("manual approved enrollment produced " + strconv.Itoa(count) + " children")
}
