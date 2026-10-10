package enrollmenthttp

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// ParentForms serves the public form flow to the authenticated parents
// portal (#2734): the form load without a captcha and the submission in the
// public wire shape, stamped with the guardian account. The portal keeps its
// own school and guardian gates and its transactions; the composition root
// binds the portal's port to these methods.
type ParentForms struct {
	Requests RequestService
}

// LoadFormBootstrap loads the form of the phase for an enrollee whose
// guardian facts unlock the restricted audiences as given. The response
// carries no captcha: the parent JWT is the anti-bot signal.
func (f ParentForms) LoadFormBootstrap(ctx context.Context, phaseID int64, now time.Time, lateInviteToken string, linkedParents, existingStudents bool) (func(http.ResponseWriter, *http.Request), error) {
	access := EnrolleeAudienceAccess{LinkedParents: linkedParents, ExistingStudents: existingStudents}
	data, err := f.Requests.LoadEnrolleeFormBootstrap(ctx, phaseID, now, lateInviteToken, access)
	if err != nil {
		return nil, err
	}
	return func(w http.ResponseWriter, r *http.Request) {
		resp := buildPublicEnrollmentFormBootstrapResponse(data, PublicCaptchaConfigResponse{})
		common.Respond(w, r, http.StatusOK, resp, "Parent enrollment form bootstrap retrieved")
	}, nil
}

// RenderFormBootstrapError renders a failed form load like the public route.
func (ParentForms) RenderFormBootstrapError(w http.ResponseWriter, r *http.Request, err error) {
	renderPublicBootstrapError(w, r, err)
}

// DecodeSubmission reads the public submit wire shape. Bind() defaults nil
// maps and slices; a token missing from the body comes from the late_invite
// query parameter.
func (f ParentForms) DecodeSubmission(r *http.Request) (func(context.Context, int64, int64, bool, string) (func(http.ResponseWriter, *http.Request), error), error) {
	wireReq := &SubmitEnrollmentRequest{}
	if err := json.NewDecoder(r.Body).Decode(wireReq); err != nil {
		return nil, err
	}
	_ = wireReq.Bind(r)
	if wireReq.LateInviteToken == "" {
		wireReq.LateInviteToken = strings.TrimSpace(r.URL.Query().Get("late_invite"))
	}
	return func(ctx context.Context, schoolID, accountID int64, submitEligible bool, clientIP string) (func(http.ResponseWriter, *http.Request), error) {
		return f.submit(ctx, wireReq, schoolID, accountID, submitEligible, clientIP)
	}, nil
}

// submit binds the wire request for the school, stamps the guardian account
// id and the submit eligibility, and submits under the tenant context. A
// parse failure returns before the submission.
func (f ParentForms) submit(ctx context.Context, wireReq *SubmitEnrollmentRequest, schoolID, accountID int64, submitEligible bool, clientIP string) (func(http.ResponseWriter, *http.Request), error) {
	serviceReq, err := buildServiceRequest(wireReq, schoolID, clientIP)
	if err != nil {
		return nil, err
	}
	serviceReq.GuardianAccountID = &accountID
	serviceReq.GuardianSubmitEligible = submitEligible
	result, err := f.Requests.Submit(tenant.WithTenantID(ctx, schoolID), serviceReq)
	if err != nil {
		return nil, err
	}
	return func(w http.ResponseWriter, r *http.Request) {
		resp := SubmitEnrollmentResponse{
			RequestID: strconv.FormatInt(result.Request.ID, 10),
			StatusURL: result.StatusURL,
			Warnings:  result.Warnings,
		}
		common.Respond(w, r, http.StatusCreated, resp, "Enrollment submitted")
	}, nil
}

// RenderSubmitError renders a refused submission like the public route.
func (ParentForms) RenderSubmitError(w http.ResponseWriter, r *http.Request, err error) {
	mapSubmitError(w, r, err)
}
