package rooms

import (
	"encoding/json"
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
	file, err := rs.runtime.ExportSnapshot(r.Context(), SnapshotRequest{
		Format: request.Format, Title: request.Title, RoomIDs: request.RoomIDs, IncludeTransit: request.IncludeTransit,
	})
	if err != nil {
		rs.runtime.Failure(w, r, FailureInternal, err, "internal_error")
		return
	}
	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.Filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(file.Data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Data)
}
