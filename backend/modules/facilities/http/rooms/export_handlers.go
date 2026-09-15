package rooms

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
)

type roomSnapshotExportRequest struct {
	Format         string   `json:"format"`
	Title          string   `json:"title"`
	RoomIDs        *[]int64 `json:"room_ids"`
	IncludeTransit bool     `json:"include_transit"`
}

// InvalidExportError marks renderer validation failures that preserve the
// established 400 response without misclassifying data-loading failures.
type InvalidExportError struct{ Err error }

func (e *InvalidExportError) Error() string { return e.Err.Error() }
func (e *InvalidExportError) Unwrap() error { return e.Err }

func (rs *Resource) exportSnapshot(w http.ResponseWriter, r *http.Request) {
	var request roomSnapshotExportRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		rs.runtime.Failure(w, r, FailureInvalid, err, "invalid_request")
		return
	}
	if request.Format == "" {
		request.Format = "pdf"
	}
	switch request.Format {
	case "pdf", "docx", "xlsx":
	default:
		rs.runtime.Failure(w, r, FailureInvalid, fmt.Errorf("unsupported export format %q", request.Format), "invalid_request")
		return
	}
	file, err := rs.runtime.ExportSnapshot(r.Context(), SnapshotRequest(request))
	if err != nil {
		kind, code := classifyExportFailure(err)
		rs.runtime.Failure(w, r, kind, err, code)
		return
	}
	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.Filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(file.Data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Data)
}

func classifyExportFailure(err error) (FailureKind, string) {
	var invalid *InvalidExportError
	if errors.As(err, &invalid) {
		return FailureInvalid, "invalid_request"
	}
	return FailureInternal, "internal_error"
}
