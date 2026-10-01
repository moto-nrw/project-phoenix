package timetablehttp

import (
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// requireWebAttendanceForActiveInstance leaves pure planning cancellations
// available while blocking cancellation of a live instance, which closes
// visits and changes attendance. It runs after the tenant transaction.
func (rs *Resource) requireWebAttendanceForActiveInstance(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rs.TimetableData == nil {
			common.RenderError(w, r, common.ErrorServiceUnavailable(errors.New("timetable data service is not configured")))
			return
		}
		id, err := common.ParseID(r)
		if err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid instance id")))
			return
		}
		instance, err := rs.TimetableData.FindScheduledInstance(r.Context(), id)
		if errors.Is(err, timetable.ErrActivityInstanceNotFound) {
			common.RenderError(w, r, common.ErrorNotFound(errors.New("instance not found")))
			return
		}
		if err != nil {
			common.RenderError(w, r, common.ErrorInternalServer(err))
			return
		}
		if instance.Status != timetable.InstanceStatusActive {
			next.ServeHTTP(w, r)
			return
		}
		common.RequireWebAttendanceEnabled(rs.SettingsService)(next).ServeHTTP(w, r)
	})
}
