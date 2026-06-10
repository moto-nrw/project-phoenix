package enrollment

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// AdminRequestSummary is the wire shape for the admin list page.
// Carries the request + per-child overview + the phase name so the
// list can render without a second fetch.
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
	StatusToken       string                    `json:"status_token"`
	CustomData        map[string]any            `json:"custom_data,omitempty"`
	ConsentFlags      map[string]any            `json:"consent_flags,omitempty"`
	SchemaFields      []AdminRequestSchemaField `json:"schema_fields,omitempty"`
	Children          []AdminRequestChild       `json:"children"`
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
	ID               string         `json:"id"`
	FirstName        string         `json:"first_name"`
	LastName         string         `json:"last_name"`
	DateOfBirth      string         `json:"date_of_birth"`
	TargetGradeLevel *int16         `json:"target_grade_level,omitempty"`
	Status           string         `json:"status"`
	StatusReason     *string        `json:"status_reason,omitempty"`
	ReviewedAt       *time.Time     `json:"reviewed_at,omitempty"`
	ReviewedBy       *int64         `json:"reviewed_by,omitempty"`
	ActivationMode   string         `json:"activation_mode"`
	CustomData       map[string]any `json:"custom_data,omitempty"`
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
	OfferingID     string   `json:"offering_id"`
	OfferingName   string   `json:"offering_name"`
	DaysOfWeekMode string   `json:"days_of_week_mode"`
	SelectedDays   []string `json:"selected_days,omitempty"`
	AvailableDays  []string `json:"available_days,omitempty"`
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
		StatusToken:       s.Request.StatusToken,
	}
	if s.Phase != nil {
		out.PhaseName = s.Phase.Name
	}
	out.Children = make([]AdminRequestChild, 0, len(s.Children))
	for _, c := range s.Children {
		out.Children = append(out.Children, AdminRequestChild{
			ID:               strconv.FormatInt(c.ID, 10),
			FirstName:        c.FirstName,
			LastName:         c.LastName,
			DateOfBirth:      c.DateOfBirth.String(),
			TargetGradeLevel: c.TargetGradeLevel,
			Status:           c.Status,
			StatusReason:     c.StatusReason,
			ReviewedAt:       c.ReviewedAt,
			ReviewedBy:       c.ReviewedBy,
			ActivationMode:   c.ActivationMode,
			CustomData:       c.CustomData,
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
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid id")))
		return
	}

	var summary *enrollmentService.RequestSummary
	var schemaFields []AdminRequestSchemaField
	var childOfferings map[int64][]enrollmentService.ChildOfferingRow
	err = rs.runInTenantTx(r, func(ctx context.Context) error {
		s, e := rs.DecisionService.Get(ctx, id)
		if e != nil {
			return e
		}
		summary = s
		// Per-child offerings — best-effort; failure here doesn't kill
		// the detail response, the admin just sees no Betreuungsangebote
		// section.
		if off, oerr := rs.DecisionService.ListChildOfferings(ctx, id); oerr == nil {
			childOfferings = off
		}
		// Pull the form schema so the response can carry labels +
		// targets alongside the raw custom_data values. Best-effort:
		// a missing schema falls back to rendering raw keys on the
		// frontend.
		if rs.FormSchemaService != nil && s.Request != nil && s.Request.SchemaID != nil {
			fs, ferr := rs.FormSchemaService.GetByID(ctx, *s.Request.SchemaID)
			if ferr == nil && fs != nil {
				schemaFields = make([]AdminRequestSchemaField, 0, len(fs.Fields))
				for _, f := range fs.Fields {
					// Information blocks collect no answer — omit them
					// from the admin's per-submission field list.
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
			}
		}
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
	detail := toAdminRequestSummary(summary)
	detail.CustomData = summary.Request.CustomData
	detail.ConsentFlags = summary.Request.ConsentFlags
	detail.SchemaFields = schemaFields
	// Stitch per-child offerings onto the detail-response children.
	// childOfferings is keyed by request_child.id as a raw int64; the
	// summary's children carry their id as the original int64 too —
	// just need to format it the same way the DTO stringifies the id.
	if childOfferings != nil {
		for i := range detail.Children {
			cid := summary.Children[i].ID
			rows := childOfferings[cid]
			if len(rows) == 0 {
				continue
			}
			out := make([]AdminRequestChildOffering, 0, len(rows))
			for _, row := range rows {
				out = append(out, AdminRequestChildOffering{
					OfferingID:     strconv.FormatInt(row.OfferingID, 10),
					OfferingName:   row.OfferingName,
					DaysOfWeekMode: row.DaysOfWeekMode,
					SelectedDays:   row.SelectedDays,
					AvailableDays:  row.AvailableDays,
				})
			}
			detail.Children[i].Offerings = out
		}
	}
	common.Respond(w, r, http.StatusOK, detail, "Admin request retrieved")
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
	requestID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || requestID <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid id")))
		return
	}
	childID, err := strconv.ParseInt(chi.URLParam(r, "childId"), 10, 64)
	if err != nil || childID <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid childId")))
		return
	}
	body := &AdminDecideRequest{}
	if err := render.Bind(r, body); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())
	reviewedBy := int64(claims.ID)

	var outcome *enrollmentService.DecideOutcome
	err = rs.runInTenantTx(r, func(ctx context.Context) error {
		out, e := rs.DecisionService.Decide(ctx, enrollmentService.DecideInput{
			RequestID:  requestID,
			ChildID:    childID,
			Status:     enrollmentService.DecisionStatus(body.Status),
			Reason:     body.Reason,
			ReviewedBy: reviewedBy,
		})
		outcome = out
		return e
	})
	if err != nil {
		switch {
		case errors.Is(err, enrollmentService.ErrDecisionChildNotFound),
			errors.Is(err, enrollmentService.ErrDecisionRequestNotFound):
			common.RenderError(w, r, common.ErrorNotFound(err))
		case errors.Is(err, enrollmentService.ErrDecisionInvalidStatus),
			errors.Is(err, enrollmentService.ErrDecisionAlreadyTerminal),
			errors.Is(err, enrollmentService.ErrDecisionInvalidData):
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
		default:
			common.RenderError(w, r, common.ErrorInternalServer(err))
		}
		return
	}

	// Post-tx: schedule guardian invitation if the approval pipeline
	// asked for one. Best-effort — failure here doesn't roll back the
	// approval (records are already committed). Logging captures the
	// failure for the admin to chase via "Re-send invitation".
	if outcome != nil && outcome.PendingInvite != nil && rs.GuardianInvitationService != nil {
		go rs.dispatchPostDecisionInvite(r.Context(), outcome.PendingInvite)
	}

	updated := outcome.Child
	common.Respond(w, r, http.StatusOK, AdminRequestChild{
		ID:               strconv.FormatInt(updated.ID, 10),
		FirstName:        updated.FirstName,
		LastName:         updated.LastName,
		DateOfBirth:      updated.DateOfBirth.String(),
		TargetGradeLevel: updated.TargetGradeLevel,
		Status:           updated.Status,
		StatusReason:     updated.StatusReason,
		ReviewedAt:       updated.ReviewedAt,
		ReviewedBy:       updated.ReviewedBy,
		ActivationMode:   updated.ActivationMode,
	}, "Decision applied")
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
