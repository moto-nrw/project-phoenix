package room

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// handleGetRoomHistory handles GET /{id}/history
func (a *API) handleGetRoomHistory(w http.ResponseWriter, r *http.Request) {
	// Get room ID from URL
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "Invalid room ID"})
		return
	}

	// Get room history
	history, err := a.store.GetRoomHistoryByRoom(r.Context(), id)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": err.Error()})
		return
	}

	render.JSON(w, r, history)
}

// handleGetRoomHistoryByDateRange handles GET /history/date
func (a *API) handleGetRoomHistoryByDateRange(w http.ResponseWriter, r *http.Request) {
	// Get date range from query parameters
	startDateStr := r.URL.Query().Get("start_date")
	endDateStr := r.URL.Query().Get("end_date")
	if startDateStr == "" || endDateStr == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "Start date and end date are required"})
		return
	}

	// Parse dates (format: YYYY-MM-DD)
	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "Invalid start date format (use YYYY-MM-DD)"})
		return
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "Invalid end date format (use YYYY-MM-DD)"})
		return
	}

	// Add one day to end date to include the end date in the range
	endDate = endDate.Add(24 * time.Hour)

	// Get room history
	history, err := a.store.GetRoomHistoryByDateRange(r.Context(), startDate, endDate)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": err.Error()})
		return
	}

	render.JSON(w, r, history)
}

// handleGetRoomHistoryBySupervisor handles GET /history/supervisor/{id}
func (a *API) handleGetRoomHistoryBySupervisor(w http.ResponseWriter, r *http.Request) {
	// Get supervisor ID from URL
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "Invalid supervisor ID"})
		return
	}

	// Get room history
	history, err := a.store.GetRoomHistoryBySupervisor(r.Context(), id)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": err.Error()})
		return
	}

	render.JSON(w, r, history)
}
