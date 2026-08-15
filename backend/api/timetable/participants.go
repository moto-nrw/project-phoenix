// Package timetable — instance participants read endpoint (#2283).
//
//	GET /api/timetable/instances/{id}/participants
//
// The Leseansicht name source: schedules:read holders (every staff role) get
// the display names of the children enrolled in one instance without the
// users:read-gated tenant-wide roster. Names are filtered per student through
// authorize.CanReadStudent, so gdpr.student_data_scope keeps its meaning:
// all_staff shows every participant, group_supervisors_only only the caller's
// own groups. Filtered-out children are omitted silently — the planner shows
// counts from the instances payload either way.
package timetable

import (
	"errors"
	"net/http"
	"sort"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/base"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
)

// InstanceParticipantResponse is one visible child in an instance.
type InstanceParticipantResponse struct {
	StudentID   int64  `json:"student_id"`
	DisplayName string `json:"display_name"`
}

// InstanceParticipantsResponse is the wire shape of the participants list.
type InstanceParticipantsResponse struct {
	InstanceID   int64                         `json:"instance_id"`
	Participants []InstanceParticipantResponse `json:"participants"`
}

// getInstanceParticipants handles GET /instances/{id}/participants.
func (rs *Resource) getInstanceParticipants(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	instanceID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid instance id")))
		return
	}
	if rs.TimetableData == nil || rs.PersonService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable resource not fully wired")))
		return
	}

	if _, err := rs.TimetableData.GetActivityInstance(ctx, instanceID); err != nil {
		if base.IsNoRows(err) {
			common.RenderError(w, r, common.ErrorNotFound(errors.New("instance not found")))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServerWrap("load instance failed", err))
		return
	}

	rows, err := rs.TimetableData.GetInstanceStudents(ctx, instanceID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("load instance students failed", err))
		return
	}

	participants, err := rs.visibleParticipants(r, rows)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("load participants failed", err))
		return
	}

	resp := InstanceParticipantsResponse{InstanceID: instanceID, Participants: participants}
	common.Respond(w, r, http.StatusOK, resp, "Instance participants retrieved")
}

// visibleParticipants maps enrolled-student rows to named entries, keeping
// only students the caller may read. Alumni are excluded like every other
// staff read (see resolveStudentForRead).
func (rs *Resource) visibleParticipants(r *http.Request, rows []*scheduleModel.InstanceStudent) ([]InstanceParticipantResponse, error) {
	ctx := r.Context()

	studentIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		studentIDs = append(studentIDs, row.StudentID)
	}
	students, err := rs.PersonService.GetStudentsByIDs(ctx, studentIDs)
	if err != nil {
		return nil, err
	}

	perms := jwt.PermissionsFromCtx(ctx)
	visible := make([]*usersModel.Student, 0, len(students))
	personIDs := make([]int64, 0, len(students))
	for _, id := range studentIDs {
		student := students[id]
		if student == nil || student.Status == usersModel.StudentStatusAlumnus {
			continue
		}
		if !authorize.CanReadStudent(ctx, perms, student, rs.UserContextService, rs.SettingsService, rs.getLogger()) {
			continue
		}
		visible = append(visible, student)
		personIDs = append(personIDs, student.PersonID)
	}

	persons, err := rs.PersonService.GetByIDs(ctx, personIDs)
	if err != nil {
		return nil, err
	}

	participants := make([]InstanceParticipantResponse, 0, len(visible))
	for _, student := range visible {
		person := persons[student.PersonID]
		if person == nil {
			continue
		}
		participants = append(participants, InstanceParticipantResponse{
			StudentID:   student.ID,
			DisplayName: person.GetFullName(),
		})
	}
	sort.Slice(participants, func(i, j int) bool {
		return participants[i].DisplayName < participants[j].DisplayName
	})
	return participants, nil
}
