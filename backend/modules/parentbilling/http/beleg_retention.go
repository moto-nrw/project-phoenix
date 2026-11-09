package parentbillinghttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	capability "github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// BelegRetentionAudit records the operator audit entry of a change in the
// caller's administrative transaction and answers false after writing the
// error response. The root binds it to the operator audit of Organisation &
// Tenancy.
type BelegRetentionAudit func(w http.ResponseWriter, r *http.Request, kind, belegType string) bool

// Kinds of a change the audit records; the root's audit knows the same.
const (
	BelegRetentionInstruction    = "instruction"
	BelegRetentionSuspension     = "suspension"
	BelegRetentionSuspensionLift = "suspension_lift"
	BelegClarificationReview     = "clarification_review"
)

// BelegRetentionResource serves the Beleg retention of a Träger to the moto
// team in the Operator portal (#4720, ADR 0184): the overview, a new
// instruction, a suspension and its lift. Every call runs in one
// administrative transaction; a write records its operator audit entry in
// the same transaction before it writes.
type BelegRetentionResource struct {
	retention  capability.BelegRetention
	audit      BelegRetentionAudit
	operatorID func(context.Context) int64
}

// NewBelegRetentionResource builds the resource; operatorID names the
// operator of the session.
func NewBelegRetentionResource(retention capability.BelegRetention, audit BelegRetentionAudit, operatorID func(context.Context) int64) *BelegRetentionResource {
	if retention == nil || audit == nil || operatorID == nil {
		panic("parent billing http: the beleg retention, its operator audit and the operator are required")
	}
	return &BelegRetentionResource{retention: retention, audit: audit, operatorID: operatorID}
}

// Overview answers GET /operator/organizations/{id}/beleg-retention. The
// root wraps it in the operator read audit.
func (rs *BelegRetentionResource) Overview(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid organization ID")
	if !ok {
		return
	}
	var overview capability.BelegRetentionOverview
	err := tenant.WithinAdmin(r.Context(), func(adminCtx context.Context) error {
		var err error
		overview, err = rs.retention.Overview(adminCtx, organizationID)
		return err
	})
	if err != nil {
		common.RenderError(w, r, belegRetentionError(err))
		return
	}
	common.Respond(w, r, http.StatusOK, overview, "Beleg retention retrieved")
}

type instructionRequest struct {
	BelegType            string        `json:"beleg_type"`
	TriggerEvent         string        `json:"trigger_event"`
	Period               int           `json:"period"`
	Basis                string        `json:"basis"`
	LegalBasis           string        `json:"legal_basis"`
	BasisJustification   string        `json:"basis_justification"`
	InstructedBy         string        `json:"instructed_by"`
	InstructedOn         calendar.Date `json:"instructed_on"`
	InstructionReference string        `json:"instruction_reference"`
}

type suspensionRequest struct {
	BelegType   string        `json:"beleg_type"`
	BelegID     int64         `json:"beleg_id,string,omitempty"`
	Reason      string        `json:"reason"`
	Scope       string        `json:"scope"`
	Responsible string        `json:"responsible"`
	ReviewOn    calendar.Date `json:"review_on"`
}

type clarificationReviewRequest struct {
	BelegType        string        `json:"beleg_type"`
	Reason           string        `json:"reason"`
	Note             string        `json:"note"`
	Escalated        bool          `json:"escalated"`
	EscalationReason string        `json:"escalation_reason"`
	NextReviewOn     calendar.Date `json:"next_review_on"`
}

// RecordInstruction answers POST .../beleg-retention/instructions.
func (rs *BelegRetentionResource) RecordInstruction(w http.ResponseWriter, r *http.Request) {
	var req instructionRequest
	organizationID, ok := decodeBelegRetention(w, r, &req)
	if !ok {
		return
	}
	var instruction capability.RetentionInstruction
	rs.write(w, r, BelegRetentionInstruction, req.BelegType, func(ctx context.Context, operatorID int64) error {
		var err error
		instruction, err = rs.retention.RecordInstruction(ctx, organizationID, operatorID, capability.NewRetentionInstruction(req))
		return err
	}, func() { common.Respond(w, r, http.StatusCreated, instruction, "Instruction recorded") })
}

// RecordSuspension answers POST .../beleg-retention/suspensions.
func (rs *BelegRetentionResource) RecordSuspension(w http.ResponseWriter, r *http.Request) {
	var req suspensionRequest
	organizationID, ok := decodeBelegRetention(w, r, &req)
	if !ok {
		return
	}
	var suspension capability.RetentionSuspension
	rs.write(w, r, BelegRetentionSuspension, req.BelegType, func(ctx context.Context, operatorID int64) error {
		var err error
		suspension, err = rs.retention.RecordSuspension(ctx, organizationID, operatorID, capability.NewRetentionSuspension(req))
		return err
	}, func() { common.Respond(w, r, http.StatusCreated, suspension, "Suspension recorded") })
}

// RecordClarificationReview answers POST
// .../beleg-retention/clarification-reviews: the review of a clarification
// entry with its note, an escalation to Florian and the next review.
func (rs *BelegRetentionResource) RecordClarificationReview(w http.ResponseWriter, r *http.Request) {
	var req clarificationReviewRequest
	organizationID, ok := decodeBelegRetention(w, r, &req)
	if !ok {
		return
	}
	var review capability.BelegClarificationReview
	rs.write(w, r, BelegClarificationReview, req.BelegType, func(ctx context.Context, operatorID int64) error {
		var err error
		review, err = rs.retention.RecordClarificationReview(ctx, organizationID, operatorID, capability.NewBelegClarificationReview(req))
		return err
	}, func() { common.Respond(w, r, http.StatusCreated, review, "Clarification review recorded") })
}

// LiftSuspension answers DELETE .../beleg-retention/suspensions/{suspensionId}:
// the suspension ends and stays as history.
func (rs *BelegRetentionResource) LiftSuspension(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid organization ID")
	if !ok {
		return
	}
	suspensionID, ok := common.ParseInt64IDWithError(w, r, "suspensionId", "invalid suspension ID")
	if !ok {
		return
	}
	rs.write(w, r, BelegRetentionSuspensionLift, "suspension", func(ctx context.Context, operatorID int64) error {
		return rs.retention.LiftSuspension(ctx, organizationID, suspensionID, operatorID)
	}, func() { common.RespondNoContent(w, r) })
}

// write runs the audit and the change in one administrative transaction; a
// refused audit has already answered, a failed change rolls the audit back.
func (rs *BelegRetentionResource) write(w http.ResponseWriter, r *http.Request, kind, belegType string,
	change func(context.Context, int64) error, respond func(),
) {
	audited := true
	err := tenant.WithinAdmin(r.Context(), func(adminCtx context.Context) error {
		if !rs.audit(w, r.WithContext(adminCtx), kind, belegType) {
			audited = false
			return errAuditRefused
		}
		return change(adminCtx, rs.operatorID(r.Context()))
	})
	switch {
	case !audited:
		return
	case err != nil:
		common.RenderError(w, r, belegRetentionError(err))
	default:
		respond()
	}
}

var errAuditRefused = errors.New("operator audit refused the change")

func decodeBelegRetention(w http.ResponseWriter, r *http.Request, req any) (int64, bool) {
	organizationID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid organization ID")
	if !ok {
		return 0, false
	}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		common.RenderError(w, r, common.OperatorInvalidRequest(err))
		return 0, false
	}
	return organizationID, true
}

// belegRetentionError answers a refused change with the code of its class.
func belegRetentionError(err error) render.Renderer {
	switch {
	case errors.Is(err, capability.ErrBelegRetentionInvalid):
		return common.OperatorInvalidRequestWithCode(err, common.CodeGeneralInput)
	case errors.Is(err, capability.ErrBelegSuspensionNotFound), errors.Is(err, capability.ErrBelegTraegerNotFound),
		errors.Is(err, capability.ErrBelegClarificationNotFound):
		return common.OperatorRejection(http.StatusNotFound, common.CodeGeneralBusinessRejection, err.Error())
	default:
		return common.OperatorInternal("beleg retention failed")
	}
}
