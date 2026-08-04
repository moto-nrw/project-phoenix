package enrollment

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/render"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// AdminRequestSummary is the wire shape for admin list-style responses.
// Carries the request + per-child overview + the phase name so the
// list can render without a second fetch. It must not carry status_token:
// the token authorizes parent-facing status/edit routes.
//
// CustomData + ConsentFlags + SchemaFields are populated on the
// detail endpoint so the decision UI can render every parent-supplied
// answer next to its label. Listing endpoints leave them empty to
// keep payloads light.
type AdminRequestSummary struct {
	ID                string                    `json:"id"`
	PhaseID           string                    `json:"phase_id"`
	PhaseName         string                    `json:"phase_name"`
	GuardianFirstName string                    `json:"guardian_first_name"`
	GuardianLastName  string                    `json:"guardian_last_name"`
	GuardianEmail     string                    `json:"guardian_email"`
	GuardianPhone     *string                   `json:"guardian_phone,omitempty"`
	SubmittedAt       time.Time                 `json:"submitted_at"`
	WithdrawnAt       *time.Time                `json:"withdrawn_at,omitempty"`
	CustomData        map[string]any            `json:"custom_data,omitempty"`
	ConsentFlags      map[string]any            `json:"consent_flags,omitempty"`
	SchemaFields      []AdminRequestSchemaField `json:"schema_fields,omitempty"`
	// SchemaLegalBlocks carries key→title pairs from the pinned schema's
	// legal blocks so the detail UI can label custom consent flags (e.g.
	// "Schwimmbad") instead of rendering raw keys. Detail endpoint only.
	SchemaLegalBlocks []AdminRequestSchemaLegalBlock `json:"schema_legal_blocks,omitempty"`
	Children          []AdminRequestChild            `json:"children"`
	// AdditionalGuardians are the co-guardians the parent added beyond the
	// primary guardian above. Empty when none were added.
	AdditionalGuardians []AdminRequestGuardian `json:"additional_guardians,omitempty"`
}

// AdminRequestDetail is the manage-only response shape for a single
// enrollment request. It includes the parent status token because the
// detail UI exposes a direct link to the parent status page.
type AdminRequestDetail struct {
	AdminRequestSummary
	StatusToken string `json:"status_token"`
}

// AdminRequestGuardian is one additional guardian (co-guardian) within an
// admin summary/detail payload. Email/phone are optional.
type AdminRequestGuardian struct {
	ID        string  `json:"id"`
	FirstName string  `json:"first_name"`
	LastName  string  `json:"last_name"`
	Email     *string `json:"email,omitempty"`
	Phone     *string `json:"phone,omitempty"`
}

// AdminRequestSchemaLegalBlock is the slim legal-block shape the admin
// detail UI needs to render consent flags with their original title.
type AdminRequestSchemaLegalBlock struct {
	Key   string `json:"key"`
	Title string `json:"title"`
}

// AdminRequestSchemaField is the slim form-field shape the admin UI
// needs to render parent-submitted answers with their original label
// + target hint. Mirrors enrollmentModels.FormField but stringified
// + tagged for JSON.
type AdminRequestSchemaField struct {
	Key            string                          `json:"key"`
	Label          string                          `json:"label"`
	Type           string                          `json:"type"`
	AppliesToChild bool                            `json:"applies_to_child"`
	Target         string                          `json:"target,omitempty"`
	Options        []AdminRequestSchemaFieldOption `json:"options,omitempty"`
}

// AdminRequestSchemaFieldOption mirrors enrollmentModels.FormFieldOption
// so the admin renderer can turn a stored value (e.g. "picked_up")
// back into its human-readable label (e.g. "Wird abgeholt").
type AdminRequestSchemaFieldOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// AdminRequestChild is one child within an admin summary/detail
// payload.
type AdminRequestChild struct {
	ID                string         `json:"id"`
	FirstName         string         `json:"first_name"`
	LastName          string         `json:"last_name"`
	DateOfBirth       string         `json:"date_of_birth"`
	TargetGradeLevel  *int16         `json:"target_grade_level,omitempty"`
	TargetSchoolClass *string        `json:"target_school_class,omitempty"`
	Status            string         `json:"status"`
	StatusReason      *string        `json:"status_reason,omitempty"`
	ReviewedAt        *time.Time     `json:"reviewed_at,omitempty"`
	ReviewedBy        *int64         `json:"reviewed_by,omitempty"`
	ActivationMode    string         `json:"activation_mode"`
	CreatedStudentID  string         `json:"created_student_id,omitempty"`
	CustomData        map[string]any `json:"custom_data,omitempty"`
	// Offerings is the per-child Betreuungsangebote selection.
	// Populated only on the detail endpoint (listing endpoints leave
	// it empty to keep the payload small).
	Offerings []AdminRequestChildOffering `json:"offerings,omitempty"`
}

// AdminRequestChildOffering is one care-offering pick for a child.
// SelectedDays is non-empty only for offerings whose
// days_of_week_mode is "parent_choice"; otherwise it's omitted and
// the offering's AvailableDays describes the (admin-fixed) schedule.
type AdminRequestChildOffering struct {
	OfferingID            string   `json:"offering_id"`
	OfferingName          string   `json:"offering_name"`
	DaysOfWeekMode        string   `json:"days_of_week_mode"`
	SelectedDays          []string `json:"selected_days,omitempty"`
	ManualSelectedDays    []string `json:"manual_selected_days,omitempty"`
	AutomaticSelectedDays []string `json:"automatic_selected_days,omitempty"`
	AvailableDays         []string `json:"available_days,omitempty"`
}

type AdminOfferingAdjustment struct {
	ID                 string          `json:"id"`
	RequestID          string          `json:"request_id"`
	RequestChildID     string          `json:"request_child_id"`
	StudentID          string          `json:"student_id"`
	ActorAccountID     string          `json:"actor_account_id"`
	ActorRole          string          `json:"actor_role"`
	ActorNameSnapshot  *string         `json:"actor_name_snapshot,omitempty"`
	ActorEmailSnapshot *string         `json:"actor_email_snapshot,omitempty"`
	Reason             string          `json:"reason"`
	Before             json.RawMessage `json:"before"`
	After              json.RawMessage `json:"after"`
	ChangedAt          time.Time       `json:"changed_at"`
}

func toAdminRequestSummary(s *enrollmentService.RequestSummary) AdminRequestSummary {
	out := AdminRequestSummary{
		ID:                strconv.FormatInt(s.Request.ID, 10),
		PhaseID:           strconv.FormatInt(s.Request.PhaseID, 10),
		GuardianFirstName: s.Request.GuardianFirstName,
		GuardianLastName:  s.Request.GuardianLastName,
		GuardianEmail:     s.Request.GuardianEmail,
		GuardianPhone:     s.Request.GuardianPhone,
		SubmittedAt:       s.Request.SubmittedAt,
		WithdrawnAt:       s.Request.WithdrawnAt,
	}
	if s.Phase != nil {
		out.PhaseName = s.Phase.Name
	}
	out.Children = make([]AdminRequestChild, 0, len(s.Children))
	for _, c := range s.Children {
		out.Children = append(out.Children, AdminRequestChild{
			ID:                strconv.FormatInt(c.ID, 10),
			FirstName:         c.FirstName,
			LastName:          c.LastName,
			DateOfBirth:       c.DateOfBirth.String(),
			TargetGradeLevel:  c.TargetGradeLevel,
			TargetSchoolClass: c.TargetSchoolClass,
			Status:            c.Status,
			StatusReason:      c.StatusReason,
			ReviewedAt:        c.ReviewedAt,
			ReviewedBy:        c.ReviewedBy,
			ActivationMode:    c.ActivationMode,
			CreatedStudentID:  optionalInt64String(c.CreatedStudentID),
			CustomData:        c.CustomData,
		})
	}
	out.AdditionalGuardians = make([]AdminRequestGuardian, 0, len(s.Guardians))
	for _, g := range s.Guardians {
		out.AdditionalGuardians = append(out.AdditionalGuardians, AdminRequestGuardian{
			ID:        strconv.FormatInt(g.ID, 10),
			FirstName: g.FirstName,
			LastName:  g.LastName,
			Email:     g.Email,
			Phone:     g.Phone,
		})
	}
	return out
}

// listAdminRequests returns the queue of submissions for the tenant in
// session. Filters: phase_id, child_status. Both optional.
func (rs *Resource) listAdminRequests(w http.ResponseWriter, r *http.Request) {
	if rs.DecisionService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("decision service not configured")))
		return
	}

	filters := enrollmentService.RequestFilters{}
	if v := r.URL.Query().Get("phase_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid phase_id")))
			return
		}
		filters.PhaseID = id
	}
	if v := r.URL.Query().Get("child_status"); v != "" {
		filters.ChildStatus = v
	}

	var summaries []*enrollmentService.RequestSummary
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		list, listErr := rs.DecisionService.List(ctx, filters)
		summaries = list
		return listErr
	})
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	out := make([]AdminRequestSummary, 0, len(summaries))
	for _, s := range summaries {
		out = append(out, toAdminRequestSummary(s))
	}
	common.Respond(w, r, http.StatusOK, out, "Admin requests retrieved")
}

func (rs *Resource) getAdminRequest(w http.ResponseWriter, r *http.Request) {
	if rs.DecisionService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("decision service not configured")))
		return
	}
	id, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid id")
	if !ok {
		return
	}

	var detail AdminRequestDetail
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		s, e := rs.DecisionService.Get(ctx, id)
		if e != nil {
			return e
		}
		detail = rs.toAdminRequestDetail(ctx, s)
		return nil
	})
	if err != nil {
		if errors.Is(err, enrollmentService.ErrDecisionRequestNotFound) {
			common.RenderError(w, r, common.ErrorNotFound(err))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, detail, "Admin request retrieved")
}

func (rs *Resource) listAdminRequestsByStudent(w http.ResponseWriter, r *http.Request) {
	if rs.DecisionService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("decision service not configured")))
		return
	}
	studentID, ok := common.ParsePositiveInt64IDWithError(w, r, "studentId", "invalid studentId")
	if !ok {
		return
	}

	var out []AdminRequestSummary
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		summaries, listErr := rs.DecisionService.ListByStudent(ctx, studentID)
		if listErr != nil {
			return listErr
		}
		out = make([]AdminRequestSummary, 0, len(summaries))
		for _, summary := range summaries {
			out = append(out, rs.toAdminRequestDetailSummary(ctx, summary))
		}
		return nil
	})
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, out, "Student admin requests retrieved")
}

func (rs *Resource) toAdminRequestDetail(ctx context.Context, summary *enrollmentService.RequestSummary) AdminRequestDetail {
	detail := AdminRequestDetail{
		AdminRequestSummary: rs.toAdminRequestDetailSummary(ctx, summary),
	}
	if summary == nil || summary.Request == nil {
		return detail
	}
	detail.StatusToken = summary.Request.StatusToken
	return detail
}

func (rs *Resource) toAdminRequestDetailSummary(ctx context.Context, summary *enrollmentService.RequestSummary) AdminRequestSummary {
	detail := toAdminRequestSummary(summary)
	if summary == nil || summary.Request == nil {
		return detail
	}
	detail.CustomData = summary.Request.CustomData
	detail.ConsentFlags = summary.Request.ConsentFlags

	if rs.FormSchemaService != nil && summary.Request.SchemaID != nil {
		if fs, err := rs.FormSchemaService.GetByID(ctx, *summary.Request.SchemaID); err == nil && fs != nil {
			detail.SchemaFields, detail.SchemaLegalBlocks = toAdminSchemaMetadata(fs)
		}
	}

	if rs.DecisionService != nil {
		if childOfferings, err := rs.DecisionService.ListChildOfferings(ctx, summary.Request.ID); err == nil {
			attachChildOfferings(detail.Children, summary.Children, childOfferings)
		}
	}
	return detail
}

// attachChildOfferings fills the per-child Offerings slices in place from
// the request's care-offering rows, matching admin children to summary
// children positionally. Children beyond the summary or without rows are
// left untouched.
func attachChildOfferings(children []AdminRequestChild, summaryChildren []*enrollmentModels.RequestChild, childOfferings map[int64][]enrollmentService.ChildOfferingRow) {
	for i := range children {
		if i >= len(summaryChildren) {
			continue
		}
		rows := childOfferings[summaryChildren[i].ID]
		if len(rows) == 0 {
			continue
		}
		children[i].Offerings = toAdminChildOfferings(rows)
	}
}

func toAdminSchemaMetadata(fs *enrollmentModels.FormSchema) ([]AdminRequestSchemaField, []AdminRequestSchemaLegalBlock) {
	schemaFields := make([]AdminRequestSchemaField, 0, len(fs.Fields))
	for _, f := range fs.Fields {
		if f.Type == enrollmentModels.FormFieldInfo {
			continue
		}
		opts := make([]AdminRequestSchemaFieldOption, 0, len(f.Options))
		for _, o := range f.Options {
			opts = append(opts, AdminRequestSchemaFieldOption{Label: o.Label, Value: o.Value})
		}
		schemaFields = append(schemaFields, AdminRequestSchemaField{
			Key:            f.Key,
			Label:          f.Label,
			Type:           string(f.Type),
			AppliesToChild: f.AppliesToCh,
			Target:         f.Target,
			Options:        opts,
		})
	}
	schemaLegalBlocks := make([]AdminRequestSchemaLegalBlock, 0, len(fs.LegalBlocks))
	for _, b := range fs.LegalBlocks {
		schemaLegalBlocks = append(schemaLegalBlocks, AdminRequestSchemaLegalBlock{
			Key:   b.Key,
			Title: b.Title,
		})
	}
	return schemaFields, schemaLegalBlocks
}

// AdminDecideRequest is the body of POST .../children/{childId}/decide.
type AdminDecideRequest struct {
	Status string `json:"status"` // approved | waitlisted | rejected | under_review
	Reason string `json:"reason,omitempty"`
}

func (req *AdminDecideRequest) Bind(_ *http.Request) error { return nil }

func (rs *Resource) decideAdminChild(w http.ResponseWriter, r *http.Request) {
	if rs.DecisionService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("decision service not configured")))
		return
	}
	requestID, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid id")
	if !ok {
		return
	}
	childID, ok := common.ParsePositiveInt64IDWithError(w, r, "childId", "invalid childId")
	if !ok {
		return
	}
	body := &AdminDecideRequest{}
	if err := render.Bind(r, body); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())
	outcome, err := rs.decideChildWithRetry(r, enrollmentService.DecideInput{
		RequestID:  requestID,
		ChildID:    childID,
		Status:     enrollmentService.DecisionStatus(body.Status),
		Reason:     body.Reason,
		ReviewedBy: int64(claims.ID),
	})
	if err != nil {
		renderDecideError(w, r, err)
		return
	}

	// Post-tx: schedule guardian invitation if the approval pipeline
	// asked for one. Best-effort — failure here doesn't roll back the
	// approval (records are already committed). Logging captures the
	// failure for the admin to chase via "Re-send invitation".
	if outcome != nil && outcome.PendingInvite != nil && rs.GuardianInvitationService != nil {
		go rs.dispatchPostDecisionInvite(r.Context(), outcome.PendingInvite)
	}

	common.Respond(w, r, http.StatusOK, newAdminRequestChild(outcome.Child), "Decision applied")
}

// restoreAdminRequest undoes a parent-initiated withdraw (#2157): the
// decision service flips every withdrawn child back to submitted, clears
// withdrawn_at, and writes the append-only audit row — all inside the
// tenant transaction owned here.
func (rs *Resource) restoreAdminRequest(w http.ResponseWriter, r *http.Request) {
	if rs.DecisionService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("decision service not configured")))
		return
	}
	requestID, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid id")
	if !ok {
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())
	var outcome *enrollmentService.RestoreOutcome
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		out, e := rs.DecisionService.RestoreWithdrawn(ctx, requestID, int64(claims.ID))
		if e != nil {
			return e
		}
		outcome = out
		return nil
	})
	if err != nil {
		renderRestoreError(w, r, err)
		return
	}

	common.Respond(w, r, http.StatusOK, map[string]any{
		"restored_children":   len(outcome.RestoredChildIDs),
		"waitlisted_children": len(outcome.WaitlistedChildIDs),
	}, "Request restored")
}

// renderRestoreError maps a restore failure to its HTTP status. The two
// business guards (inactive phase, active duplicate) surface as 409s with
// stable codes so the frontend can show specific German messages.
func renderRestoreError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, context.Canceled):
		common.RenderError(w, r, common.ErrorClientClosed(err))
	case errors.Is(err, context.DeadlineExceeded):
		common.RenderError(w, r, common.ErrorRequestTimeout(err))
	case errors.Is(err, enrollmentService.ErrDecisionRequestNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, enrollmentService.ErrRestoreNothingWithdrawn):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	case errors.Is(err, enrollmentService.ErrRestorePhaseInactive):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "enrollment.restore_phase_inactive"))
	case errors.Is(err, enrollmentService.ErrRestoreDuplicateActive):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "enrollment.restore_duplicate"))
	case errors.Is(err, enrollmentService.ErrCareOfferingFull):
		// Reject-mode phase: the capacity gate refuses the restore because
		// an offering is meanwhile full. Same code the submit path uses.
		common.RenderError(w, r, common.ErrorConflictWithCode(err, ErrCodeEnrollmentCareOfferingFull))
	case errors.Is(err, enrollmentService.ErrCareOfferingClosed):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "enrollment.restore_offering_closed"))
	case common.IsTransientDatabaseError(err):
		common.RenderError(w, r, common.ErrorServiceUnavailable(err))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}

// decideChildWithRetry runs the decision inside a tenant transaction,
// retrying once on a transient database error when the decision body has
// not yet committed. A cancelled/expired request context stops the retry
// loop and surfaces the context error.
func (rs *Resource) decideChildWithRetry(r *http.Request, input enrollmentService.DecideInput) (*enrollmentService.DecideOutcome, error) {
	var err error
	var outcome *enrollmentService.DecideOutcome
	for attempt := 0; attempt < 2; attempt++ {
		outcome = nil
		decisionBodySucceeded := false
		err = rs.runInTenantTx(r, func(ctx context.Context) error {
			out, e := rs.DecisionService.Decide(ctx, input)
			if e != nil {
				return e
			}
			outcome = out
			decisionBodySucceeded = true
			return nil
		})
		if err == nil {
			break
		}
		if reqErr := r.Context().Err(); reqErr != nil {
			return outcome, reqErr
		}
		if decisionBodySucceeded || !common.IsTransientDatabaseError(err) || attempt == 1 {
			break
		}
		slog.WarnContext(r.Context(), "transient enrollment decision failure, retrying",
			slog.Int64("request_id", input.RequestID),
			slog.Int64("child_id", input.ChildID),
			slog.String("error", err.Error()),
		)
	}
	return outcome, err
}

// renderDecideError maps a decision failure to its HTTP status. Context
// cancellation/timeout and transient DB errors get their own codes so the
// frontend can distinguish a retryable failure from a terminal one.
func renderDecideError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, context.Canceled):
		common.RenderError(w, r, common.ErrorClientClosed(err))
	case errors.Is(err, context.DeadlineExceeded):
		common.RenderError(w, r, common.ErrorRequestTimeout(err))
	case errors.Is(err, enrollmentService.ErrDecisionChildNotFound),
		errors.Is(err, enrollmentService.ErrDecisionRequestNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, enrollmentService.ErrDecisionInvalidStatus),
		errors.Is(err, enrollmentService.ErrDecisionAlreadyTerminal),
		errors.Is(err, enrollmentService.ErrDecisionInvalidData):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	case errors.Is(err, enrollmentService.ErrWaitlistDisabled):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "enrollment.waitlist_disabled"))
	case errors.Is(err, enrollmentService.ErrGuardianAccountMismatch):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "enrollment.guardian_account_mismatch"))
	case common.IsTransientDatabaseError(err):
		common.RenderError(w, r, common.ErrorServiceUnavailable(err))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}

// newAdminRequestChild maps a decided child model onto the wire shape
// returned by the decide endpoint (no CustomData / Offerings — those are
// detail-endpoint concerns).
func newAdminRequestChild(child *enrollmentModels.RequestChild) AdminRequestChild {
	return AdminRequestChild{
		ID:                strconv.FormatInt(child.ID, 10),
		FirstName:         child.FirstName,
		LastName:          child.LastName,
		DateOfBirth:       child.DateOfBirth.String(),
		TargetGradeLevel:  child.TargetGradeLevel,
		TargetSchoolClass: child.TargetSchoolClass,
		Status:            child.Status,
		StatusReason:      child.StatusReason,
		ReviewedAt:        child.ReviewedAt,
		ReviewedBy:        child.ReviewedBy,
		ActivationMode:    child.ActivationMode,
		CreatedStudentID:  optionalInt64String(child.CreatedStudentID),
	}
}

type AdminUpdateOfferingsRequest struct {
	Offerings []AdminUpdateOfferingSelection `json:"offerings"`
	Reason    string                         `json:"reason"`
}

type AdminUpdateOfferingSelection struct {
	OfferingID   string   `json:"offering_id"`
	SelectedDays []string `json:"selected_days,omitempty"`
}

func (req *AdminUpdateOfferingsRequest) Bind(_ *http.Request) error { return nil }

func (rs *Resource) updateAdminChildOfferings(w http.ResponseWriter, r *http.Request) {
	if rs.DecisionService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("decision service not configured")))
		return
	}
	requestID, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid id")
	if !ok {
		return
	}
	childID, ok := common.ParsePositiveInt64IDWithError(w, r, "childId", "invalid childId")
	if !ok {
		return
	}
	body := &AdminUpdateOfferingsRequest{}
	if err := render.Bind(r, body); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	selections := make([]enrollmentService.OfferingAdjustmentSelection, 0, len(body.Offerings))
	for _, row := range body.Offerings {
		offeringID, parseErr := strconv.ParseInt(row.OfferingID, 10, 64)
		if parseErr != nil || offeringID <= 0 {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid offering_id")))
			return
		}
		selections = append(selections, enrollmentService.OfferingAdjustmentSelection{
			OfferingID:   offeringID,
			SelectedDays: row.SelectedDays,
		})
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	actorRole := actorRoleFromClaims(claims.Roles)

	var updated *enrollmentModels.RequestChild
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		child, updateErr := rs.DecisionService.UpdateChildOfferings(ctx, enrollmentService.UpdateChildOfferingsInput{
			RequestID:      requestID,
			ChildID:        childID,
			Offerings:      selections,
			Reason:         body.Reason,
			ActorAccountID: int64(claims.ID),
			ActorRole:      actorRole,
		})
		updated = child
		return updateErr
	})
	if err != nil {
		switch {
		case errors.Is(err, enrollmentService.ErrDecisionChildNotFound),
			errors.Is(err, enrollmentService.ErrDecisionRequestNotFound):
			common.RenderError(w, r, common.ErrorNotFound(err))
		case errors.Is(err, enrollmentService.ErrOfferingAdjustmentInvalid),
			errors.Is(err, enrollmentService.ErrCareOfferingClosed),
			errors.Is(err, enrollmentService.ErrRequiredCareOfferingMissing),
			errors.Is(err, enrollmentService.ErrCareOfferingMissing),
			errors.Is(err, enrollmentService.ErrCareOfferingExactlyOneRequired):
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
		case errors.Is(err, enrollmentService.ErrCareOfferingsDisabled):
			common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, ErrCodeEnrollmentCareOfferingsDisabled))
		default:
			common.RenderError(w, r, common.ErrorInternalServer(err))
		}
		return
	}
	out := AdminRequestChild{
		ID:                strconv.FormatInt(updated.ID, 10),
		FirstName:         updated.FirstName,
		LastName:          updated.LastName,
		DateOfBirth:       updated.DateOfBirth.String(),
		TargetGradeLevel:  updated.TargetGradeLevel,
		TargetSchoolClass: updated.TargetSchoolClass,
		Status:            updated.Status,
		StatusReason:      updated.StatusReason,
		ReviewedAt:        updated.ReviewedAt,
		ReviewedBy:        updated.ReviewedBy,
		ActivationMode:    updated.ActivationMode,
		CreatedStudentID:  optionalInt64String(updated.CreatedStudentID),
		CustomData:        updated.CustomData,
	}
	err = rs.runInTenantTx(r, func(ctx context.Context) error {
		rowsByChild, listErr := rs.DecisionService.ListChildOfferings(ctx, requestID)
		if listErr != nil {
			return listErr
		}
		out.Offerings = toAdminChildOfferings(rowsByChild[childID])
		return nil
	})
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, out, "Child offerings updated")
}

func optionalInt64String(value *int64) string {
	if value == nil || *value <= 0 {
		return ""
	}
	return strconv.FormatInt(*value, 10)
}

func (rs *Resource) listAdminChildOfferingAdjustments(w http.ResponseWriter, r *http.Request) {
	if rs.DecisionService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("decision service not configured")))
		return
	}
	requestID, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid id")
	if !ok {
		return
	}
	childID, ok := common.ParsePositiveInt64IDWithError(w, r, "childId", "invalid childId")
	if !ok {
		return
	}
	var rows []*auditModels.EnrollmentOfferingAdjustment
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		list, listErr := rs.DecisionService.ListOfferingAdjustments(ctx, requestID, childID)
		rows = list
		return listErr
	})
	if err != nil {
		if errors.Is(err, enrollmentService.ErrDecisionChildNotFound) {
			common.RenderError(w, r, common.ErrorNotFound(err))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	out := make([]AdminOfferingAdjustment, 0, len(rows))
	for _, row := range rows {
		out = append(out, toAdminOfferingAdjustment(row))
	}
	common.Respond(w, r, http.StatusOK, out, "Offering adjustments retrieved")
}

func toAdminChildOfferings(rows []enrollmentService.ChildOfferingRow) []AdminRequestChildOffering {
	out := make([]AdminRequestChildOffering, 0, len(rows))
	for _, row := range rows {
		out = append(out, AdminRequestChildOffering{
			OfferingID:            strconv.FormatInt(row.OfferingID, 10),
			OfferingName:          row.OfferingName,
			DaysOfWeekMode:        row.DaysOfWeekMode,
			SelectedDays:          row.SelectedDays,
			ManualSelectedDays:    row.ManualSelectedDays,
			AutomaticSelectedDays: row.AutomaticSelectedDays,
			AvailableDays:         row.AvailableDays,
		})
	}
	return out
}

func toAdminOfferingAdjustment(row *auditModels.EnrollmentOfferingAdjustment) AdminOfferingAdjustment {
	return AdminOfferingAdjustment{
		ID:                 strconv.FormatInt(row.ID, 10),
		RequestID:          strconv.FormatInt(row.RequestID, 10),
		RequestChildID:     strconv.FormatInt(row.RequestChildID, 10),
		StudentID:          strconv.FormatInt(row.StudentID, 10),
		ActorAccountID:     strconv.FormatInt(row.ActorAccountID, 10),
		ActorRole:          row.ActorRole,
		ActorNameSnapshot:  row.ActorNameSnapshot,
		ActorEmailSnapshot: row.ActorEmailSnapshot,
		Reason:             row.Reason,
		Before:             row.Before,
		After:              row.After,
		ChangedAt:          row.ChangedAt,
	}
}

func actorRoleFromClaims(roles []string) string {
	for _, role := range roles {
		trimmed := strings.TrimSpace(role)
		if trimmed != "" {
			return trimmed
		}
	}
	return "admin"
}

// dispatchPostDecisionInvite fires the guardian invitation flow after
// the approval tx commits. Runs in its own goroutine so the HTTP
// response isn't blocked on SMTP / outbox writes; the invitation
// service writes to platform.email_outbox synchronously, then the
// outbox worker dispatches the email asynchronously on its own tick.
func (rs *Resource) dispatchPostDecisionInvite(parentCtx context.Context, invite *enrollmentService.PendingGuardianInvite) {
	// Detach from request lifetime so the goroutine isn't cancelled by
	// the response writer flushing. Re-attach tenant from the parent so
	// the invitation service's tenant-scoped writes resolve.
	tenantID := tenant.FromContext(parentCtx)
	bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	bgCtx = tenant.WithTenantID(bgCtx, tenantID)

	// Wrap in a tenant tx so the invitation service's repo writes pick
	// up the right RLS scope. The service has its own RunInTx that
	// reuses an existing tx context, so this stays a single tx.
	err := tenant.WithTenantTx(bgCtx, rs.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, e := rs.GuardianInvitationService.Create(txCtx, authService.GuardianInvitationCreateRequest{
			GuardianProfileID: invite.GuardianProfileID,
			CreatedBy:         invite.CreatedBy,
		})
		return e
	})
	if err != nil {
		slog.Default().Warn("post-decision guardian invitation failed",
			slog.Int64("guardian_profile_id", invite.GuardianProfileID),
			slog.Int64("created_by", invite.CreatedBy),
			slog.String("error", err.Error()))
	}
}
