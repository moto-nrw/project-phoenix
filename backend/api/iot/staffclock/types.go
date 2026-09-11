package staffclock

import (
	"errors"
	"net/http"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
)

type StateRequest struct {
	RFIDTag string `json:"rfid_tag"`
}

func (req *StateRequest) Bind(_ *http.Request) error {
	return validation.ValidateStruct(req,
		validation.Field(&req.RFIDTag, validation.Required),
	)
}

type CommandRequest struct {
	RFIDTag                string `json:"rfid_tag"`
	Action                 string `json:"action"`
	Status                 string `json:"status,omitempty"`
	Reason                 string `json:"reason,omitempty"`
	PlannedDurationMinutes *int   `json:"planned_duration_minutes,omitempty"`
}

func (req *CommandRequest) Bind(_ *http.Request) error {
	if err := validation.ValidateStruct(req,
		validation.Field(&req.RFIDTag, validation.Required),
		validation.Field(&req.Action, validation.Required, validation.In(
			devicescan.StaffClockActionCheckIn,
			devicescan.StaffClockActionCheckOut,
			devicescan.StaffClockActionBreakStart,
			devicescan.StaffClockActionBreakEnd,
		)),
	); err != nil {
		return err
	}
	if req.Action == devicescan.StaffClockActionCheckIn && req.Status == "" {
		return errors.New("status is required for check-in")
	}
	return nil
}
