package importapi

import (
	"context"
	"slices"

	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
)

// childQuotaPreview tells the import preview whether the new children fit
// into the Kinderkontingent (#3571). The place names match the details of the
// 409 refusal (students.child_quota_reached), so the client reads both alike.
type childQuotaPreview struct {
	BookedPlaces    int  `json:"booked_places"`
	OccupiedPlaces  int  `json:"occupied_places"`
	RequestedPlaces int  `json:"requested_places"`
	FreePlaces      int  `json:"free_places"`
	Fits            bool `json:"fits"`
}

// studentPreviewResponse is the preview result plus the Kinderkontingent,
// which is left out for a school without one.
type studentPreviewResponse struct {
	*importModels.ImportResult[importModels.StudentImportRow]
	ChildQuota *childQuotaPreview `json:"child_quota,omitempty"`
}

// previewStudents runs the preview's dry run and judges its new children
// against the Kinderkontingent.
func (rs *Resource) previewStudents(ctx context.Context, request importModels.ImportRequest[importModels.StudentImportRow]) (studentPreviewResponse, error) {
	result, err := rs.studentImportService.Import(ctx, request)
	if err != nil {
		return studentPreviewResponse{}, err
	}
	quota, err := rs.previewChildQuota(ctx, result.ChildQuotaRequested)
	if err != nil {
		return studentPreviewResponse{}, err
	}
	return studentPreviewResponse{ImportResult: result, ChildQuota: quota}, nil
}

// previewChildQuota judges the children a preview adds to the
// Kontingentzahl: new children that count and updates that bring a child
// back into the count, never a plain update.
func (rs *Resource) previewChildQuota(ctx context.Context, requested int) (*childQuotaPreview, error) {
	quota, limited, err := rs.runtime.ChildQuota(ctx)
	if err != nil || !limited {
		return nil, err
	}
	return &childQuotaPreview{
		BookedPlaces: quota.Booked, OccupiedPlaces: quota.Occupied, RequestedPlaces: requested,
		FreePlaces: quota.Free, Fits: quota.Admit(requested) == nil,
	}, nil
}

// admitStudentImport refuses an import whose new children do not all fit
// into the Kinderkontingent before its first batch runs, so an import is
// applied whole or not at all (#3571). It counts them with the preview's dry
// run, in every mode: an update can resume a child's care. Every batch still
// checks under the quota lock, which catches children created by others
// between this check and the batch.
// It returns the owner's refusal, or nil when the import may start.
func (rs *Resource) admitStudentImport(ctx context.Context, request importModels.ImportRequest[importModels.StudentImportRow]) error {
	quota, limited, err := rs.runtime.ChildQuota(ctx)
	if err != nil || !limited {
		return err
	}
	// The dry run may normalize its rows; the batches get the upload as sent.
	request.Rows, request.DryRun = slices.Clone(request.Rows), true
	dryRun, err := rs.studentImportService.Import(ctx, request)
	if err != nil {
		return err
	}
	return quota.Admit(dryRun.ChildQuotaRequested)
}
