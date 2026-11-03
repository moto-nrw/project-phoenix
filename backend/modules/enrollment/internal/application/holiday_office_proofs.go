package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// The proof of benefits the Träger-Büro uploads (#5627): a family brings it
// on paper, the office scans it and keeps it with the registration, beside
// the proofs the parents uploaded themselves (#5238). It has the same limits
// and goes with the registration (ADR 0118). A proof is no condition of a
// cost takeover.

// UploadHolidayRegistrationProof stores the file as a proof of the
// registration and records the upload in its history.
func (h *HolidayRegistrationReview) UploadHolidayRegistrationProof(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID, registrationID int64, upload enrollment.HolidayProofUpload) (enrollment.HolidayRegistrationDetail, error) {
	options := enrollment.HolidayProofOptions
	size := int64(len(upload.Content))
	if size <= 0 || size != upload.SizeBytes || size > options.MaxBytes() || !options.AllowsContentType(upload.ContentType) {
		return enrollment.HolidayRegistrationDetail{}, &enrollment.UploadRejection{MaxMB: options.MaxSizeMB, AllowedTypes: options.AllowedTypes}
	}
	upload.Filename = uploadDisplayName(upload.Filename, upload.Extension)
	return h.change(ctx, reviewer, periodID, registrationID, func(_ enrollment.HolidayPeriod, current holidayRegistrationState) (string, any, error) {
		existing, err := h.store.ReadHolidayProofs(ctx, reviewer.OrganizationID, current.ID)
		if err != nil {
			return "", nil, fmt.Errorf("enrollment: read holiday registration proofs: %w", err)
		}
		if len(existing) >= options.MaxFiles() {
			return "", nil, enrollment.ErrHolidayProofLimit
		}
		proofID, err := h.store.InsertClaimedHolidayProof(ctx, reviewer.OrganizationID, periodID, current.ID, upload)
		if err != nil {
			return "", nil, fmt.Errorf("enrollment: store holiday registration proof: %w", err)
		}
		return enrollment.HolidayAuditProofUpload, map[string]any{"proof_id": proofID}, nil
	})
}

// HolidayProofRegistrations names the registrations of a period of the
// reviewer's Träger with a proof, for the list a cost bearer gets (#5627),
// and records the read in the period's trail like a list.
func (h *HolidayRegistrationReview) HolidayProofRegistrations(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID int64) (map[int64]bool, error) {
	if err := holidayReviewKey(reviewer, periodID, 1); err != nil {
		return nil, err
	}
	if _, err := h.period(ctx, reviewer.OrganizationID, periodID, false); err != nil {
		return nil, err
	}
	proofs, err := h.store.HolidayProofRegistrations(ctx, reviewer.OrganizationID, periodID)
	if err != nil {
		return nil, fmt.Errorf("enrollment: read holiday proof registrations: %w", err)
	}
	if err := h.audit(ctx, reviewer, periodID, 0, enrollment.HolidayAuditList, map[string]any{"proofs": len(proofs)}); err != nil {
		return nil, err
	}
	return proofs, nil
}
