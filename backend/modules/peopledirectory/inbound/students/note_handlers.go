package students

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Child note card ("Notizen", Kartei): the durable hints that replaced the
// Betreuernotizen field and the dated entries of the chronicle.
//
// Reading is gated by users:read plus the per-child read predicate every other
// child surface uses; WHICH notes come back is then decided by the audience
// (securityruntime.ResolveStudentNoteAudience), not by the route.

type studentNoteResponse struct {
	ID         string `json:"id"`
	StudentID  string `json:"student_id"`
	Kind       string `json:"kind"`
	Visibility string `json:"visibility"`
	Category   string `json:"category,omitempty"`
	Body       string `json:"body"`
	// Origin is "staff" or "master_data"; the latter marks the hints carried
	// over from the former Betreuernotizen field, which have no author.
	Origin           string `json:"origin"`
	AuthorAccountID  string `json:"author_account_id,omitempty"`
	AuthorName       string `json:"author_name,omitempty"`
	SubjectDate      string `json:"subject_date,omitempty"`
	ActivityGroupID  string `json:"activity_group_id,omitempty"`
	EducationGroupID string `json:"education_group_id,omitempty"`
	// CanEdit and CanDelete tell the client which affordances to render, so it
	// never offers an action the next request refuses.
	CanEdit   bool      `json:"can_edit"`
	CanDelete bool      `json:"can_delete"`
	Edited    bool      `json:"edited"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type studentNoteRequestBody struct {
	Kind             string  `json:"kind"`
	Visibility       string  `json:"visibility"`
	Category         string  `json:"category"`
	Body             string  `json:"body"`
	SubjectDate      *string `json:"subject_date"`
	ActivityGroupID  *string `json:"activity_group_id"`
	EducationGroupID *string `json:"education_group_id"`
}

func (rs *Resource) listStudentNotes(w http.ResponseWriter, r *http.Request) {
	student, _, audience, ok := rs.resolveNoteReader(w, r)
	if !ok {
		return
	}
	kind := r.URL.Query().Get("kind")
	if kind != "" && !peopleModule.ValidStudentNoteKind(kind) {
		renderError(w, r, common.ErrorInvalidRequest(errors.New("unknown note kind")))
		return
	}
	notes, err := rs.StudentNotes.ListStudentNotes(r.Context(), peopleModule.StudentNoteFilter{
		StudentID: student.ID, Kind: kind, Audience: toModuleAudience(audience, callerAccountID(r)),
	})
	if err != nil {
		renderError(w, r, studentNoteErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK,
		rs.renderNotes(r, notes, audience, student), "Student notes retrieved")
}

func (rs *Resource) createStudentNote(w http.ResponseWriter, r *http.Request) {
	student, child, audience, ok := rs.resolveNoteReader(w, r)
	if !ok {
		return
	}
	body, ok := decodeStudentNoteBody(w, r)
	if !ok {
		return
	}
	subject, err := parseNoteSubject(body)
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if err := validateNoteSubjectForChild(subject, child); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	note, err := rs.StudentNotes.CreateStudentNote(r.Context(), peopleModule.CreateStudentNote{
		StudentID: student.ID, AuthorAccountID: callerAccountID(r),
		Kind: body.Kind, Visibility: body.Visibility, Category: body.Category,
		Body: body.Body, Subject: subject,
		RevalidateSubject: func(ctx context.Context) error {
			return rs.revalidateNoteSubject(ctx, student.ID, subject)
		},
	})
	if err != nil {
		renderError(w, r, studentNoteErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusCreated,
		rs.renderNote(r, note, audience, student), "Student note created")
}

func (rs *Resource) updateStudentNote(w http.ResponseWriter, r *http.Request) {
	student, _, audience, ok := rs.resolveNoteReader(w, r)
	if !ok {
		return
	}
	noteID, ok := common.ParsePositiveInt64IDWithError(w, r, "noteId", "invalid note id")
	if !ok {
		return
	}
	body, ok := decodeStudentNoteBody(w, r)
	if !ok {
		return
	}
	subject, err := parseNoteSubject(studentNoteRequestBody{SubjectDate: body.SubjectDate})
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	note, err := rs.StudentNotes.UpdateStudentNote(r.Context(), peopleModule.UpdateStudentNote{
		ID: noteID, StudentID: student.ID, ActorAccountID: callerAccountID(r), Kind: body.Kind,
		Visibility: body.Visibility, Category: body.Category, Body: body.Body, SubjectDate: subject.Date,
	})
	if err != nil {
		renderError(w, r, studentNoteErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK,
		rs.renderNote(r, note, audience, student), "Student note updated")
}

func (rs *Resource) deleteStudentNote(w http.ResponseWriter, r *http.Request) {
	student, child, audience, ok := rs.resolveNoteReader(w, r)
	if !ok {
		return
	}
	noteID, ok := common.ParsePositiveInt64IDWithError(w, r, "noteId", "invalid note id")
	if !ok {
		return
	}
	// Read the note through the same audience-filtered list the timeline uses:
	// a note this caller may not see is a not-found for them, and must not
	// become visible through a delete attempt.
	found, err := rs.StudentNotes.ListStudentNotes(r.Context(), peopleModule.StudentNoteFilter{
		StudentID: student.ID, NoteID: noteID, Audience: toModuleAudience(audience, callerAccountID(r)),
	})
	if err != nil {
		renderError(w, r, studentNoteErrorRenderer(err))
		return
	}
	if len(found) == 0 {
		renderError(w, r, common.ErrorNotFound(errors.New("note not found")))
		return
	}
	if err := rs.StudentNotes.DeleteStudentNote(r.Context(), peopleModule.DeleteStudentNote{
		ID: noteID, StudentID: student.ID, ActorAccountID: callerAccountID(r),
		Authorization: peopleModule.StudentNoteDeleteAuthorization{
			Admin:                 audience.Admin,
			LedActivityGroupIDs:   audience.LedActivityGroupIDs,
			LedEducationGroupIDs:  audience.LedEducationGroupIDs,
			ChildEducationGroupID: child.GroupID,
		},
		ResolveAuthorization: func(ctx context.Context) (peopleModule.StudentNoteDeleteAuthorization, error) {
			return rs.currentNoteDeleteAuthorization(ctx, student.ID, callerAccountID(r))
		},
	}); err != nil {
		renderError(w, r, studentNoteErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]string{
		"id": strconv.FormatInt(noteID, 10),
	}, "Student note deleted")
}

// revalidateNoteSubject reads the current assignment while the note service
// holds the child's row lock. Group changes serialize on that row, so the
// reference cannot be written against the earlier handler snapshot.
func (rs *Resource) revalidateNoteSubject(
	ctx context.Context, studentID int64, subject peopleModule.StudentNoteSubject,
) error {
	student, err := rs.lockStudent(ctx, studentID)
	if err != nil {
		return err
	}
	child, err := rs.noteChild(ctx, student)
	if err != nil {
		return err
	}
	return validateNoteSubjectForChild(subject, child)
}

// currentNoteDeleteAuthorization reloads the foreign authorization facts in
// the transaction that removes the note. The ordinary request cache is right
// for reads, but would preserve the stale answer this write must reject.
func (rs *Resource) currentNoteDeleteAuthorization(
	ctx context.Context, studentID, accountID int64,
) (peopleModule.StudentNoteDeleteAuthorization, error) {
	student, err := rs.lockStudent(ctx, studentID)
	if err != nil {
		return peopleModule.StudentNoteDeleteAuthorization{}, err
	}
	if cache := jwt.RequestIdentityCacheFrom(ctx); cache != nil {
		cache.Evict(tenant.FromContext(ctx), accountID)
	}
	reader, err := rs.noteReader(ctx)
	if err != nil {
		return peopleModule.StudentNoteDeleteAuthorization{}, err
	}
	child, err := rs.noteChild(ctx, student)
	if err != nil {
		return peopleModule.StudentNoteDeleteAuthorization{}, err
	}
	audience := securityruntime.ResolveStudentNoteAudience(
		jwt.PermissionsFromCtx(ctx), reader, child)
	return peopleModule.StudentNoteDeleteAuthorization{
		Admin:                 audience.Admin,
		LedActivityGroupIDs:   audience.LedActivityGroupIDs,
		LedEducationGroupIDs:  audience.LedEducationGroupIDs,
		ChildEducationGroupID: child.GroupID,
	}, nil
}

// resolveNoteReader loads the child, re-checks the per-child read predicate and
// resolves the caller's audience. Every note route starts here: the route
// permission (users:read) says the caller may read children at all, this says
// which of THIS child's notes they may see.
func (rs *Resource) resolveNoteReader(
	w http.ResponseWriter, r *http.Request,
) (*Student, securityruntime.StudentNoteChild, securityruntime.StudentNoteAudience, bool) {
	if rs.StudentNotes == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("student notes capability is not configured")))
		return nil, securityruntime.StudentNoteChild{}, securityruntime.StudentNoteAudience{}, false
	}
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return nil, securityruntime.StudentNoteChild{}, securityruntime.StudentNoteAudience{}, false
	}
	if !rs.checkStudentReadAccess(r, student) {
		renderError(w, r, common.ErrorForbidden(errors.New("read access required to view notes")))
		return nil, securityruntime.StudentNoteChild{}, securityruntime.StudentNoteAudience{}, false
	}
	child, err := rs.noteChild(r.Context(), student)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return nil, securityruntime.StudentNoteChild{}, securityruntime.StudentNoteAudience{}, false
	}
	reader, err := rs.noteReader(r.Context())
	if err != nil {
		// A reach that cannot be read fails the request. The wide audience
		// would leak, the empty one would render the child's card as empty —
		// both are worse than an error the caller can retry.
		renderError(w, r, common.ErrorInternalServer(err))
		return nil, securityruntime.StudentNoteChild{}, securityruntime.StudentNoteAudience{}, false
	}
	return student, child, securityruntime.ResolveStudentNoteAudience(
		jwt.PermissionsFromCtx(r.Context()), reader, child), true
}

// noteReader gathers the caller's assignments for the audience policy, which
// takes facts rather than fetching them. The school classes are normalized
// here because the child's class is too, and both sides must use the rule the
// class-scoped reads and their LOWER(BTRIM(...)) index already use.
func (rs *Resource) noteReader(ctx context.Context) (securityruntime.StudentNoteReader, error) {
	groupIDs, err := rs.UserContextService.MyGroupIDs(ctx)
	if err != nil {
		return securityruntime.StudentNoteReader{}, err
	}
	activityIDs, err := rs.UserContextService.MyActivityGroupIDs(ctx)
	if err != nil {
		return securityruntime.StudentNoteReader{}, err
	}
	classes, err := rs.UserContextService.MySchoolClasses(ctx)
	if err != nil {
		return securityruntime.StudentNoteReader{}, err
	}
	normalized := make([]string, 0, len(classes))
	for _, class := range classes {
		normalized = append(normalized, schoolClassKey(class))
	}
	return securityruntime.StudentNoteReader{
		GroupIDs: groupIDs, ActivityGroupIDs: activityIDs, Classes: normalized,
	}, nil
}

// noteChild reads the child side of the audience question. The activity
// enrollments are read for today: a note audience follows who has the child
// now, not who had it last term.
func (rs *Resource) noteChild(ctx context.Context, student *Student) (securityruntime.StudentNoteChild, error) {
	child := securityruntime.StudentNoteChild{
		GroupID: student.GroupID, SchoolClass: schoolClassKey(student.SchoolClass),
	}
	if rs.ActiveEnrollments == nil {
		return child, nil
	}
	groupsByStudent, err := rs.ActiveEnrollments.ActiveEnrollmentGroups(
		ctx, []int64{student.ID}, timezone.TodayDate())
	if err != nil {
		return securityruntime.StudentNoteChild{}, err
	}
	for _, group := range groupsByStudent[student.ID] {
		child.ActivityGroupIDs = append(child.ActivityGroupIDs, group.ID)
	}
	return child, nil
}

func decodeStudentNoteBody(w http.ResponseWriter, r *http.Request) (studentNoteRequestBody, bool) {
	var body studentNoteRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(errors.New("invalid note payload")))
		return studentNoteRequestBody{}, false
	}
	if body.Kind == "" {
		body.Kind = peopleModule.StudentNoteKindJournal
	}
	if body.Visibility == "" {
		body.Visibility = peopleModule.StudentNoteVisibilityAllStaff
	}
	if !peopleModule.ValidStudentNoteKind(body.Kind) ||
		!peopleModule.ValidStudentNoteVisibility(body.Visibility) ||
		!peopleModule.ValidStudentNoteCategory(body.Category) {
		renderError(w, r, common.ErrorInvalidRequest(errors.New("unknown note kind, visibility or category")))
		return studentNoteRequestBody{}, false
	}
	return body, true
}

func parseNoteSubject(body studentNoteRequestBody) (peopleModule.StudentNoteSubject, error) {
	subject := peopleModule.StudentNoteSubject{}
	if body.SubjectDate != nil && *body.SubjectDate != "" {
		date, err := timezone.ParseDate(*body.SubjectDate)
		if err != nil {
			return subject, errors.New("subject_date must be a YYYY-MM-DD date")
		}
		if date.After(timezone.TodayDate()) {
			return subject, errors.New("subject_date must not be in the future")
		}
		subject.Date = &date
	}
	activityID, err := parseOptionalID(body.ActivityGroupID, "activity_group_id")
	if err != nil {
		return subject, err
	}
	educationID, err := parseOptionalID(body.EducationGroupID, "education_group_id")
	if err != nil {
		return subject, err
	}
	if activityID != nil && educationID != nil {
		return subject, errors.New("a note refers to an activity or to a group, not to both")
	}
	subject.ActivityGroupID, subject.EducationGroupID = activityID, educationID
	return subject, nil
}

// validateNoteSubjectForChild keeps a note's reference inside the selected
// child's current group or enrollment. The same child snapshot already drives
// the audience decision, so this adds no second cross-owner lookup.
func validateNoteSubjectForChild(
	subject peopleModule.StudentNoteSubject,
	child securityruntime.StudentNoteChild,
) error {
	if subject.EducationGroupID != nil &&
		(child.GroupID == nil || *subject.EducationGroupID != *child.GroupID) {
		return errors.New("education_group_id must belong to the child")
	}
	if subject.ActivityGroupID != nil {
		for _, activityGroupID := range child.ActivityGroupIDs {
			if *subject.ActivityGroupID == activityGroupID {
				return nil
			}
		}
		return errors.New("activity_group_id must belong to the child")
	}
	return nil
}

func parseOptionalID(value *string, field string) (*int64, error) {
	if value == nil || *value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseInt(*value, 10, 64)
	if err != nil || parsed <= 0 {
		return nil, errors.New(field + " must be a positive numeric id")
	}
	return &parsed, nil
}

func callerAccountID(r *http.Request) int64 {
	return int64(jwt.ClaimsFromCtx(r.Context()).ID)
}

// toModuleAudience turns the policy's facts into the owner's filter. The
// visibility values are named here, in the one package that holds both sides,
// so the policy never keeps a second copy of the owner's enum.
func toModuleAudience(audience securityruntime.StudentNoteAudience, accountID int64) peopleModule.StudentNoteAudience {
	visibilities := []string{peopleModule.StudentNoteVisibilityAllStaff}
	if audience.CareTeam {
		visibilities = append(visibilities, peopleModule.StudentNoteVisibilityCareTeam)
	}
	if audience.Admin {
		visibilities = append(visibilities, peopleModule.StudentNoteVisibilityGroupLeads)
	}
	return peopleModule.StudentNoteAudience{
		Visibilities:         visibilities,
		LedActivityGroupIDs:  audience.LedActivityGroupIDs,
		LedEducationGroupIDs: audience.LedEducationGroupIDs,
		ReaderAccountID:      accountID,
	}
}

func (rs *Resource) renderNotes(
	r *http.Request,
	notes []peopleModule.StudentNote,
	audience securityruntime.StudentNoteAudience,
	student *Student,
) []studentNoteResponse {
	reader := callerAccountID(r)
	result := make([]studentNoteResponse, 0, len(notes))
	for _, note := range notes {
		result = append(result, buildNoteResponse(note, audience, student, reader))
	}
	return result
}

func (rs *Resource) renderNote(
	r *http.Request,
	note peopleModule.StudentNote,
	audience securityruntime.StudentNoteAudience,
	student *Student,
) studentNoteResponse {
	return buildNoteResponse(note, audience, student, callerAccountID(r))
}

func buildNoteResponse(
	note peopleModule.StudentNote,
	audience securityruntime.StudentNoteAudience,
	student *Student,
	readerAccountID int64,
) studentNoteResponse {
	response := studentNoteResponse{
		ID: strconv.FormatInt(note.ID, 10), StudentID: strconv.FormatInt(note.StudentID, 10),
		Kind: note.Kind, Visibility: note.Visibility, Category: note.Category,
		Body: note.Body, Origin: note.Origin, Edited: note.Edited(),
		CreatedAt: note.CreatedAt, UpdatedAt: note.UpdatedAt,
		CanDelete: securityruntime.CanDeleteStudentNote(
			audience, note.Subject.ActivityGroupID, note.Subject.EducationGroupID, student.GroupID),
	}
	if note.AuthorAccountID != nil {
		response.AuthorAccountID = strconv.FormatInt(*note.AuthorAccountID, 10)
		response.AuthorName = note.AuthorName
		// Only the author rewords an entry, and never one that was carried
		// over from the master-data field — that one has no author to stand
		// behind the new wording.
		response.CanEdit = *note.AuthorAccountID == readerAccountID &&
			note.Origin == peopleModule.StudentNoteOriginStaff
	}
	if note.Subject.Date != nil {
		response.SubjectDate = note.Subject.Date.String()
	}
	if note.Subject.ActivityGroupID != nil {
		response.ActivityGroupID = strconv.FormatInt(*note.Subject.ActivityGroupID, 10)
	}
	if note.Subject.EducationGroupID != nil {
		response.EducationGroupID = strconv.FormatInt(*note.Subject.EducationGroupID, 10)
	}
	return response
}

var studentNoteErrorRenderer = common.RulesRenderer([]common.ErrorRule{
	{Target: peopleModule.ErrStudentNoteInvalid, Render: common.ErrorInvalidRequest},
	{Target: peopleModule.ErrStudentNoteNotFound, Render: common.ErrorNotFound},
	{Target: peopleModule.ErrStudentNoteNotAuthor, Render: common.ErrorForbidden},
	{Target: peopleModule.ErrStudentNoteImmutable, Render: common.ErrorForbidden},
	{Target: peopleModule.ErrStudentNoteDeleteForbidden, Render: common.ErrorForbidden},
}, common.ErrorInternalServer)
