package careplanhttp

import (
	errors "errors"
	fmt "fmt"
	http "net/http"
	strconv "strconv"
	strings "strings"
	time "time"

	render "github.com/go-chi/render"
	common "github.com/moto-nrw/project-phoenix/api/common"
	careplan "github.com/moto-nrw/project-phoenix/modules/careplan"
	sessiontoken "github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	securityruntime "github.com/moto-nrw/project-phoenix/modules/securityruntime"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

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
		return errors.New("bitte geben Sie den letzten Betreuungstag an")
	}
	return nil
}

func (b *careWithdrawalExitRequest) toInput() (careplan.CareExitInput, error) {
	day, err := calendar.ParseDate(strings.TrimSpace(b.LastCareDay))
	if err != nil {
		return careplan.CareExitInput{}, errors.New("bitte geben Sie den letzten Betreuungstag im Format JJJJ-MM-TT an")
	}
	return careplan.CareExitInput{
		LastCareDay: day, Reason: strings.TrimSpace(b.Reason), ReasonNote: strings.TrimSpace(b.ReasonNote),
	}, nil
}

func (b *careExitRequest) Bind(_ *http.Request) error {
	if len(b.StudentIDs) == 0 {
		return careplan.ErrCareExitNoStudents
	}
	if strings.TrimSpace(b.LastCareDay) == "" {
		return errors.New("bitte geben Sie den letzten Betreuungstag an")
	}
	return nil
}

// toInput parses the wire shape into the service input. IDs arrive as strings
// (backend int64 → frontend string, CLAUDE.md rule 3).
func (b *careExitRequest) toInput() (careplan.CareExitInput, error) {
	ids := make([]int64, 0, len(b.StudentIDs))
	for _, raw := range b.StudentIDs {
		id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err != nil || id <= 0 {
			return careplan.CareExitInput{}, errors.New("die Auswahl enthält ein ungültiges Kind")
		}
		ids = append(ids, id)
	}
	day, err := calendar.ParseDate(strings.TrimSpace(b.LastCareDay))
	if err != nil {
		return careplan.CareExitInput{}, errors.New("bitte geben Sie den letzten Betreuungstag im Format JJJJ-MM-TT an")
	}
	return careplan.CareExitInput{
		StudentIDs:  ids,
		LastCareDay: day,
		Reason:      strings.TrimSpace(b.Reason),
		ReasonNote:  strings.TrimSpace(b.ReasonNote),
	}, nil
}

// careExitImpactResponse is one child in the preview, named.
type careExitImpactResponse struct {
	StudentID          string                            `json:"student_id"`
	FirstName          string                            `json:"first_name"`
	LastName           string                            `json:"last_name"`
	SchoolClass        string                            `json:"school_class,omitempty"`
	PlannedRosterRows  int                               `json:"planned_roster_rows"`
	ActivityBookings   int                               `json:"activity_bookings"`
	OpenParentRequests int                               `json:"open_parent_requests"`
	HasRFIDTag         bool                              `json:"has_rfid_tag"`
	CurrentlyPresent   bool                              `json:"currently_present"`
	SourceOfferings    []careplan.CareExitSourceOffering `json:"source_offerings"`
	WeeklyPlans        []string                          `json:"weekly_plans"`
	PlannedEndsOn      *string                           `json:"planned_ends_on,omitempty"`
	Blocker            string                            `json:"blocker,omitempty"`
}

type careExitPreviewResponse struct {
	Token       string                   `json:"token"`
	LastCareDay string                   `json:"last_care_day"`
	Reason      string                   `json:"reason"`
	ReasonNote  string                   `json:"reason_note,omitempty"`
	Blocked     bool                     `json:"blocked"`
	Students    []careExitImpactResponse `json:"students"`
}

func toCareExitPreviewResponse(preview *careplan.CareExitPreview) careExitPreviewResponse {
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
	actorID := int64(sessiontoken.ClaimsFromCtx(r.Context()).ID)
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

func toCareWithdrawalResponse(row careplan.WithdrawalCompletion) careWithdrawalResponse {
	studentID := ""
	if row.StudentID != nil {
		studentID = strconv.FormatInt(*row.StudentID, 10)
	}
	response := careWithdrawalResponse{
		ID: strconv.FormatInt(row.ID, 10), StudentID: studentID,
		FirstName: row.FirstName, LastName: row.LastName, SchoolClass: row.SchoolClass,
		FirstBookinglessDay: row.FirstBookinglessDay.String(), Urgency: row.UrgencyOn(calendar.TodayDate()), State: row.State,
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
	filter := careplan.CareWithdrawalFilter{
		Search: r.URL.Query().Get("search"), StudentID: studentID, Page: page, PageSize: pageSize,
	}.Normalized()
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	var rows []careplan.WithdrawalCompletion
	var total int
	switch state {
	case "", careplan.WithdrawalStatePending:
		rows, total, err = rs.CareLifecycleService.ListPendingWithdrawals(r.Context(), filter)
	case careplan.WithdrawalStateResolved:
		rows, total, err = rs.CareLifecycleService.ListResolvedWithdrawals(r.Context(), filter)
	default:
		renderError(w, r, common.ErrorInvalidRequest(errors.New("state must be pending or resolved")))
		return
	}
	if err != nil {
		renderError(w, r, careExitErrorRenderer(err))
		return
	}
	respondCareWithdrawalPage(w, r, rows, total, filter, "Offene Abmeldungen geladen")
}

// respondCareWithdrawalPage writes one page of withdrawal tasks in the
// response type that never carries the exit reason.
func respondCareWithdrawalPage(w http.ResponseWriter, r *http.Request, rows []careplan.WithdrawalCompletion, total int, filter careplan.CareWithdrawalFilter, message string) {
	out := make([]careWithdrawalResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, toCareWithdrawalResponse(row))
	}
	common.Respond(w, r, http.StatusOK, map[string]any{
		"items": out, "total": total, "page": filter.Page, "page_size": filter.PageSize,
	}, message)
}

// listStudentResolvedCareWithdrawals pages the completed withdrawals of one child for
// its Änderungsprotokoll (#5025). It carries the request log's gate instead of
// the tenant list's users:delete, so it stays narrower than that list in every
// other way: the child the caller may read, resolved tasks only, no search,
// and the response type that never carries the exit reason.
func (rs *Resource) listStudentResolvedCareWithdrawals(w http.ResponseWriter, r *http.Request) {
	if rs.CareLifecycleService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("care lifecycle service not configured")))
		return
	}
	if !securityruntime.CanReviewExcusedAbsenceRequests(sessiontoken.PermissionsFromCtx(r.Context())) {
		renderError(w, r, common.ErrorForbidden(errors.New("users:absence requires users:read to view the child's withdrawals")))
		return
	}
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}
	if !rs.checkStudentReadAccess(r, student) {
		renderError(w, r, common.ErrorForbidden(errors.New("read access required to view the child's withdrawals")))
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	filter := careplan.CareWithdrawalFilter{StudentID: student.ID, Page: page, PageSize: pageSize}.Normalized()
	rows, total, err := rs.CareLifecycleService.ListResolvedWithdrawals(r.Context(), filter)
	if err != nil {
		renderError(w, r, careExitErrorRenderer(err))
		return
	}
	respondCareWithdrawalPage(w, r, rows, total, filter, "Abgeschlossene Abmeldungen geladen")
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
		r.Context(), id, body.Token, input, int64(sessiontoken.ClaimsFromCtx(r.Context()).ID),
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

type careExitCancelRequest struct {
	StudentIDs []string `json:"student_ids"`
}

func (b *careExitCancelRequest) Bind(_ *http.Request) error {
	if len(b.StudentIDs) == 0 {
		return careplan.ErrCareExitNoStudents
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
			renderError(w, r, common.ErrorInvalidRequest(errors.New("student_ids enthält einen ungültigen Wert")))
			return
		}
		ids = append(ids, id)
	}
	actorID := int64(sessiontoken.ClaimsFromCtx(r.Context()).ID)
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
		return errors.New("bitte geben Sie den neuen Beginn an")
	}
	if !b.Checked {
		return careplan.ErrCareResumeNotChecked
	}
	return nil
}

// resumeCare reopens one child's care from a new start day.
func (rs *Resource) resumeCare(w http.ResponseWriter, r *http.Request) {
	if rs.CareResumption == nil {
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
	start, parseErr := calendar.ParseDate(strings.TrimSpace(body.NewStart))
	if parseErr != nil {
		renderError(w, r, common.ErrorInvalidRequest(errors.New("bitte geben Sie den neuen Beginn im Format JJJJ-MM-TT an")))
		return
	}
	err := rs.CareResumption(r.Context(), careplan.CareResumeInput{
		StudentID:      studentID,
		NewStart:       start,
		ActorAccountID: int64(sessiontoken.ClaimsFromCtx(r.Context()).ID),
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
	filter := careplan.EndedCareFilter{
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

// careExitErrorRenderer classifies the care-lifecycle sentinels. Each one
// answers with its own code, so the frontend words it from the catalog
// (ADR 0006); the German sentence stays as diagnosis for older readers.
var careExitErrorRenderer = common.RulesRenderer([]common.ErrorRule{
	{Target: careplan.ErrCareExitNoStudents, Render: invalidWithCode(common.CodeStudentsCareExitNoStudents)},
	{Target: careplan.ErrCareExitTooManyStudents, Render: invalidWithCode(common.CodeStudentsCareExitTooManyStudents)},
	{Target: careplan.ErrCareExitDayInPast, Render: invalidWithCode(common.CodeStudentsCareExitDayInPast)},
	{Target: careplan.ErrCareExitPreviewChanged, Render: conflictWithCode(common.CodeStudentsCareExitPreviewChanged)},
	{Target: careplan.ErrCareExitBlocked, Render: conflictWithCode(common.CodeStudentsCareExitBlocked)},
	{Target: careplan.ErrCareExitNotPlanned, Render: conflictWithCode(common.CodeStudentsCareExitNotPlanned)},
	{Target: careplan.ErrCareExitAlreadyEffective, Render: conflictWithCode(common.CodeStudentsCareExitAlreadyEffective)},
	{Target: careplan.ErrCareResumeNotEnded, Render: conflictWithCode(common.CodeStudentsCareResumeNotEnded)},
	{Target: careplan.ErrCareResumeMissing, Render: conflictWithCode(common.CodeStudentsCareResumeMissing)},
	{Target: careplan.ErrCareResumeStartInPast, Render: invalidWithCode(common.CodeStudentsCareResumeStartInPast)},
	{Target: careplan.ErrCareResumeNotChecked, Render: invalidWithCode(common.CodeStudentsCareResumeNotChecked)},
	{Target: careplan.ErrCareResumeUnknownStudent, Render: notFoundWithCode(common.CodeTimetableStudentNotFound)},
	{Target: careplan.ErrCareResumeAlumnus, Render: conflictWithCode(common.CodeStudentsCareResumeAlumnus)},
	{Target: careplan.ErrCareWithdrawalNotFound, Render: notFoundWithCode(common.CodeStudentsCareWithdrawalNotFound)},
	{Target: careplan.ErrCareWithdrawalAfterGap, Render: invalidWithCode(common.CodeStudentsCareWithdrawalAfterGap)},
	{Target: careplan.ErrCareWithdrawalAlreadyResolved, Render: conflictWithCode(common.CodeStudentsCareWithdrawalAlreadyResolved)},
	{Match: func(err error) bool {
		var dateErr *careplan.CareWithdrawalDateError
		return errors.As(err, &dateErr)
	}, Render: invalidWithCode(common.CodeStudentsCareWithdrawalDateInvalid)},
	{Target: careplan.ErrCareExitInvalidReason, Render: invalidWithCode(common.CodeStudentsCareExitInvalidReason)},
	{Target: careplan.ErrCareExitNoteRequired, Render: invalidWithCode(common.CodeStudentsCareExitNoteRequired)},
	{Target: careplan.ErrCareExitNoteNotAllowed, Render: invalidWithCode(common.CodeStudentsCareExitNoteNotAllowed)},
	{Target: careplan.ErrCareExitNoteTooLong, Render: invalidWithCode(common.CodeStudentsCareExitNoteTooLong)},
	// Resuming care counts against the Kinderkontingent (#3567).
	{Match: common.IsBusinessRejection, Render: common.ErrorBusinessRejection},
}, func(cause error) render.Renderer {
	// Alles Unerwartete (DB-Fehler, Sperren, Timeouts) bekommt EINEN ruhigen
	// Satz als Diagnose. Der Dialog zeigt den Katalogtext für den Fehlercode;
	// die volle Fehlerkette geht an slog und Sentry.
	return common.ErrorInternalServerWrap(
		"Die Betreuung wurde nicht beendet. Bitte versuchen Sie es noch einmal.",
		cause,
	)
})
