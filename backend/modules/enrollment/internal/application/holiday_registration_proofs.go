package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// The proof of benefits of the public holiday registration (#5238). The
// upload reuses the rules of the file field of the online registration
// (#4783): limits, display name, upload bucket per IP and the single-use
// token the submission claims. The bytes stay in the Träger's row.

// holidayRateLimitKeyUploadIP is the upload bucket of the public link.
const holidayRateLimitKeyUploadIP = enrollment.SubmissionRateLimitKeyTypeUploadIP

// HolidayProofStore persists the proofs of benefits of a Träger.
type HolidayProofStore interface {
	// InsertHolidayProof stores an unclaimed proof of the period with the
	// SHA-256 of its token.
	InsertHolidayProof(ctx context.Context, organizationID, periodID int64, proof enrollment.HolidayProofUpload, tokenHash string) error
	// ClaimHolidayProofs binds the unclaimed proofs of the period whose
	// token hash is listed and that are younger than
	// enrollment.UnclaimedUploadLifetime to the registration, and returns
	// how many it bound.
	ClaimHolidayProofs(ctx context.Context, organizationID, periodID, registrationID int64, tokenHashes []string) (int, error)
}

// CountHolidayProofUpload counts the upload in the IP's upload bucket of the
// Träger, with the window and limit of the online registration's uploads.
func (h *HolidayRegistrations) CountHolidayProofUpload(ctx context.Context, organizationID int64, remoteIP string) error {
	if organizationID <= 0 {
		return errHolidayPeriodRequired
	}
	remoteIP = strings.TrimSpace(remoteIP)
	if remoteIP == "" {
		return nil
	}
	attempts, err := h.deps.Registrations.CountHolidayRegistrationAttempt(ctx, organizationID, holidayRateLimitKeyUploadIP, remoteIP, rateLimitWindowUploadIP)
	if err != nil {
		return fmt.Errorf("enrollment: count holiday proof upload: %w", err)
	}
	if attempts > rateLimitMaxAttemptsUploadIP {
		h.deps.Logger.Info("holiday proof upload rate-limited",
			slog.Int("attempts", attempts),
			slog.Int64("organization_id", organizationID))
		return enrollment.ErrUploadRateLimited
	}
	return nil
}

// UploadHolidayProof stores the file as an unclaimed proof of the link's
// period while its registration window is open.
func (h *HolidayRegistrations) UploadHolidayProof(ctx context.Context, link enrollment.HolidayRegistrationLink, upload enrollment.HolidayProofUpload) (enrollment.HolidayProofReceipt, error) {
	if h.deps.Random == nil || h.deps.Fingerprint == nil {
		return enrollment.HolidayProofReceipt{}, errors.New("enrollment: holiday proof uploads are not configured")
	}
	period, err := h.publishedPeriod(ctx, link)
	if err != nil {
		return enrollment.HolidayProofReceipt{}, err
	}
	today := h.deps.Today()
	if today.Before(period.RegistrationOpensOn) || today.After(period.RegistrationClosesOn) {
		return enrollment.HolidayProofReceipt{}, enrollment.ErrHolidayRegistrationClosed
	}
	options := enrollment.HolidayProofOptions
	size := int64(len(upload.Content))
	if size <= 0 || size != upload.SizeBytes || size > options.MaxBytes() || !options.AllowsContentType(upload.ContentType) {
		return enrollment.HolidayProofReceipt{}, &enrollment.UploadRejection{MaxMB: options.MaxSizeMB, AllowedTypes: options.AllowedTypes}
	}
	token, err := enrollment.NewStatusToken(h.deps.Random)
	if err != nil {
		return enrollment.HolidayProofReceipt{}, fmt.Errorf("enrollment: generate holiday proof token: %w", err)
	}
	upload.Filename = uploadDisplayName(upload.Filename, upload.Extension)
	if err := h.deps.Proofs.InsertHolidayProof(ctx, link.OrganizationID, link.PeriodID, upload, h.holidayProofTokenHash(token)); err != nil {
		return enrollment.HolidayProofReceipt{}, fmt.Errorf("enrollment: store holiday proof: %w", err)
	}
	return enrollment.HolidayProofReceipt{Token: token, Filename: upload.Filename, SizeBytes: size, ContentType: upload.ContentType}, nil
}

// claimHolidayProofs binds the proofs the submission names to its
// registration. A token that claims nothing refuses the whole registration,
// so no parent believes a proof arrived that did not.
func (h *HolidayRegistrations) claimHolidayProofs(ctx context.Context, link enrollment.HolidayRegistrationLink, registrationID int64, tokens []string) error {
	if len(tokens) == 0 {
		return nil
	}
	if h.deps.Fingerprint == nil {
		return errors.New("enrollment: holiday proof uploads are not configured")
	}
	hashes := make([]string, len(tokens))
	for i, token := range tokens {
		hashes[i] = h.holidayProofTokenHash(token)
	}
	claimed, err := h.deps.Proofs.ClaimHolidayProofs(ctx, link.OrganizationID, link.PeriodID, registrationID, hashes)
	if err != nil {
		return fmt.Errorf("enrollment: claim holiday proofs: %w", err)
	}
	if claimed != len(hashes) {
		return enrollment.ErrHolidayProofExpired
	}
	h.deps.Logger.Info("holiday proofs claimed",
		slog.Int64("organization_id", link.OrganizationID),
		slog.Int64("registration_id", registrationID),
		slog.Int("proofs", claimed))
	return nil
}

// normalizeHolidayProofTokens trims the tokens and drops empty and repeated
// ones.
func normalizeHolidayProofTokens(tokens []string) []string {
	out := make([]string, 0, len(tokens))
	seen := map[string]bool{}
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" || seen[token] {
			continue
		}
		seen[token] = true
		out = append(out, token)
	}
	return out
}

// validateHolidayProofTokens refuses proofs without the statement on
// benefits and more files than a multiple file field takes.
func validateHolidayProofTokens(input enrollment.HolidayRegistrationInput, errs validation.Errors) {
	switch {
	case len(input.BenefitsProofTokens) > 0 && !input.BenefitsDeclared:
		errs["benefits_proofs"] = errors.New("a proof needs the statement that the family receives benefits")
	case len(input.BenefitsProofTokens) > enrollment.HolidayProofOptions.MaxFiles():
		errs["benefits_proofs"] = fmt.Errorf("at most %d files", enrollment.HolidayProofOptions.MaxFiles())
	}
}

// holidayProofTokenHash is the stored identity of a proof token: its
// fingerprint, like the tokens of the online registration's uploads.
func (h *HolidayRegistrations) holidayProofTokenHash(token string) string {
	return h.deps.Fingerprint([]byte(token))
}
