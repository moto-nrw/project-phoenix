package students

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	userService "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// "Betreuung beenden" (#2487). The whole surface sits behind users:delete —
// the same permission the permanent deletion needs — because ending a care
// relationship and reading why it ended are the two halves of the same
// sensitive decision.

// careExitRequest is the body of both the preview and the confirmation. The
// confirmation additionally quotes the preview token back.
type careExitRequest struct {
	StudentIDs  []string `json:"student_ids"`
	LastCareDay string   `json:"last_care_day"`
	Reason      string   `json:"reason"`
	ReasonNote  string   `json:"reason_note"`
	Token       string   `json:"token"`
}

type careWithdrawalExitRequest struct {
	LastCareDay string `json:"last_care_day"`
	Reason      string `json:"reason"`
	ReasonNote  string `json:"reason_note"`
	Token       string `json:"token"`
}

func (b *careWithdrawalExitRequest) Bind(_ *http.Request) error {
	if strings.TrimSpace(b.LastCareDay) == "" {
		return errors.New("Bitte geben Sie den letzten Betreuungstag an.") //nolint:staticcheck // user-facing German message
	}
	return nil
}

func (b *careWithdrawalExitRequest) toInput() (userService.CareExitInput, error) {
	day, err := timezone.ParseDate(strings.TrimSpace(b.LastCareDay))
	if err != nil {
		return userService.CareExitInput{}, errors.New("Bitte geben Sie den letzten Betreuungstag im Format JJJJ-MM-TT an.") //nolint:staticcheck // user-facing German message
	}
	return userService.CareExitInput{
		LastCareDay: day, Reason: strings.TrimSpace(b.Reason), ReasonNote: strings.TrimSpace(b.ReasonNote),
	}, nil
}

func (b *careExitRequest) Bind(_ *http.Request) error {
	if len(b.StudentIDs) == 0 {
		return userService.ErrCareExitNoStudents
	}
	if strings.TrimSpace(b.LastCareDay) == "" {
		return errors.New("Bitte geben Sie den letzten Betreuungstag an.") //nolint:staticcheck // ST1005: user-facing German message
	}
	return nil
}

// toInput parses the wire shape into the service input. IDs arrive as strings
// (backend int64 → frontend string, CLAUDE.md rule 3).
func (b *careExitRequest) toInput() (userService.CareExitInput, error) {
	ids := make([]int64, 0, len(b.StudentIDs))
	for _, raw := range b.StudentIDs {
		id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err != nil || id <= 0 {
			return userService.CareExitInput{}, errors.New("Die Auswahl enthält ein ungültiges Kind.") //nolint:staticcheck // ST1005: user-facing German message
		}
		ids = append(ids, id)
	}
	day, err := timezone.ParseDate(strings.TrimSpace(b.LastCareDay))
	if err != nil {
		return userService.CareExitInput{}, errors.New("Bitte geben Sie den letzten Betreuungstag im Format JJJJ-MM-TT an.") //nolint:staticcheck // ST1005: user-facing German message
	}
	return userService.CareExitInput{
		StudentIDs:  ids,
		LastCareDay: day,
		Reason:      strings.TrimSpace(b.Reason),
		ReasonNote:  strings.TrimSpace(b.ReasonNote),
	}, nil
}

// careExitImpactResponse is one child in the preview, named.
type careExitImpactResponse struct {
	StudentID          string                              `json:"student_id"`
	FirstName          string                              `json:"first_name"`
	LastName           string                              `json:"last_name"`
	SchoolClass        string                              `json:"school_class,omitempty"`
	PlannedRosterRows  int                                 `json:"planned_roster_rows"`
	ActivityBookings   int                                 `json:"activity_bookings"`
	OpenParentRequests int                                 `json:"open_parent_requests"`
	HasRFIDTag         bool                                `json:"has_rfid_tag"`
	CurrentlyPresent   bool                                `json:"currently_present"`
	SourceOfferings    []userModels.CareExitSourceOffering `json:"source_offerings"`
	WeeklyPlans        []string                            `json:"weekly_plans"`
	PlannedEndsOn      *string                             `json:"planned_ends_on,omitempty"`
	Blocker            string                              `json:"blocker,omitempty"`
}

type careExitPreviewResponse struct {
	Token       string                   `json:"token"`
	LastCareDay string                   `json:"last_care_day"`
	Reason      string                   `json:"reason"`
	ReasonNote  string                   `json:"reason_note,omitempty"`
	Blocked     bool                     `json:"blocked"`
	Students    []careExitImpactResponse `json:"students"`
}

func toCareExitPreviewResponse(preview *userService.CareExitPreview) careExitPreviewResponse {
	students := make([]careExitImpactResponse, 0, len(preview.Students))
	for _, impact := range preview.Students {
		row := careExitImpactResponse{
			StudentID:          strconv.FormatInt(impact.StudentID, 10),
			FirstName:          impact.FirstName,
			LastName:           impact.LastName,
			SchoolClass:        impact.SchoolClass,
			PlannedRosterRows:  impact.PlannedRosterRows,
			ActivityBookings:   impact.ActivityBookings,
			OpenParentRequests: impact.OpenParentRequests,
			HasRFIDTag:         impact.HasRFIDTag,
			CurrentlyPresent:   impact.CurrentlyPresent,
			SourceOfferings:    impact.SourceOfferings,
			WeeklyPlans:        impact.WeeklyPlans,
			Blocker:            impact.Blocker,
		}
		if impact.PlannedEndsOn != nil {
			value := impact.PlannedEndsOn.String()
			row.PlannedEndsOn = &value
		}
		students = append(students, row)
	}
	return careExitPreviewResponse{
		Token:       preview.Token,
		LastCareDay: preview.LastCareDay.String(),
		Reason:      preview.Reason,
		ReasonNote:  preview.ReasonNote,
		Blocked:     preview.Blocked,
		Students:    students,
	}
}

// previewCareExit answers "what happens if we end the care of these children
// on this day" without changing anything.
func (rs *Resource) previewCareExit(w http.ResponseWriter, r *http.Request) {
	if rs.CareLifecycleService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("care lifecycle service not configured")))
		return
	}
	body := new(careExitRequest)
	if err := render.Bind(r, body); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	input, err := body.toInput()
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	preview, err := rs.CareLifecycleService.Preview(r.Context(), input)
	if err != nil {
		renderError(w, r, careExitErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, toCareExitPreviewResponse(preview), "Vorschau erstellt")
}

// confirmCareExit ends the care for exactly the previewed state, or changes
// nothing at all.
func (rs *Resource) confirmCareExit(w http.ResponseWriter, r *http.Request) {
	if rs.CareLifecycleService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("care lifecycle service not configured")))
		return
	}
	body := new(careExitRequest)
	if err := render.Bind(r, body); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	input, err := body.toInput()
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	actorID := int64(jwt.ClaimsFromCtx(r.Context()).ID)
	result, err := rs.CareLifecycleService.Confirm(r.Context(), body.Token, input, actorID)
	if err != nil {
		renderError(w, r, careExitErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]any{
		"students_ended":      result.StudentsEnded,
		"roster_rows_removed": result.RosterRowsRemoved,
		"bookings_ended":      result.BookingsEnded,
	}, "Betreuung beendet")
}

type careWithdrawalResponse struct {
	ID                  string `json:"id"`
	StudentID           string `json:"student_id,omitempty"`
	FirstName           string `json:"first_name,omitempty"`
	LastName            string `json:"last_name,omitempty"`
	SchoolClass         string `json:"school_class,omitempty"`
	FirstBookinglessDay string `json:"first_bookingless_day"`
	Urgency             string `json:"urgency"`
	State               string `json:"state"`
	Outcome             string `json:"outcome,omitempty"`
	ResolvedAt          string `json:"resolved_at,omitempty"`
}

func toCareWithdrawalResponse(row *userModels.CareWithdrawalCompletion) careWithdrawalResponse {
	studentID := ""
	if row.StudentID != nil {
		studentID = strconv.FormatInt(*row.StudentID, 10)
	}
	response := careWithdrawalResponse{
		ID: strconv.FormatInt(row.ID, 10), StudentID: studentID,
		FirstName: row.FirstName, LastName: row.LastName, SchoolClass: row.SchoolClass,
		FirstBookinglessDay: row.FirstBookinglessDay.String(), Urgency: row.UrgencyOn(timezone.TodayDate()), State: row.State,
	}
	if row.Outcome != nil {
		response.Outcome = *row.Outcome
	}
	if row.ResolvedAt != nil {
		response.ResolvedAt = row.ResolvedAt.Format(time.RFC3339)
	}
	return response
}

func (rs *Resource) listCareWithdrawals(w http.ResponseWriter, r *http.Request) {
	if rs.CareLifecycleService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("care lifecycle service not configured")))
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	studentID, err := optionalPositiveQueryID(r, "student_id")
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	filter := userModels.CareWithdrawalCompletionFilter{
		Search: r.URL.Query().Get("search"), StudentID: studentID, Page: page, PageSize: pageSize,
	}.Normalized()
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	var rows []*userModels.CareWithdrawalCompletion
	var total int
	if state == "" || state == userModels.CareWithdrawalStatePending {
		rows, total, err = rs.CareLifecycleService.ListPendingWithdrawals(r.Context(), filter)
	} else if state == userModels.CareWithdrawalStateResolved {
		rows, total, err = rs.CareLifecycleService.ListResolvedWithdrawals(r.Context(), filter)
	} else {
		renderError(w, r, common.ErrorInvalidRequest(errors.New("state must be pending or resolved")))
		return
	}
	if err != nil {
		renderError(w, r, careExitErrorRenderer(err))
		return
	}
	out := make([]careWithdrawalResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, toCareWithdrawalResponse(row))
	}
	common.Respond(w, r, http.StatusOK, map[string]any{
		"items": out, "total": total, "page": filter.Page, "page_size": filter.PageSize,
	}, "Offene Abmeldungen geladen")
}

func optionalPositiveQueryID(r *http.Request, key string) (int64, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return 0, nil
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}
	return id, nil
}

func (rs *Resource) previewWithdrawalCareEnd(w http.ResponseWriter, r *http.Request) {
	if rs.CareLifecycleService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("care lifecycle service not configured")))
		return
	}
	id, ok := common.ParsePositiveInt64IDWithError(w, r, "completionId", "invalid completion ID")
	if !ok {
		return
	}
	body := new(careWithdrawalExitRequest)
	if err := render.Bind(r, body); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	input, err := body.toInput()
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	preview, err := rs.CareLifecycleService.PreviewWithdrawalCareEnd(r.Context(), id, input)
	if err != nil {
		renderError(w, r, careExitErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, toCareExitPreviewResponse(preview), "Vorschau erstellt")
}

func (rs *Resource) confirmWithdrawalCareEnd(w http.ResponseWriter, r *http.Request) {
	if rs.CareLifecycleService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("care lifecycle service not configured")))
		return
	}
	id, ok := common.ParsePositiveInt64IDWithError(w, r, "completionId", "invalid completion ID")
	if !ok {
		return
	}
	body := new(careWithdrawalExitRequest)
	if err := render.Bind(r, body); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	input, err := body.toInput()
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	result, err := rs.CareLifecycleService.ConfirmWithdrawalCareEnd(
		r.Context(), id, body.Token, input, int64(jwt.ClaimsFromCtx(r.Context()).ID),
	)
	if err != nil {
		renderError(w, r, careExitErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]any{
		"students_ended": result.StudentsEnded, "roster_rows_removed": result.RosterRowsRemoved,
		"bookings_ended": result.BookingsEnded,
	}, "Betreuung beendet")
}

func (rs *Resource) previewWithdrawalDeletion(w http.ResponseWriter, r *http.Request) {
	if rs.CareLifecycleService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("care lifecycle service not configured")))
		return
	}
	id, ok := common.ParsePositiveInt64IDWithError(w, r, "completionId", "invalid completion ID")
	if !ok {
		return
	}
	if _, ok := rs.authorizeWithdrawalDeletion(w, r, id); !ok {
		return
	}
	impact, err := rs.CareLifecycleService.PreviewWithdrawalDeletion(r.Context(), id)
	if err != nil {
		renderError(w, r, withdrawalDeletionErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, toStudentDeleteImpactResponse(impact), "Auswirkungen der Löschung geladen")
}

func (rs *Resource) deleteWithdrawalStudent(w http.ResponseWriter, r *http.Request) {
	if rs.CareLifecycleService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("care lifecycle service not configured")))
		return
	}
	id, ok := common.ParsePositiveInt64IDWithError(w, r, "completionId", "invalid completion ID")
	if !ok {
		return
	}
	body := new(studentDeleteRequest)
	if err := render.Bind(r, body); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	completion, ok := rs.authorizeWithdrawalDeletion(w, r, id)
	if !ok {
		return
	}
	studentID := *completion.StudentID
	result, err := rs.CareLifecycleService.DeleteWithdrawal(r.Context(), id, userService.StudentDeletionInput{
		ActorAccountID:      int64(jwt.ClaimsFromCtx(r.Context()).ID),
		ExpectedFingerprint: body.ExpectedFingerprint,
		ConfirmationName:    body.ConfirmationName,
		Reason:              body.Reason,
		Acknowledged:        body.Acknowledged,
	})
	if err != nil {
		// The route middleware owns the ambient transaction. The service cannot
		// roll it back itself when the delete re-check fails.
		tenant.MarkRollback(r.Context())
		renderError(w, r, withdrawalDeletionErrorRenderer(err))
		return
	}
	if rs.StudentPhotos != nil {
		rs.StudentPhotos.ScheduleUnlinkAfterCommit(r.Context(), result.PhotoPath)
	}
	if len(result.CompanionIDs) > 0 {
		tenantID := tenant.FromContext(r.Context())
		tenant.RegisterAfterCommit(r.Context(), func() {
			rs.broadcastStudentCompanionsChanged(tenantID, studentID)
		})
	}
	common.Respond(w, r, http.StatusOK, nil, "Kind und verknüpfte Daten gelöscht")
}

func (rs *Resource) authorizeWithdrawalDeletion(
	w http.ResponseWriter,
	r *http.Request,
	completionID int64,
) (*userModels.CareWithdrawalCompletion, bool) {
	completion, err := rs.CareLifecycleService.GetPendingWithdrawal(r.Context(), completionID)
	if err != nil {
		renderError(w, r, withdrawalDeletionErrorRenderer(err))
		return nil, false
	}
	students, err := rs.StudentService.GetByIDs(r.Context(), []int64{*completion.StudentID})
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return nil, false
	}
	student := students[*completion.StudentID]
	if student == nil {
		renderError(w, r, common.ErrorNotFound(userService.ErrCareWithdrawalNotFound))
		return nil, false
	}
	authorized, authErr := canDeleteStudent(
		r.Context(), jwt.PermissionsFromCtx(r.Context()), student, rs.UserContextService,
	)
	if !authorized {
		renderError(w, r, common.ErrorForbidden(authErr))
		return nil, false
	}
	return completion, true
}

type careExitCancelRequest struct {
	StudentIDs []string `json:"student_ids"`
}

func (b *careExitCancelRequest) Bind(_ *http.Request) error {
	if len(b.StudentIDs) == 0 {
		return userService.ErrCareExitNoStudents
	}
	return nil
}

// cancelCareExit withdraws exits that have not taken effect yet.
func (rs *Resource) cancelCareExit(w http.ResponseWriter, r *http.Request) {
	if rs.CareLifecycleService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("care lifecycle service not configured")))
		return
	}
	body := new(careExitCancelRequest)
	if err := render.Bind(r, body); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	ids := make([]int64, 0, len(body.StudentIDs))
	for _, raw := range body.StudentIDs {
		id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err != nil || id <= 0 {
			renderError(w, r, common.ErrorInvalidRequest(errors.New("student_ids enthält einen ungültigen Wert"))) //nolint:staticcheck // ST1005: user-facing German message
			return
		}
		ids = append(ids, id)
	}
	actorID := int64(jwt.ClaimsFromCtx(r.Context()).ID)
	cancelled, err := rs.CareLifecycleService.Cancel(r.Context(), ids, actorID)
	if err != nil {
		renderError(w, r, careExitErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]any{"students_cancelled": cancelled}, "Geplantes Ende storniert")
}

type careResumeRequest struct {
	NewStart string `json:"new_start"`
	Checked  bool   `json:"checked"`
}

func (b *careResumeRequest) Bind(_ *http.Request) error {
	if strings.TrimSpace(b.NewStart) == "" {
		return errors.New("Bitte geben Sie den neuen Beginn an.") //nolint:staticcheck // ST1005: user-facing German message
	}
	if !b.Checked {
		return userService.ErrCareResumeNotChecked
	}
	return nil
}

// resumeCare reopens one child's care from a new start day.
func (rs *Resource) resumeCare(w http.ResponseWriter, r *http.Request) {
	if rs.CareLifecycleService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("care lifecycle service not configured")))
		return
	}
	studentID, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid student ID")
	if !ok {
		return
	}
	body := new(careResumeRequest)
	if err := render.Bind(r, body); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	start, parseErr := timezone.ParseDate(strings.TrimSpace(body.NewStart))
	if parseErr != nil {
		renderError(w, r, common.ErrorInvalidRequest(errors.New("Bitte geben Sie den neuen Beginn im Format JJJJ-MM-TT an."))) //nolint:staticcheck // ST1005: user-facing German message
		return
	}
	err := rs.CareLifecycleService.Resume(r.Context(), userService.CareResumeInput{
		StudentID:      studentID,
		NewStart:       start,
		ActorAccountID: int64(jwt.ClaimsFromCtx(r.Context()).ID),
		Checked:        body.Checked,
	})
	if err != nil {
		renderError(w, r, careExitErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]any{"resumed": true, "new_start": start.String()}, "Betreuung wieder aufgenommen")
}

type endedCareResponse struct {
	StudentID   string  `json:"student_id"`
	FirstName   string  `json:"first_name"`
	LastName    string  `json:"last_name"`
	SchoolClass string  `json:"school_class,omitempty"`
	LastCareDay string  `json:"last_care_day"`
	Reason      *string `json:"reason,omitempty"`
	ReasonNote  *string `json:"reason_note,omitempty"`
	RecordedAt  *string `json:"recorded_at,omitempty"`
}

// listEndedCare is the "Beendete Betreuungen" archive. It holds every
// regularly ended care — manually ended, ended because an enrollment phase ran
// out, or ended by a later guided close-out — because all three write the same
// enrollment interval. Jahrgangs-Abgänge are NOT here: they live in the grade
// transition's own Abgänge view.
func (rs *Resource) listEndedCare(w http.ResponseWriter, r *http.Request) {
	if rs.CareLifecycleService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("care lifecycle service not configured")))
		return
	}
	filter := userModels.CareExitListFilter{
		Search:        strings.TrimSpace(r.URL.Query().Get("search")),
		SchoolClasses: parseMultiValueParam(r.URL.Query()["school_class"]),
		Page:          1,
		PageSize:      50,
	}
	if raw := r.URL.Query().Get("page"); raw != "" {
		if page, err := strconv.Atoi(raw); err == nil && page > 0 {
			filter.Page = page
		}
	}
	if raw := r.URL.Query().Get("page_size"); raw != "" {
		if size, err := strconv.Atoi(raw); err == nil && size > 0 && size <= 200 {
			filter.PageSize = size
		}
	}

	rows, total, err := rs.CareLifecycleService.ListEnded(r.Context(), filter)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	items := make([]endedCareResponse, 0, len(rows))
	for _, row := range rows {
		item := endedCareResponse{
			StudentID:   strconv.FormatInt(row.StudentID, 10),
			FirstName:   row.FirstName,
			LastName:    row.LastName,
			SchoolClass: row.SchoolClass,
			LastCareDay: row.LastCareDay.String(),
			Reason:      row.Reason,
			ReasonNote:  row.ReasonNote,
		}
		if row.RecordedAt != nil {
			value := row.RecordedAt.String()
			item.RecordedAt = &value
		}
		items = append(items, item)
	}
	common.Respond(w, r, http.StatusOK, map[string]any{
		"items":     items,
		"total":     total,
		"page":      filter.Page,
		"page_size": filter.PageSize,
	}, "Beendete Betreuungen")
}

// careExitErrorRenderer classifies the care-lifecycle sentinels. Everything
// here carries a German message meant for the person who pressed the button,
// so the renderers pass the error through rather than replacing its text.
var careExitErrorRenderer = common.RulesRenderer([]common.ErrorRule{
	{Target: userService.ErrCareExitNoStudents, Render: common.ErrorInvalidRequest},
	{Target: userService.ErrCareExitTooManyStudents, Render: common.ErrorInvalidRequest},
	{Target: userService.ErrCareExitDayInPast, Render: common.ErrorInvalidRequest},
	{Target: userService.ErrCareExitPreviewChanged, Render: common.ErrorConflict},
	{Target: userService.ErrCareExitBlocked, Render: common.ErrorConflict},
	{Target: userService.ErrCareExitNotPlanned, Render: common.ErrorConflict},
	{Target: userService.ErrCareExitAlreadyEffective, Render: common.ErrorConflict},
	{Target: userService.ErrCareResumeNotEnded, Render: common.ErrorConflict},
	{Target: userService.ErrCareResumeMissing, Render: common.ErrorConflict},
	{Target: userService.ErrCareResumeStartInPast, Render: common.ErrorInvalidRequest},
	{Target: userService.ErrCareResumeNotChecked, Render: common.ErrorInvalidRequest},
	{Target: userService.ErrCareWithdrawalNotFound, Render: common.ErrorNotFound},
	{Target: userService.ErrCareWithdrawalAfterGap, Render: common.ErrorInvalidRequest},
	{Target: userModels.ErrCareWithdrawalAlreadyResolved, Render: common.ErrorConflict},
	{Match: func(err error) bool {
		var dateErr *userService.CareWithdrawalDateError
		return errors.As(err, &dateErr)
	}, Render: common.ErrorInvalidRequest},
	{Target: userModels.ErrCareExitInvalidReason, Render: common.ErrorInvalidRequest},
	{Target: userModels.ErrCareExitNoteRequired, Render: common.ErrorInvalidRequest},
	{Target: userModels.ErrCareExitNoteNotAllowed, Render: common.ErrorInvalidRequest},
	{Target: userModels.ErrCareExitNoteTooLong, Render: common.ErrorInvalidRequest},
}, func(cause error) render.Renderer {
	// Alles Unerwartete (DB-Fehler, Sperren, Timeouts) bekommt EINEN ruhigen
	// Satz. Der Dialog zeigt diese Meldung wörtlich an, und interne Details
	// gehören dort nicht hin — die volle Fehlerkette geht an slog und Sentry.
	return common.ErrorInternalServerWrap(
		"Die Betreuung wurde nicht beendet. Bitte versuchen Sie es noch einmal.",
		cause,
	)
})

func withdrawalDeletionErrorRenderer(err error) render.Renderer {
	if errors.Is(err, userService.ErrCareWithdrawalNotFound) || errors.Is(err, userModels.ErrCareWithdrawalAlreadyResolved) {
		return careExitErrorRenderer(err)
	}
	return studentDeletionErrorRenderer(err)
}
