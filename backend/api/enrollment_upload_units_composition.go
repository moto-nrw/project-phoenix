package api

import (
	"context"

	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// enrollmentUploadUnits opens the tenant units of the files parents attach
// to an enrollment (#4783). The upload row commits in its own unit before the
// route writes the bytes; a failed write discards the row in a new one. A
// staff open loads the upload and writes its access record in one unit, so
// no file is served without a committed record.
type enrollmentUploadUnits struct {
	intake  enrollmentOwner.UploadIntake
	records enrollmentOwner.RequestUploadRecords
	reads   enrollmentOwner.RequestUploadReads
}

func (u enrollmentUploadUnits) PrepareUpload(ctx context.Context, input enrollmentOwner.UploadInput) (*enrollmentOwner.PreparedUpload, error) {
	id, err := tenant.NewTenantID(input.TenantID)
	if err != nil {
		return nil, err
	}
	var prepared *enrollmentOwner.PreparedUpload
	err = tenant.WithinTenant(ctx, id, func(txCtx context.Context) error {
		var prepareErr error
		prepared, prepareErr = u.intake.PrepareUpload(txCtx, input)
		return prepareErr
	})
	return prepared, err
}

func (u enrollmentUploadUnits) DiscardUpload(ctx context.Context, tenantID, uploadID int64) error {
	id, err := tenant.NewTenantID(tenantID)
	if err != nil {
		return err
	}
	return tenant.WithinTenant(ctx, id, func(txCtx context.Context) error {
		return u.records.DeleteRequestUpload(txCtx, uploadID)
	})
}

func (u enrollmentUploadUnits) OpenRequestUpload(ctx context.Context, access enrollmentOwner.UploadAccess) (*enrollmentOwner.RequestUpload, error) {
	var upload *enrollmentOwner.RequestUpload
	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		var openErr error
		upload, openErr = u.reads.OpenRequestUpload(txCtx, access)
		return openErr
	})
	return upload, err
}
