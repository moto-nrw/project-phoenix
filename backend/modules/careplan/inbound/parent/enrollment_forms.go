package parent

import (
	"context"
	"net/http"
	"time"
)

// EnrollmentForms is Enrollment's public form flow as the parents portal
// serves it (#2734): the form load and the submission in the owner's wire
// shape, validation and error mapping. The portal keeps its own school,
// guardian and hidden-school gates and opens the transactions; the
// composition root binds the port to the enrollment routes.
type EnrollmentForms interface {
	// LoadFormBootstrap loads the form of the phase in the tenant
	// transaction of ctx for an enrollee whose guardian facts unlock the
	// linked_parents and existing_students audiences as given, and returns
	// the response that renders it.
	LoadFormBootstrap(ctx context.Context, phaseID int64, now time.Time, lateInviteToken string, linkedParents, existingStudents bool) (EnrollmentResponse, error)
	// RenderFormBootstrapError renders a failed form load.
	RenderFormBootstrapError(w http.ResponseWriter, r *http.Request, err error)
	// DecodeSubmission reads the public submit wire shape from the request
	// body; the late_invite query parameter fills a missing token.
	DecodeSubmission(r *http.Request) (EnrollmentSubmit, error)
	// RenderSubmitError renders a refused submission.
	RenderSubmitError(w http.ResponseWriter, r *http.Request, err error)
}

// EnrollmentResponse writes a loaded form or a stored submission as the
// route's response.
type EnrollmentResponse = func(w http.ResponseWriter, r *http.Request)

// EnrollmentSubmit stores a decoded submission for the guardian account in
// the school, under the transaction of ctx. submitEligible is the
// school-wide submit fact the linked_parents audience needs.
type EnrollmentSubmit = func(ctx context.Context, schoolID, accountID int64, submitEligible bool, clientIP string) (EnrollmentResponse, error)
