// Package timetable — later pickup times that still need a block (#3261).
//
//	GET  /api/timetable/pickup-extensions[?student_id=]
//	POST /api/timetable/pickup-extensions/{id}/resolve
//
// A child who is picked up later than before may not be on any block for the
// extra time. The Timetable owner keeps an open task per day or weekday; the
// Leitung resolves it by picking one or more blocks, or none. Both routes need
// SchedulesManage: the choice changes block rosters, and a list nobody can act
// on is noise.
package timetable

import (
	"errors"
	"net/http"
	"sort"
	"strconv"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
)

const (
	pickupExtensionNotFoundCode  = "pickup_extension_not_found"
	pickupExtensionBlockGoneCode = "pickup_extension_block_gone"
)

type pickupExtensionBlockResponse struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}

type pickupExtensionResponse struct {
	ID                 int64                          `json:"id"`
	StudentID          int64                          `json:"student_id"`
	StudentName        string                         `json:"student_name"`
	Kind               string                         `json:"kind"`
	Date               string                         `json:"date,omitempty"`
	Weekday            int                            `json:"weekday,omitempty"`
	EffectiveFrom      string                         `json:"effective_from,omitempty"`
	PreviousPickupTime string                         `json:"previous_pickup_time"`
	PickupTime         string                         `json:"pickup_time"`
	Blocks             []pickupExtensionBlockResponse `json:"blocks"`
}

type pickupExtensionsResponse struct {
	Tasks []pickupExtensionResponse `json:"tasks"`
}

type resolvePickupExtensionRequest struct {
	BlockIDs []int64 `json:"block_ids"`
}

func (req *resolvePickupExtensionRequest) Bind(_ *http.Request) error {
	if len(req.BlockIDs) > 20 {
		return errors.New("block_ids cannot exceed 20 items")
	}
	for _, id := range req.BlockIDs {
		if id <= 0 {
			return errors.New("block_ids must be positive")
		}
	}
	return nil
}

type resolvePickupExtensionResponse struct {
	StudentID      int64                          `json:"student_id"`
	Kind           string                         `json:"kind"`
	AssignedBlocks []pickupExtensionBlockResponse `json:"assigned_blocks"`
	InstanceCount  int                            `json:"instance_count"`
}

// listPickupExtensions handles GET /api/timetable/pickup-extensions.
func (rs *Resource) listPickupExtensions(w http.ResponseWriter, r *http.Request) {
	if rs.PickupExtensions == nil || rs.PersonService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable resource not fully wired")))
		return
	}
	var studentID int64
	if raw := r.URL.Query().Get("student_id"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid student_id")))
			return
		}
		studentID = parsed
	}
	tasks, err := rs.PickupExtensions.ListOpenPickupExtensions(r.Context(), studentID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("list pickup extensions failed", err))
		return
	}
	names, err := rs.readableStudentNames(r, pickupExtensionStudentIDs(tasks))
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("load students failed", err))
		return
	}
	response := pickupExtensionsResponse{Tasks: make([]pickupExtensionResponse, 0, len(tasks))}
	for _, task := range tasks {
		name, visible := names[task.StudentID]
		if !visible {
			continue
		}
		response.Tasks = append(response.Tasks, pickupExtensionToResponse(task, name))
	}
	common.Respond(w, r, http.StatusOK, response, "Pickup extensions retrieved")
}

// resolvePickupExtension handles POST /api/timetable/pickup-extensions/{id}/resolve.
// A weekday choice writes the template roster, so the handler takes the
// tenant recurrence gate first, like every other template writer.
func (rs *Resource) resolvePickupExtension(w http.ResponseWriter, r *http.Request) {
	if rs.PickupExtensions == nil || rs.PersonService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable resource not fully wired")))
		return
	}
	taskID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid pickup extension id")))
		return
	}
	req := &resolvePickupExtensionRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	ctx := r.Context()
	studentID, err := rs.PickupExtensions.FindPickupExtensionStudent(ctx, taskID)
	if err != nil {
		renderPickupExtensionError(w, r, err)
		return
	}
	names, err := rs.readableStudentNames(r, []int64{studentID})
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("load student failed", err))
		return
	}
	if _, visible := names[studentID]; !visible {
		common.RenderError(w, r, common.ErrorNotFoundWithCode(errors.New("pickup extension not found"), pickupExtensionNotFoundCode))
		return
	}
	if rs.RecurrenceLock == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("template recurrence lock not wired")))
		return
	}
	if err := rs.RecurrenceLock.LockRecurrenceWrites(ctx); err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("lock template recurrence failed", err))
		return
	}
	result, err := rs.PickupExtensions.ResolvePickupExtension(ctx, taskID, req.BlockIDs)
	if err != nil {
		tenant.MarkRollback(ctx)
		renderPickupExtensionError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, resolvePickupExtensionResponse{
		StudentID:      result.StudentID,
		Kind:           result.Kind,
		AssignedBlocks: pickupExtensionBlocksToResponse(result.AssignedBlocks),
		InstanceCount:  len(result.InstanceIDs),
	}, "Pickup extension resolved")
}

func renderPickupExtensionError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, timetable.ErrInvalidPickupExtension):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	case errors.Is(err, timetable.ErrPickupExtensionNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, pickupExtensionNotFoundCode))
	case errors.Is(err, timetable.ErrPickupExtensionBlockGone):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, pickupExtensionBlockGoneCode))
	default:
		common.RenderError(w, r, common.ErrorInternalServerWrap("resolve pickup extension failed", err))
	}
}

func pickupExtensionStudentIDs(tasks []timetable.PickupExtensionTask) []int64 {
	seen := make(map[int64]struct{}, len(tasks))
	ids := make([]int64, 0, len(tasks))
	for _, task := range tasks {
		if _, ok := seen[task.StudentID]; ok {
			continue
		}
		seen[task.StudentID] = struct{}{}
		ids = append(ids, task.StudentID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func pickupExtensionToResponse(task timetable.PickupExtensionTask, studentName string) pickupExtensionResponse {
	return pickupExtensionResponse{
		ID:                 task.ID,
		StudentID:          task.StudentID,
		StudentName:        studentName,
		Kind:               task.Kind,
		Date:               task.Date,
		Weekday:            task.Weekday,
		EffectiveFrom:      task.EffectiveFrom,
		PreviousPickupTime: task.PreviousPickup,
		PickupTime:         task.Pickup,
		Blocks:             pickupExtensionBlocksToResponse(task.Blocks),
	}
}

func pickupExtensionBlocksToResponse(blocks []timetable.PickupExtensionBlock) []pickupExtensionBlockResponse {
	result := make([]pickupExtensionBlockResponse, 0, len(blocks))
	for _, block := range blocks {
		result = append(result, pickupExtensionBlockResponse(block))
	}
	return result
}
