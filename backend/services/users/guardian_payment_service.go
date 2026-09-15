package users

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Guardian payment data (#2608). Two hard rules, carried over from the staff
// bank & tax path (#1423): no change without an audit row in the same tenant
// transaction, and no read of bank data without a data-access log row — the
// service refuses to serve an IBAN when the audit write fails.
//
// The IBAN belongs to the guardian; which of a child's guardians is charged
// belongs to the relationship (StudentGuardian.IsPayer). Siblings therefore
// share one maintained IBAN and carry the mark once per child.

// Payment input errors — HTTP 400. Each is its own sentinel so the handler can
// render one specific German sentence: these messages are shown to a school
// office worker, not to a developer, and a wrapped "invalid payment value:
// malformed IBAN" told them nothing they could act on.
var (
	// ErrGuardianPaymentInvalid is the class marker every input error wraps,
	// so callers can classify without listing each case.
	ErrGuardianPaymentInvalid = errors.New("invalid payment value")

	// ErrGuardianIBANInvalid marks an IBAN that fails the structural check or
	// the ISO 13616 mod-97 checksum.
	ErrGuardianIBANInvalid = fmt.Errorf("%w: malformed IBAN", ErrGuardianPaymentInvalid)

	// ErrGuardianAccountHolderTooLong marks a Kontoinhaber over the column bound.
	ErrGuardianAccountHolderTooLong = fmt.Errorf("%w: account holder too long", ErrGuardianPaymentInvalid)

	// ErrGuardianStudentRequired marks a missing student id.
	ErrGuardianStudentRequired = fmt.Errorf("%w: student id is required", ErrGuardianPaymentInvalid)
)

// ErrGuardianNotLinkedToStudent is returned when a guardian is named as payer
// for a child they are not linked to — HTTP 400 rather than 404: the caller
// asked for an assignment that cannot exist.
var ErrGuardianNotLinkedToStudent = errors.New("guardian is not linked to this student")

// guardianIBANPattern is the structural IBAN shape (ISO 13616): country code,
// two check digits, then 11–30 alphanumerics. The mod-97 checksum is verified
// separately by ibanChecksumValid.
var guardianIBANPattern = regexp.MustCompile(`^[A-Z]{2}\d{2}[A-Z0-9]{11,30}$`)

// guardianIBANLengths contains the ISO 13616 lengths for countries whose
// accounts can be entered in moto. A checksum alone cannot detect a truncated
// German IBAN that happens to satisfy mod-97.
var guardianIBANLengths = map[string]int{
	"AD": 24, "AT": 20, "BE": 16, "BG": 22, "CH": 21, "CY": 28,
	"CZ": 24, "DE": 22, "DK": 18, "EE": 20, "ES": 24, "FI": 18,
	"FR": 27, "GB": 22, "GI": 23, "GR": 27, "HR": 21, "HU": 28,
	"IE": 22, "IS": 26, "IT": 27, "LI": 21, "LT": 20, "LU": 20,
	"LV": 21, "MC": 27, "MT": 31, "NL": 18, "NO": 15, "PL": 28,
	"PT": 25, "RO": 24, "SE": 24, "SI": 19, "SK": 24,
}

// maxAccountHolderLen bounds the Kontoinhaber free-text. The column is TEXT;
// the bound keeps a paste accident from storing an unbounded payload.
const maxAccountHolderLen = 120

// GuardianPaymentInput is the full bank section of a guardian; nil clears a
// field.
type GuardianPaymentInput struct {
	IBAN          *string
	AccountHolder *string
}

// GuardianPaymentMasked is the default (masked) read: the IBAN shows its last
// four characters only. The Kontoinhaber is a name, not an access credential,
// and is served in full. The unmasked IBAN requires RevealGuardianPayment.
type GuardianPaymentMasked struct {
	GuardianProfileID int64
	IBANMasked        *string
	AccountHolder     *string
}

// GuardianPaymentPlain carries the unmasked values after an audited reveal.
type GuardianPaymentPlain struct {
	GuardianProfileID int64
	IBAN              *string
	AccountHolder     *string
}

// GuardianPaymentRow is one line of the Bankverbindungen list: a child, the
// guardian charged for it, and that guardian's IBAN. GuardianProfileID is nil
// when no payer is assigned — those rows are the point of the list, not noise.
// IBAN carries the full value ONLY on the export path; the on-screen list
// fills IBANMasked and leaves it empty.
type GuardianPaymentRow struct {
	StudentID         int64
	StudentName       string
	SchoolClass       string
	GuardianProfileID *int64
	GuardianName      string
	RelationshipType  string
	// AccountHolder is the resolved Kontoinhaber: the explicit override when
	// the account runs on another name, otherwise the guardian's own name.
	AccountHolder string
	IBAN          string
	IBANMasked    string
}

// HasIBAN reports whether a usable bank account is stored for this row.
func (r GuardianPaymentRow) HasIBAN() bool {
	return r.IBAN != "" || r.IBANMasked != ""
}

// GetGuardianPaymentMasked serves the masked bank details and logs the read.
// No data leaves without a successful audit row.
func (s *GuardianService) GetGuardianPaymentMasked(ctx context.Context, guardianProfileID int64, actorAccountID int64, actorRole string) (*GuardianPaymentMasked, error) {
	data, err := s.loadPaymentAudited(ctx, guardianProfileID, actorAccountID, actorRole, auditModels.ResourceTypeGuardianFinancialView)
	if err != nil {
		return nil, err
	}
	masked := &GuardianPaymentMasked{GuardianProfileID: guardianProfileID}
	if data != nil {
		masked.IBANMasked = maskTailPtr(data.IBAN, 4)
		masked.AccountHolder = data.AccountHolder
	}
	return masked, nil
}

// RevealGuardianPayment serves the full IBAN after the explicit "Anzeigen"
// toggle and logs the reveal.
func (s *GuardianService) RevealGuardianPayment(ctx context.Context, guardianProfileID int64, actorAccountID int64, actorRole string) (*GuardianPaymentPlain, error) {
	data, err := s.loadPaymentAudited(ctx, guardianProfileID, actorAccountID, actorRole, auditModels.ResourceTypeGuardianFinancialReveal)
	if err != nil {
		return nil, err
	}
	plain := &GuardianPaymentPlain{GuardianProfileID: guardianProfileID}
	if data != nil {
		plain.IBAN = data.IBAN
		plain.AccountHolder = data.AccountHolder
	}
	return plain, nil
}

// UpdateGuardianPayment replaces the bank section of one guardian. Audit rows
// carry masked values only and record the authenticated account as actor.
func (s *GuardianService) UpdateGuardianPayment(ctx context.Context, guardianProfileID int64, input GuardianPaymentInput, changedByAccountID int64, note string) error {
	normalized, err := normalizeGuardianPaymentInput(input)
	if err != nil {
		return err
	}
	if s.GuardianFinancialAudit == nil {
		return fmt.Errorf("guardian financial audit repository is not wired; refusing unaudited change")
	}
	if changedByAccountID <= 0 {
		return fmt.Errorf("changed-by actor id is required")
	}

	tenantID := tenant.FromContext(ctx)
	return tenant.WithTenantTx(ctx, s.DB, tenantID, func(ctx context.Context, _ bun.Tx) error {
		// Lock the guardian row so two concurrent editors of the same bank
		// section serialize: the second reads the first's committed values and
		// its diff is against reality, not against a stale snapshot.
		if err := s.GuardianProfileRepo.LockByIDForUpdate(ctx, guardianProfileID); err != nil {
			return err
		}
		if _, err := s.GuardianProfileRepo.FindByID(ctx, guardianProfileID); err != nil {
			return err
		}

		data, err := s.GuardianFinancialRepo.FindByGuardianProfileID(ctx, guardianProfileID)
		if err != nil {
			return err
		}

		var current users.GuardianFinancialData
		if data != nil {
			current = *data
		}

		// The no-op decision compares plaintext (a masked comparison would miss
		// a change with identical last-4), but the audit rows carry masked
		// values only — the trail must not become a second store of bank data
		// outside the guardians:financial gate.
		var changes []stammdatenChange
		if strPtrAuditValue(current.IBAN) != strPtrAuditValue(normalized.IBAN) {
			changes = append(changes, maskedChange(auditModels.GuardianPaymentFieldIBAN,
				maskTailPtr(current.IBAN, 4), maskTailPtr(normalized.IBAN, 4)))
		}
		if strPtrAuditValue(current.AccountHolder) != strPtrAuditValue(normalized.AccountHolder) {
			changes = append(changes, maskedChange(auditModels.GuardianPaymentFieldAccountHolder,
				maskAllPtr(current.AccountHolder), maskAllPtr(normalized.AccountHolder)))
		}
		if len(changes) == 0 {
			return nil
		}

		trimmedNote := strings.TrimSpace(note)
		for _, change := range changes {
			if err := s.GuardianFinancialAudit.Create(ctx, &auditModels.GuardianFinancialChange{
				GuardianProfileID: guardianProfileID,
				ChangedBy:         changedByAccountID,
				FieldName:         change.field,
				OldValue:          change.oldValue,
				NewValue:          change.newValue,
				Note:              trimmedNote,
			}); err != nil {
				return fmt.Errorf("write guardian payment audit: %w", err)
			}
		}

		if data == nil {
			fresh := &users.GuardianFinancialData{GuardianProfileID: guardianProfileID}
			fresh.IBAN = normalized.IBAN
			fresh.AccountHolder = normalized.AccountHolder
			return s.GuardianFinancialRepo.Create(ctx, fresh)
		}
		data.IBAN = normalized.IBAN
		data.AccountHolder = normalized.AccountHolder
		return s.GuardianFinancialRepo.Update(ctx, data)
	})
}

// SetStudentPayer marks which guardian's account is charged for one child, or
// clears the assignment when guardianProfileID is nil. The clear and the set
// share one tenant transaction with the audit row.
func (s *GuardianService) SetStudentPayer(ctx context.Context, studentID int64, guardianProfileID *int64, changedByAccountID int64) error {
	if studentID <= 0 {
		return ErrGuardianStudentRequired
	}
	if s.GuardianFinancialAudit == nil {
		return fmt.Errorf("guardian financial audit repository is not wired; refusing unaudited change")
	}
	if changedByAccountID <= 0 {
		return fmt.Errorf("changed-by actor id is required")
	}

	tenantID := tenant.FromContext(ctx)
	return tenant.WithTenantTx(ctx, s.DB, tenantID, func(ctx context.Context, _ bun.Tx) error {
		// The student row is the serialization point for every payer change.
		// Lock it before loading relationships so a waiting assignment observes
		// the committed payer and records an audit trail that matches reality.
		if _, err := s.StudentRepo.FindByIDForUpdate(ctx, studentID); err != nil {
			return err
		}

		relationships, err := s.StudentGuardianRepo.FindByStudentID(ctx, studentID)
		if err != nil {
			return err
		}

		var currentPayer *int64
		linked := false
		for _, rel := range relationships {
			if rel.IsPayer {
				id := rel.GuardianProfileID
				currentPayer = &id
			}
			if guardianProfileID != nil && rel.GuardianProfileID == *guardianProfileID {
				linked = true
			}
		}
		if guardianProfileID != nil && !linked {
			return ErrGuardianNotLinkedToStudent
		}
		if samePayer(currentPayer, guardianProfileID) {
			return nil
		}

		// One audit row per side of the move, so the trail reads "X stopped
		// paying" / "Y started paying" without the reader having to diff two
		// rows against each other.
		if currentPayer != nil {
			if err := s.recordPayerChange(ctx, *currentPayer, studentID, changedByAccountID, "true", "false"); err != nil {
				return err
			}
		}
		if guardianProfileID != nil {
			if err := s.recordPayerChange(ctx, *guardianProfileID, studentID, changedByAccountID, "false", "true"); err != nil {
				return err
			}
		}

		return s.StudentGuardianRepo.SetPayer(ctx, studentID, guardianProfileID)
	})
}

// ListPaymentOverview serves the school-wide Bankverbindungen list with MASKED
// IBANs and logs the read.
func (s *GuardianService) ListPaymentOverview(ctx context.Context, actorAccountID int64, actorRole string) ([]GuardianPaymentRow, error) {
	return s.buildPaymentRows(ctx, actorAccountID, actorRole, false, auditModels.ResourceTypeGuardianPaymentOverview, nil)
}

// ListPaymentExportRows serves the same list with UNMASKED IBANs for the file
// export and logs it as its own resource type — a bulk export of bank data is
// a materially different event from viewing the masked list.
func (s *GuardianService) ListPaymentExportRows(ctx context.Context, actorAccountID int64, actorRole string, format string) ([]GuardianPaymentRow, error) {
	return s.buildPaymentRows(ctx, actorAccountID, actorRole, true, auditModels.ResourceTypeGuardianPaymentExport, map[string]interface{}{
		"format": format,
	})
}

// --- internals -------------------------------------------------------------

// buildPaymentRows assembles the Bankverbindungen rows and writes the
// data-access log entry. Nothing is returned when the audit write fails.
func (s *GuardianService) buildPaymentRows(
	ctx context.Context,
	actorAccountID int64,
	actorRole string,
	unmasked bool,
	resourceType string,
	extraMetadata map[string]interface{},
) ([]GuardianPaymentRow, error) {
	var result []GuardianPaymentRow
	err := s.txHandler.RunInTx(ctx, func(txCtx context.Context) error {
		var err error
		result, err = s.buildPaymentRowsInTx(txCtx, actorAccountID, actorRole, unmasked, resourceType, extraMetadata)
		return err
	})
	return result, err
}

func (s *GuardianService) buildPaymentRowsInTx(
	ctx context.Context,
	actorAccountID int64,
	actorRole string,
	unmasked bool,
	resourceType string,
	extraMetadata map[string]interface{},
) ([]GuardianPaymentRow, error) {
	if s.DataAccessLog == nil {
		return nil, fmt.Errorf("data access log repository is not wired; refusing unaudited payment read")
	}
	if actorAccountID <= 0 {
		return nil, fmt.Errorf("actor account id is required for payment reads")
	}

	assignments, err := s.StudentGuardianRepo.ListPaymentAssignments(ctx)
	if err != nil {
		return nil, err
	}

	guardianIDs := make([]int64, 0, len(assignments))
	seen := make(map[int64]struct{}, len(assignments))
	for _, a := range assignments {
		if a.GuardianProfileID == nil {
			continue
		}
		if _, ok := seen[*a.GuardianProfileID]; ok {
			continue
		}
		seen[*a.GuardianProfileID] = struct{}{}
		guardianIDs = append(guardianIDs, *a.GuardianProfileID)
	}

	financial, err := s.GuardianFinancialRepo.ListByGuardianProfileIDs(ctx, guardianIDs)
	if err != nil {
		return nil, err
	}

	rows := make([]GuardianPaymentRow, 0, len(assignments))
	for _, a := range assignments {
		row := GuardianPaymentRow{
			StudentID:        a.StudentID,
			StudentName:      joinName(a.StudentFirstName, a.StudentLastName),
			SchoolClass:      a.SchoolClass,
			RelationshipType: a.RelationshipTypeRaw,
		}
		if a.GuardianProfileID != nil {
			id := *a.GuardianProfileID
			row.GuardianProfileID = &id
			row.GuardianName = joinName(a.GuardianFirstName, a.GuardianLastName)
			row.AccountHolder = row.GuardianName
			if data := financial[id]; data != nil {
				if data.AccountHolder != nil && *data.AccountHolder != "" {
					row.AccountHolder = *data.AccountHolder
				}
				if data.IBAN != nil && *data.IBAN != "" {
					if unmasked {
						row.IBAN = *data.IBAN
					}
					if m := maskTailPtr(data.IBAN, 4); m != nil {
						row.IBANMasked = *m
					}
				}
			}
		}
		rows = append(rows, row)
	}

	metadata := map[string]interface{}{"row_count": len(rows)}
	for k, v := range extraMetadata {
		metadata[k] = v
	}
	now := time.Now()
	if err := s.DataAccessLog.Create(ctx, &auditModels.DataAccessLog{
		ActorAccountID: actorAccountID,
		ActorRole:      fallbackActorRole(actorRole),
		ResourceType:   resourceType,
		RangeStart:     now,
		RangeEnd:       now,
		AccessedAt:     now,
		Metadata:       metadata,
	}); err != nil {
		return nil, fmt.Errorf("write payment access audit: %w", err)
	}
	return rows, nil
}

// loadPaymentAudited resolves the guardian, loads the bank row (nil when none
// exists) and writes the data-access log entry. Callers must not serve any
// value when the returned error is non-nil.
func (s *GuardianService) loadPaymentAudited(ctx context.Context, guardianProfileID int64, actorAccountID int64, actorRole string, resourceType string) (*users.GuardianFinancialData, error) {
	var result *users.GuardianFinancialData
	err := s.txHandler.RunInTx(ctx, func(txCtx context.Context) error {
		var err error
		result, err = s.loadPaymentAuditedInTx(txCtx, guardianProfileID, actorAccountID, actorRole, resourceType)
		return err
	})
	return result, err
}

func (s *GuardianService) loadPaymentAuditedInTx(ctx context.Context, guardianProfileID int64, actorAccountID int64, actorRole string, resourceType string) (*users.GuardianFinancialData, error) {
	if s.DataAccessLog == nil {
		return nil, fmt.Errorf("data access log repository is not wired; refusing unaudited payment read")
	}
	if actorAccountID <= 0 {
		return nil, fmt.Errorf("actor account id is required for payment reads")
	}

	// Existence (and tenant scope) check before anything is disclosed.
	if _, err := s.GuardianProfileRepo.FindByID(ctx, guardianProfileID); err != nil {
		return nil, err
	}

	data, err := s.GuardianFinancialRepo.FindByGuardianProfileID(ctx, guardianProfileID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	if err := s.DataAccessLog.Create(ctx, &auditModels.DataAccessLog{
		ActorAccountID: actorAccountID,
		ActorRole:      fallbackActorRole(actorRole),
		ResourceType:   resourceType,
		RangeStart:     now,
		RangeEnd:       now,
		AccessedAt:     now,
		Metadata: map[string]interface{}{
			"guardian_profile_id": guardianProfileID,
		},
	}); err != nil {
		return nil, fmt.Errorf("write payment access audit: %w", err)
	}
	return data, nil
}

// recordPayerChange writes one is_payer audit row.
func (s *GuardianService) recordPayerChange(ctx context.Context, guardianProfileID, studentID, changedBy int64, oldValue, newValue string) error {
	student := studentID
	if err := s.GuardianFinancialAudit.Create(ctx, &auditModels.GuardianFinancialChange{
		GuardianProfileID: guardianProfileID,
		StudentID:         &student,
		ChangedBy:         changedBy,
		FieldName:         auditModels.GuardianPaymentFieldIsPayer,
		OldValue:          oldValue,
		NewValue:          newValue,
	}); err != nil {
		return fmt.Errorf("write guardian payer audit: %w", err)
	}
	return nil
}

// normalizeGuardianPaymentInput trims, uppercases and validates the payment
// fields. Empty strings clear a field (nil).
func normalizeGuardianPaymentInput(input GuardianPaymentInput) (GuardianPaymentInput, error) {
	out := GuardianPaymentInput{}

	if v := normalizeCompact(input.IBAN); v != nil {
		iban := strings.ToUpper(*v)
		if !guardianIBANPattern.MatchString(iban) {
			return out, ErrGuardianIBANInvalid
		}
		expectedLength, knownCountry := guardianIBANLengths[iban[:2]]
		if !knownCountry || len(iban) != expectedLength || !ibanChecksumValid(iban) {
			return out, ErrGuardianIBANInvalid
		}
		out.IBAN = &iban
	}
	if v := normalizeOptional(input.AccountHolder); v != nil {
		if len([]rune(*v)) > maxAccountHolderLen {
			return out, ErrGuardianAccountHolderTooLong
		}
		out.AccountHolder = v
	}
	return out, nil
}

func samePayer(a, b *int64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func joinName(first, last string) string {
	return strings.TrimSpace(strings.TrimSpace(first) + " " + strings.TrimSpace(last))
}

func fallbackActorRole(role string) string {
	if strings.TrimSpace(role) == "" {
		return "unknown"
	}
	return role
}
