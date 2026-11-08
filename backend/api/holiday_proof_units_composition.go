package api

import (
	"context"

	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// holidayProofUnits opens the Träger unit of a proof of benefits the public
// holiday registration uploads (#5238). Enrollment counts the upload before,
// in a unit of its own. The link names the Träger, never the request.
type holidayProofUnits struct {
	registrations enrollmentOwner.HolidayRegistrations
}

func (u holidayProofUnits) UploadHolidayProof(ctx context.Context, link enrollmentOwner.HolidayRegistrationLink, upload enrollmentOwner.HolidayProofUpload) (enrollmentOwner.HolidayProofReceipt, error) {
	var receipt enrollmentOwner.HolidayProofReceipt
	err := tenant.WithinTraeger(ctx, link.OrganizationID, func(txCtx context.Context) error {
		var uploadErr error
		receipt, uploadErr = u.registrations.UploadHolidayProof(txCtx, link, upload)
		return uploadErr
	})
	return receipt, err
}
