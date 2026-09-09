package data

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/modules/devicescan"
)

// assignStaffRFIDTag handles assigning an RFID tag to a staff member (device-authenticated endpoint)
func (rs *RFIDResource) assignStaffRFIDTag(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device from context
	if _, ok := rs.runtime.requireDevice(w, r); !ok {
		return
	}

	// Parse staff ID from URL
	staffID, err := rs.runtime.ParseID(r, "staffId")
	if err != nil {
		rs.runtime.Failure(w, r, http.StatusBadRequest, errors.New(invalidStaffIDMessage), "")
		return
	}

	// Parse request
	req := &RFIDAssignmentRequest{}
	if err := render.Bind(r, req); err != nil {
		rs.runtime.Failure(w, r, http.StatusBadRequest, err, "")
		return
	}

	response, err := rs.Tags.AssignStaffTag(r.Context(), staffID, req.RFIDTag)
	if err != nil {
		rs.renderTagFailure(w, r, err)
		return
	}

	rs.runtime.Success(w, r, http.StatusOK, response, response.Message)
}

// unassignStaffRFIDTag handles removing an RFID tag from a staff member (device-authenticated endpoint)
func (rs *RFIDResource) unassignStaffRFIDTag(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device from context
	if _, ok := rs.runtime.requireDevice(w, r); !ok {
		return
	}

	// Parse staff ID from URL
	staffID, err := rs.runtime.ParseID(r, "staffId")
	if err != nil {
		rs.runtime.Failure(w, r, http.StatusBadRequest, errors.New(invalidStaffIDMessage), "")
		return
	}

	response, err := rs.Tags.UnassignStaffTag(r.Context(), staffID)
	if err != nil {
		rs.renderTagFailure(w, r, err)
		return
	}

	rs.runtime.Success(w, r, http.StatusOK, response, response.Message)
}

func (rs *RFIDResource) renderTagFailure(w http.ResponseWriter, r *http.Request, err error) {
	if failure, ok := devicescan.IsFailure(err); ok {
		switch failure.Kind {
		case devicescan.FailureNotFound:
			rs.runtime.Failure(w, r, http.StatusNotFound, errors.New(failure.Message), "")
			return
		case devicescan.FailureInternal:
			rs.runtime.Failure(w, r, http.StatusInternalServerError, failure.Cause, failure.Message)
			return
		}
	}
	rs.runtime.Failure(w, r, http.StatusInternalServerError, err, "")
}
