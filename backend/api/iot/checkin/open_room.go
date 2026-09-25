package checkin

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
)

// OpenRoomRequest books the child behind a card into a released room chosen
// at the device the child leaves (#3067).
type OpenRoomRequest struct {
	StudentRFID string `json:"student_rfid"`
	RoomID      int64  `json:"room_id"`
}

// Bind validates the open-room request.
func (req *OpenRoomRequest) Bind(_ *http.Request) error {
	return validation.ValidateStruct(req,
		validation.Field(&req.StudentRFID, validation.Required),
		validation.Field(&req.RoomID, validation.Required, validation.Min(int64(1))),
	)
}

// OpenRoomResponse is the booked independent stay. Moved is false when the
// child already stayed in that room.
type OpenRoomResponse struct {
	StudentID     int64     `json:"student_id"`
	StudentName   string    `json:"student_name"`
	Action        string    `json:"action"`
	RoomID        int64     `json:"room_id"`
	RoomName      string    `json:"room_name"`
	ActiveGroupID int64     `json:"active_group_id"`
	Moved         bool      `json:"moved"`
	ProcessedAt   time.Time `json:"processed_at"`
	Message       string    `json:"message"`
}

// OpenRoomResource is the kiosk's destination booking into released rooms.
type OpenRoomResource struct {
	booking devicescan.OpenRoomBooking
	runtime Runtime
	logger  *slog.Logger
}

// NewOpenRoomResource creates the destination booking resource over the
// public device-scan contract.
func NewOpenRoomResource(booking devicescan.OpenRoomBooking, runtime Runtime, logger *slog.Logger) *OpenRoomResource {
	if booking == nil || !runtime.valid() {
		panic("open room resource: the booking and the runtime are required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &OpenRoomResource{booking: booking, runtime: runtime, logger: logger}
}

// Router returns the router for the destination booking. The route requires
// device authentication (API key + staff PIN).
func (rs *OpenRoomResource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Post("/move-to-room", rs.moveToRoom)
	return r
}

// moveToRoom records an independent stay in the chosen released room. The
// destination needs no device, no second scan and no supervision.
func (rs *OpenRoomResource) moveToRoom(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	device, err := rs.booking.Device(ctx)
	if err != nil {
		renderFailure(w, r, rs.runtime, rs.logger, err)
		return
	}

	req := &OpenRoomRequest{}
	if err := render.Bind(r, req); err != nil {
		rs.logger.WarnContext(ctx, "invalid open room request",
			slog.String("device_id", device.DeviceID),
			slog.String("error", err.Error()),
		)
		invalidRequest(w, r, rs.runtime, err)
		return
	}

	result, err := rs.booking.BookOpenRoom(ctx, devicescan.OpenRoomCommand{RFIDTag: req.StudentRFID, RoomID: req.RoomID})
	if err != nil {
		renderFailure(w, r, rs.runtime, rs.logger, err)
		return
	}
	rs.runtime.Success(w, r, http.StatusOK, OpenRoomResponse{
		StudentID:     result.StudentID,
		StudentName:   result.StudentName,
		Action:        result.Action,
		RoomID:        result.RoomID,
		RoomName:      result.RoomName,
		ActiveGroupID: result.RoomSessionID,
		Moved:         result.Moved,
		ProcessedAt:   time.Now(),
		Message:       result.Message,
	}, result.Message)
}
