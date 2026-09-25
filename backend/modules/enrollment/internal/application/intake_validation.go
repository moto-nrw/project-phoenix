package application

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/contact"
)

// validateSubmission checks the guardian, the consents and the children of a
// submission against the tenant's form capabilities. The phase window is the
// caller's check.
func (s *Intake) validateSubmission(ctx context.Context, req SubmitRequest, legalBlocks []enrollment.LegalBlock, capabilities enrollment.FormCapabilities) error {
	if !s.isEnrollmentEnabled(ctx) {
		return enrollment.ErrEnrollmentDisabled
	}
	if req.PhaseID <= 0 {
		return fmt.Errorf("%w: phase_id is required", enrollment.ErrInvalidSubmission)
	}
	if err := validateSubmissionGuardian(req); err != nil {
		return err
	}
	for _, key := range requiredConsentKeys(legalBlocks) {
		accepted, ok := req.ConsentFlags[key].(bool)
		if !ok || !accepted {
			return fmt.Errorf("%w: consent %s is required", enrollment.ErrInvalidSubmission, key)
		}
	}
	if len(req.Children) == 0 {
		return fmt.Errorf("%w: at least one child is required", enrollment.ErrInvalidSubmission)
	}
	gradeMax, err := s.resolveGradeMax(ctx)
	if err != nil {
		return err
	}
	for i, child := range req.Children {
		if err := validateSubmissionChild(i, child, capabilities, gradeMax); err != nil {
			return err
		}
	}
	return nil
}

// validateSubmissionGuardian checks the primary guardian against the same
// canonical email and phone formats the approval enforces, so a submittable
// request can never get stuck at the approval step.
func validateSubmissionGuardian(req SubmitRequest) error {
	if strings.TrimSpace(req.GuardianFirstName) == "" {
		return fmt.Errorf("%w: guardian first name is required", enrollment.ErrInvalidSubmission)
	}
	if strings.TrimSpace(req.GuardianLastName) == "" {
		return fmt.Errorf("%w: guardian last name is required", enrollment.ErrInvalidSubmission)
	}
	emailAddr := strings.TrimSpace(req.GuardianEmail)
	if emailAddr == "" {
		return fmt.Errorf("%w: guardian email is required", enrollment.ErrInvalidSubmission)
	}
	if err := contact.ValidateOptionalEmail(emailAddr); err != nil {
		return enrollment.ErrInvalidGuardianEmail
	}
	if req.GuardianPhone != nil {
		if err := contact.ValidateOptionalPhone(*req.GuardianPhone); err != nil {
			return enrollment.ErrInvalidGuardianPhone
		}
	}
	return nil
}

func validateSubmissionChild(i int, child SubmitChild, capabilities enrollment.FormCapabilities, gradeMax int) error {
	if strings.TrimSpace(child.FirstName) == "" || strings.TrimSpace(child.LastName) == "" {
		return fmt.Errorf("%w: child %d missing name", enrollment.ErrInvalidSubmission, i)
	}
	if child.DateOfBirth.IsZero() {
		return fmt.Errorf("%w: child %d missing date_of_birth", enrollment.ErrInvalidSubmission, i)
	}
	if capabilities.CollectGradeLevel && child.TargetGradeLevel == nil {
		return fmt.Errorf("%w: child %d missing target_grade_level", enrollment.ErrInvalidSubmission, i)
	}
	if capabilities.CollectGradeLevel && (*child.TargetGradeLevel < 1 || int(*child.TargetGradeLevel) > gradeMax) {
		return fmt.Errorf("%w: child %d grade out of range 1..%d", enrollment.ErrInvalidSubmission, i, gradeMax)
	}
	return nil
}

// normalizeAdditionalGuardians cleans the co-guardians in place: it trims
// every field, drops fully blank cards, refuses a half-filled card or a
// malformed email or phone, and drops co-guardians that duplicate the primary
// guardian or each other, so an approval never links one profile twice.
func normalizeAdditionalGuardians(req *SubmitRequest) error {
	if len(req.AdditionalGuardians) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(req.AdditionalGuardians)+1)
	primaryPhone := ""
	if req.GuardianPhone != nil {
		primaryPhone = strings.TrimSpace(*req.GuardianPhone)
	}
	if primary := guardianDedupKey(req.GuardianEmail, primaryPhone, req.GuardianFirstName, req.GuardianLastName); primary != "" {
		seen[primary] = struct{}{}
	}
	cleaned := make([]SubmitGuardian, 0, len(req.AdditionalGuardians))
	for i, g := range req.AdditionalGuardians {
		out, keep, err := normalizeAdditionalGuardian(i, g)
		if err != nil {
			return err
		}
		if !keep {
			continue
		}
		key := guardianDedupKey(optionalString(out.Email), optionalString(out.Phone), out.FirstName, out.LastName)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		cleaned = append(cleaned, out)
	}
	req.AdditionalGuardians = cleaned
	return nil
}

// normalizeAdditionalGuardian trims and validates one co-guardian card. A
// fully blank card is dropped; only the names are required.
func normalizeAdditionalGuardian(index int, g SubmitGuardian) (SubmitGuardian, bool, error) {
	first := strings.TrimSpace(g.FirstName)
	last := strings.TrimSpace(g.LastName)
	email := strings.ToLower(strings.TrimSpace(optionalString(g.Email)))
	phone := strings.TrimSpace(optionalString(g.Phone))
	if first == "" && last == "" && email == "" && phone == "" {
		return SubmitGuardian{}, false, nil
	}
	if first == "" || last == "" {
		return SubmitGuardian{}, false, fmt.Errorf("%w: additional guardian %d missing name", enrollment.ErrInvalidSubmission, index)
	}
	if email != "" {
		if err := contact.ValidateOptionalEmail(email); err != nil {
			return SubmitGuardian{}, false, enrollment.ErrInvalidGuardianEmail
		}
	}
	if phone != "" {
		if err := contact.ValidateOptionalPhone(phone); err != nil {
			return SubmitGuardian{}, false, enrollment.ErrInvalidGuardianPhone
		}
	}
	out := SubmitGuardian{FirstName: first, LastName: last}
	if email != "" {
		out.Email = &email
	}
	if phone != "" {
		out.Phone = &phone
	}
	return out, true, nil
}

func optionalString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// guardianDedupKey is a guardian's stable identity: the lower-cased email
// when present, otherwise the case-folded full name plus the phone, so two
// same-name co-guardians with different phones stay distinct. "" only when
// every input is empty.
func guardianDedupKey(email, phone, first, last string) string {
	if e := strings.ToLower(strings.TrimSpace(email)); e != "" {
		return "e:" + e
	}
	f := strings.ToLower(strings.TrimSpace(first))
	l := strings.ToLower(strings.TrimSpace(last))
	p := strings.TrimSpace(phone)
	if f == "" && l == "" && p == "" {
		return ""
	}
	return "n:" + f + "|" + l + "|p:" + p
}

// requiredConsentKeys are the consents the parent must accept, so a hidden
// or empty block never blocks a submission.
func requiredConsentKeys(blocks []enrollment.LegalBlock) []string {
	required := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if block.Required {
			required = append(required, block.Key)
		}
	}
	return required
}

// filterConsentFlags drops every consent key the legal contract does not
// declare; the flags are legally meaningful data.
func filterConsentFlags(flags map[string]any, blocks []enrollment.LegalBlock) map[string]any {
	if len(flags) == 0 {
		return flags
	}
	allowed := make(map[string]bool, len(blocks))
	for _, block := range blocks {
		allowed[block.Key] = true
	}
	out := make(map[string]any, len(flags))
	for key, value := range flags {
		if allowed[key] {
			out[key] = value
		}
	}
	return out
}

func ensureRequiredConsentFlags(flags map[string]any, legalBlocks []enrollment.LegalBlock) map[string]any {
	out := cloneSourceMetadata(flags)
	for _, key := range requiredConsentKeys(legalBlocks) {
		out[key] = true
	}
	return out
}

// legalBlocksSnapshotEntry freezes the legal blocks and the guardian's
// filtered consent flags into the append-only evidence entry of the request
// (Art. 5 Abs. 2, Art. 7 Abs. 1 DSGVO). It is the only reliable record of the
// wording the family saw and the answers it gave at that moment.
func legalBlocksSnapshotEntry(blocks []enrollment.LegalBlock, flags map[string]any, at time.Time) (enrollmentModels.LegalBlocksSnapshotEntry, error) {
	snapshot := make([]enrollmentModels.LegalBlockSnapshot, 0, len(blocks))
	for _, block := range blocks {
		snapshot = append(snapshot, enrollmentModels.LegalBlockSnapshot{
			Key: block.Key, Kind: block.Kind, Title: block.Title, Label: block.Label, Text: block.Text,
			Required: block.Required, Source: block.Source, Translations: block.Translations.Texts(),
		})
	}
	if flags == nil {
		flags = map[string]any{}
	}
	frozenFlags, err := json.Marshal(flags)
	if err != nil {
		return enrollmentModels.LegalBlocksSnapshotEntry{}, err
	}
	return enrollmentModels.LegalBlocksSnapshotEntry{SnapshotAt: at, Blocks: snapshot, ConsentFlags: frozenFlags}, nil
}

// normalizeSubmissionForCapabilities drops the grade and class when the grade
// is not collected and refuses offering picks while care offerings are off,
// with Care Plan's shared value.
func normalizeSubmissionForCapabilities(req *SubmitRequest, capabilities enrollment.FormCapabilities) error {
	for i := range req.Children {
		child := &req.Children[i]
		if !capabilities.CollectGradeLevel {
			child.TargetGradeLevel = nil
			child.TargetSchoolClass = nil
		}
		if !capabilities.CareOfferingsEnabled {
			if len(child.OfferingIDs) > 0 || len(child.OfferingDays) > 0 {
				return careplan.ErrCareOfferingsDisabled
			}
			child.OfferingIDs = nil
			child.OfferingDays = nil
		}
	}
	return nil
}
