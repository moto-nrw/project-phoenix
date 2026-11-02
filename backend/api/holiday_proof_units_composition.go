package api

import (
	"context"
	"errors"

	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// holidayProofUnits opens the Träger units of a proof of benefits the public
// holiday registration uploads (#5238), like the units of the online
// registration's uploads (#4783): the upload counts in a unit of its own,
// which commits even when it refuses, so a refused upload still counts; the
// proof then commits in a unit of its own. The link names the Träger, never
// the request.
type holidayProofUnits struct {
	registrations enrollmentOwner.HolidayRegistrations
}

func (u holidayProofUnits) UploadHolidayProof(ctx context.Context, link enrollmentOwner.HolidayRegistrationLink, remoteIP string, upload enrollmentOwner.HolidayProofUpload) (enrollmentOwner.HolidayProofReceipt, error) {
	var limitErr error
	err := tenant.WithinTraeger(ctx, link.OrganizationID, func(txCtx context.Context) error {
		limitErr = u.registrations.CountHolidayProofUpload(txCtx, link.OrganizationID, remoteIP)
		if errors.Is(limitErr, enrollmentOwner.ErrUploadRateLimited) {
			return nil
		}
		return limitErr
	})
	if err == nil {
		err = limitErr
	}
	if err != nil {
		return enrollmentOwner.HolidayProofReceipt{}, err
	}
	var receipt enrollmentOwner.HolidayProofReceipt
	err = tenant.WithinTraeger(ctx, link.OrganizationID, func(txCtx context.Context) error {
		var uploadErr error
		receipt, uploadErr = u.registrations.UploadHolidayProof(txCtx, link, upload)
		return uploadErr
	})
	return receipt, err
}
