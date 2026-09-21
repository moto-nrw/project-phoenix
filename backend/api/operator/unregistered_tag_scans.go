package operator

import (
	"errors"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
)

// UnregisteredTagScanResolveError renders a failed resolution of an
// unregistered RFID scan. Unknown or already handled scans, a missing
// operator, and invalid IDs stay 400 with their text. Persistence failures
// stay internal and never echo adapter or Postgres text: Device Fleet wraps
// driver errors and does not return *models/base.DatabaseError.
func UnregisteredTagScanResolveError(err error) render.Renderer {
	if _, ok := errors.AsType[*modelBase.DatabaseError](err); ok {
		return common.OperatorInternal("Failed to resolve unregistered RFID scan")
	}
	if unregisteredTagScanResolveIsClientError(err) {
		return common.OperatorInvalidRequest(err)
	}
	return common.OperatorInternal("Failed to resolve unregistered RFID scan")
}

func unregisteredTagScanResolveIsClientError(err error) bool {
	for err != nil {
		switch err.Error() {
		case "operator ID is required",
			"scan ID is required",
			"unregistered tag scan not found",
			"unregistered tag scan already resolved",
			"invalid unregistered tag scan":
			return true
		}
		err = errors.Unwrap(err)
	}
	return false
}
