package api

import "fmt"

// enrollmentSubmission is one demo enrollment request: who submits it, its
// body, and the decision the school takes on it ("" leaves it open).
type enrollmentSubmission struct {
	source string
	key    string
	body   map[string]any
	auth   AuthRef
	path   string
	status string
	reason string
}

// seedSubmissionProgress records the one-off steps of a submission run: the
// first approval switches the activation mode to immediate, and the first
// open public request is withdrawn.
type seedSubmissionProgress struct {
	immediateSeeded bool
	withdrawnSeeded bool
}

// submitSeedEnrollments submits the requests in order, records each in the
// state and settles it.
func (s parentEnrollmentSeedStep) submitSeedEnrollments(rt *Runtime, adminAuth AuthRef, offerings map[string]int64, submissions []enrollmentSubmission, state *SeedEnrollmentState) error {
	var progress seedSubmissionProgress
	for index, submission := range submissions {
		request, err := s.submitSeedEnrollment(rt, adminAuth, submission, index)
		if err != nil {
			return err
		}
		state.Requests = append(state.Requests, request)
		if err := s.settleSeedEnrollment(rt, adminAuth, submission, request, &state.Requests[len(state.Requests)-1], &progress); err != nil {
			return err
		}
		if err := s.seedSubmissionFollowUps(rt, adminAuth, offerings, submission, request); err != nil {
			return err
		}
	}
	return nil
}

// submitSeedEnrollment sends one request, the parent's through the parent
// portal after loading its profile, and returns it with its detail.
func (s parentEnrollmentSeedStep) submitSeedEnrollment(rt *Runtime, adminAuth AuthRef, submission enrollmentSubmission, index int) (SeedEnrollmentRequest, error) {
	if submission.source == "parent" {
		if _, err := rt.Client.GetWithAuth(submission.auth, "/parent/enrollments/"+rt.Bootstrap.TenantSlug+"/profile"); err != nil {
			return SeedEnrollmentRequest{}, fmt.Errorf("load parent enrollment profile before submit: %w", err)
		}
	}

	var respBody []byte
	var submitErr error
	if submission.source == "parent" {
		respBody, submitErr = rt.Client.PostWithAuth(submission.auth, submission.path, submission.body)
	} else {
		respBody, submitErr = rt.Client.PostPublicWithHeaders(
			submission.path,
			submission.body,
			publicEnrollmentSeedHeaders(index),
		)
	}
	if submitErr != nil {
		return SeedEnrollmentRequest{}, fmt.Errorf("submit %s enrollment: %w", submission.source, submitErr)
	}
	request, err := parseEnrollmentSubmitResponse(respBody, submission.source)
	if err != nil {
		return SeedEnrollmentRequest{}, err
	}
	detail, err := s.loadEnrollmentRequestDetail(rt, adminAuth, request.RequestID)
	if err != nil {
		return SeedEnrollmentRequest{}, err
	}
	request.StatusToken = detail.StatusToken
	request.ChildIDs = detail.ChildIDs
	request.Key = submission.key
	return request, nil
}

// settleSeedEnrollment decides the request as listed, or withdraws the first
// open public one, and records the new status on the stored request.
func (s parentEnrollmentSeedStep) settleSeedEnrollment(rt *Runtime, adminAuth AuthRef, submission enrollmentSubmission, request SeedEnrollmentRequest, stored *SeedEnrollmentRequest, progress *seedSubmissionProgress) error {
	if submission.status != "" {
		if err := s.decideSeedSubmission(rt, adminAuth, request, submission.status, submission.reason, &progress.immediateSeeded); err != nil {
			return err
		}
		stored.Status = submission.status
	} else if !progress.withdrawnSeeded && submission.source == "public" {
		if _, err := rt.Client.PostPublic(fmt.Sprintf("/api/enrollment/requests/%s/withdraw", request.StatusToken), map[string]any{}); err != nil {
			return fmt.Errorf("withdraw demo enrollment request: %w", err)
		}
		stored.Status = "withdrawn"
		progress.withdrawnSeeded = true
	}
	return nil
}

// seedSubmissionFollowUps changes the offerings of the parent's approved
// request and opens a change conversation on every rejected one.
func (s parentEnrollmentSeedStep) seedSubmissionFollowUps(rt *Runtime, adminAuth AuthRef, offerings map[string]int64, submission enrollmentSubmission, request SeedEnrollmentRequest) error {
	if submission.source == "parent" && submission.status == "approved" {
		if err := s.seedOfferingChange(rt, adminAuth, submission.auth, request.RequestID, offerings); err != nil {
			return err
		}
	}
	if submission.status == "rejected" {
		if err := seedEnrollmentChangeConversation(rt, adminAuth, submission.body, request); err != nil {
			return err
		}
	}
	return nil
}
