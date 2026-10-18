package application

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

// Guardian payment data (#2608). Two hard rules, carried over from the staff
// bank & tax path (#1423): no change without an audit row in the same tenant
// transaction, and no read of bank data without a data-access log row — the
// service refuses to serve an IBAN when the audit write fails.
//
// The IBAN belongs to the guardian; which of a child's guardians is charged
// belongs to the relationship (StudentGuardian.IsPayer). Siblings therefore
// share one maintained IBAN and carry the mark once per child.

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

// GetGuardianPaymentMasked serves the masked bank details and logs the read.
// No data leaves without a successful audit row.
func (s *RetainedGuardianService) GetGuardianPaymentMasked(ctx context.Context, guardianProfileID int64, actorAccountID int64, actorRole string) (*peoplePorts.GuardianPaymentMasked, error) {
	data, err := s.loadPaymentAudited(ctx, guardianProfileID, actorAccountID, actorRole, peoplePorts.GuardianPaymentAccessView)
	if err != nil {
		return nil, err
	}
	masked := &peoplePorts.GuardianPaymentMasked{GuardianProfileID: guardianProfileID}
	if data != nil {
		masked.IBANMasked = maskTailPtr(data.IBAN, 4)
		masked.AccountHolder = data.AccountHolder
	}
	return masked, nil
}

// RevealGuardianPayment serves the full IBAN after the explicit "Anzeigen"
// toggle and logs the reveal.
func (s *RetainedGuardianService) RevealGuardianPayment(ctx context.Context, guardianProfileID int64, actorAccountID int64, actorRole string) (*peoplePorts.GuardianPaymentPlain, error) {
	data, err := s.loadPaymentAudited(ctx, guardianProfileID, actorAccountID, actorRole, peoplePorts.GuardianPaymentAccessReveal)
	if err != nil {
		return nil, err
	}
	plain := &peoplePorts.GuardianPaymentPlain{GuardianProfileID: guardianProfileID}
	if data != nil {
		plain.IBAN = data.IBAN
		plain.AccountHolder = data.AccountHolder
	}
	return plain, nil
}

// UpdateGuardianPayment replaces the bank section of one guardian. Audit rows
// carry masked values only and record the authenticated account as actor.
func (s *RetainedGuardianService) UpdateGuardianPayment(ctx context.Context, guardianProfileID int64, input peoplePorts.GuardianPaymentInput, changedByAccountID int64, note string) error {
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

	return s.transactions.WithTenantTx(ctx, func(ctx context.Context) error {
		return s.updateGuardianPaymentInTx(ctx, guardianProfileID, normalized, changedByAccountID, note)
	})
}

// updateGuardianPaymentInTx writes the audit rows and the bank row of one
// payment change inside the tenant transaction.
func (s *RetainedGuardianService) updateGuardianPaymentInTx(ctx context.Context, guardianProfileID int64, normalized peoplePorts.GuardianPaymentInput, changedByAccountID int64, note string) error {
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

	var current peopleValues.GuardianPayment
	if data != nil {
		current = *data
	}

	changes := guardianPaymentChanges(current, normalized)
	if len(changes) == 0 {
		return nil
	}

	trimmedNote := strings.TrimSpace(note)
	for _, change := range changes {
		if err := s.GuardianFinancialAudit.RecordGuardianFinancialChange(ctx, peoplePorts.GuardianFinancialChange{
			GuardianProfileID: guardianProfileID,
			ChangedBy:         changedByAccountID,
			Field:             change.field,
			OldValue:          change.oldValue,
			NewValue:          change.newValue,
			Note:              trimmedNote,
		}); err != nil {
			return fmt.Errorf("write guardian payment audit: %w", err)
		}
	}

	if data == nil {
		fresh := &peopleValues.GuardianPayment{GuardianProfileID: guardianProfileID}
		fresh.IBAN = normalized.IBAN
		fresh.AccountHolder = normalized.AccountHolder
		return s.GuardianFinancialRepo.Create(ctx, fresh)
	}
	data.IBAN = normalized.IBAN
	data.AccountHolder = normalized.AccountHolder
	return s.GuardianFinancialRepo.Update(ctx, data)
}

// guardianPaymentChanges diffs the stored bank section against the new one.
// The no-op decision compares plaintext (a masked comparison would miss a
// change with identical last-4), but the audit rows carry masked values only —
// the trail must not become a second store of bank data outside the
// guardians:financial gate.
func guardianPaymentChanges(current peopleValues.GuardianPayment, normalized peoplePorts.GuardianPaymentInput) []stammdatenChange {
	var changes []stammdatenChange
	if strPtrAuditValue(current.IBAN) != strPtrAuditValue(normalized.IBAN) {
		changes = append(changes, maskedChange(peoplePorts.GuardianPaymentFieldIBAN,
			maskTailPtr(current.IBAN, 4), maskTailPtr(normalized.IBAN, 4)))
	}
	if strPtrAuditValue(current.AccountHolder) != strPtrAuditValue(normalized.AccountHolder) {
		changes = append(changes, maskedChange(peoplePorts.GuardianPaymentFieldAccountHolder,
			maskAllPtr(current.AccountHolder), maskAllPtr(normalized.AccountHolder)))
	}
	return changes
}

// SetStudentPayer marks which guardian's account is charged for one child, or
// clears the assignment when guardianProfileID is nil. The clear and the set
// share one tenant transaction with the audit row.
func (s *RetainedGuardianService) SetStudentPayer(ctx context.Context, studentID int64, guardianProfileID *int64, changedByAccountID int64) error {
	if studentID <= 0 {
		return peoplePorts.ErrGuardianStudentRequired
	}
	if s.GuardianFinancialAudit == nil {
		return fmt.Errorf("guardian financial audit repository is not wired; refusing unaudited change")
	}
	if changedByAccountID <= 0 {
		return fmt.Errorf("changed-by actor id is required")
	}

	return s.transactions.WithTenantTx(ctx, func(ctx context.Context) error {
		return s.setStudentPayerInTx(ctx, studentID, guardianProfileID, changedByAccountID)
	})
}

// setStudentPayerInTx moves the payer mark of one child inside the tenant
// transaction, with one audit row per side of the move.
func (s *RetainedGuardianService) setStudentPayerInTx(ctx context.Context, studentID int64, guardianProfileID *int64, changedByAccountID int64) error {
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

	currentPayer, linked := studentPayerState(relationships, guardianProfileID)
	if guardianProfileID != nil && !linked {
		return peoplePorts.ErrGuardianNotLinkedToStudent
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
}

// studentPayerState reports the child's current payer and whether the
// requested guardian is linked to the child at all.
func studentPayerState(relationships []*peopleValues.GuardianRelationshipSnapshot, guardianProfileID *int64) (*int64, bool) {
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
	return currentPayer, linked
}

// ListPaymentOverview serves the school-wide Bankverbindungen list with MASKED
// IBANs and logs the read.
func (s *RetainedGuardianService) ListPaymentOverview(ctx context.Context, actorAccountID int64, actorRole string) ([]peoplePorts.GuardianPaymentRow, error) {
	return s.buildPaymentRows(ctx, actorAccountID, actorRole, false, peoplePorts.GuardianPaymentAccessOverview, nil)
}

// ListPaymentExportRows serves the same list with UNMASKED IBANs for the file
// export and logs it as its own resource type — a bulk export of bank data is
// a materially different event from viewing the masked list.
func (s *RetainedGuardianService) ListPaymentExportRows(ctx context.Context, actorAccountID int64, actorRole string, format string) ([]peoplePorts.GuardianPaymentRow, error) {
	return s.buildPaymentRows(ctx, actorAccountID, actorRole, true, peoplePorts.GuardianPaymentAccessExport, map[string]interface{}{
		"format": format,
	})
}

// --- internals -------------------------------------------------------------

// buildPaymentRows assembles the Bankverbindungen rows and writes the
// data-access log entry. Nothing is returned when the audit write fails.
func (s *RetainedGuardianService) buildPaymentRows(
	ctx context.Context,
	actorAccountID int64,
	actorRole string,
	unmasked bool,
	access peoplePorts.GuardianPaymentAccess,
	extraMetadata map[string]interface{},
) ([]peoplePorts.GuardianPaymentRow, error) {
	var result []peoplePorts.GuardianPaymentRow
	err := s.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		var err error
		result, err = s.buildPaymentRowsInTx(txCtx, actorAccountID, actorRole, unmasked, access, extraMetadata)
		return err
	})
	return result, err
}

func (s *RetainedGuardianService) buildPaymentRowsInTx(
	ctx context.Context,
	actorAccountID int64,
	actorRole string,
	unmasked bool,
	access peoplePorts.GuardianPaymentAccess,
	extraMetadata map[string]interface{},
) ([]peoplePorts.GuardianPaymentRow, error) {
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

	financial, err := s.GuardianFinancialRepo.ListByGuardianProfileIDs(ctx, payerGuardianIDs(assignments))
	if err != nil {
		return nil, err
	}

	rows := make([]peoplePorts.GuardianPaymentRow, 0, len(assignments))
	for _, a := range assignments {
		rows = append(rows, guardianPaymentRow(a, financial, unmasked))
	}

	metadata := map[string]interface{}{"row_count": len(rows)}
	for k, v := range extraMetadata {
		metadata[k] = v
	}
	if err := s.DataAccessLog.RecordGuardianDataAccess(ctx, peoplePorts.GuardianDataAccess{
		ActorAccountID: actorAccountID,
		ActorRole:      fallbackActorRole(actorRole),
		Access:         access,
		AccessedAt:     time.Now(),
		Metadata:       metadata,
	}); err != nil {
		return nil, fmt.Errorf("write payment access audit: %w", err)
	}
	return rows, nil
}

// payerGuardianIDs lists each assigned payer once, in assignment order.
func payerGuardianIDs(assignments []peopleValues.GuardianPaymentAssignment) []int64 {
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
	return guardianIDs
}

// guardianPaymentRow builds one Bankverbindungen row. The full IBAN is set
// only when unmasked; the masked one always.
func guardianPaymentRow(a peopleValues.GuardianPaymentAssignment, financial map[int64]*peopleValues.GuardianPayment, unmasked bool) peoplePorts.GuardianPaymentRow {
	row := peoplePorts.GuardianPaymentRow{
		StudentID:        a.StudentID,
		StudentName:      joinName(a.StudentFirstName, a.StudentLastName),
		SchoolClass:      a.SchoolClass,
		RelationshipType: a.RelationshipTypeRaw,
	}
	if a.GuardianProfileID == nil {
		return row
	}
	id := *a.GuardianProfileID
	row.GuardianProfileID = &id
	row.GuardianName = joinName(a.GuardianFirstName, a.GuardianLastName)
	row.AccountHolder = row.GuardianName
	data := financial[id]
	if data == nil {
		return row
	}
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
	return row
}

// loadPaymentAudited resolves the guardian, loads the bank row (nil when none
// exists) and writes the data-access log entry. Callers must not serve any
// value when the returned error is non-nil.
func (s *RetainedGuardianService) loadPaymentAudited(ctx context.Context, guardianProfileID int64, actorAccountID int64, actorRole string, access peoplePorts.GuardianPaymentAccess) (*peopleValues.GuardianPayment, error) {
	var result *peopleValues.GuardianPayment
	err := s.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		var err error
		result, err = s.loadPaymentAuditedInTx(txCtx, guardianProfileID, actorAccountID, actorRole, access)
		return err
	})
	return result, err
}

func (s *RetainedGuardianService) loadPaymentAuditedInTx(ctx context.Context, guardianProfileID int64, actorAccountID int64, actorRole string, access peoplePorts.GuardianPaymentAccess) (*peopleValues.GuardianPayment, error) {
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

	if err := s.DataAccessLog.RecordGuardianDataAccess(ctx, peoplePorts.GuardianDataAccess{
		ActorAccountID: actorAccountID,
		ActorRole:      fallbackActorRole(actorRole),
		Access:         access,
		AccessedAt:     time.Now(),
		Metadata: map[string]interface{}{
			"guardian_profile_id": guardianProfileID,
		},
	}); err != nil {
		return nil, fmt.Errorf("write payment access audit: %w", err)
	}
	return data, nil
}

// recordPayerChange writes one is_payer audit row.
func (s *RetainedGuardianService) recordPayerChange(ctx context.Context, guardianProfileID, studentID, changedBy int64, oldValue, newValue string) error {
	student := studentID
	if err := s.GuardianFinancialAudit.RecordGuardianFinancialChange(ctx, peoplePorts.GuardianFinancialChange{
		GuardianProfileID: guardianProfileID,
		StudentID:         &student,
		ChangedBy:         changedBy,
		Field:             peoplePorts.GuardianPaymentFieldIsPayer,
		OldValue:          oldValue,
		NewValue:          newValue,
	}); err != nil {
		return fmt.Errorf("write guardian payer audit: %w", err)
	}
	return nil
}

// normalizeGuardianPaymentInput trims, uppercases and validates the payment
// fields. Empty strings clear a field (nil).
func normalizeGuardianPaymentInput(input peoplePorts.GuardianPaymentInput) (peoplePorts.GuardianPaymentInput, error) {
	out := peoplePorts.GuardianPaymentInput{}

	if v := normalizeCompact(input.IBAN); v != nil {
		iban := strings.ToUpper(*v)
		if !guardianIBANPattern.MatchString(iban) {
			return out, peoplePorts.ErrGuardianIBANInvalid
		}
		expectedLength, knownCountry := guardianIBANLengths[iban[:2]]
		if !knownCountry || len(iban) != expectedLength || !ibanChecksumValid(iban) {
			return out, peoplePorts.ErrGuardianIBANInvalid
		}
		out.IBAN = &iban
	}
	if v := normalizeOptional(input.AccountHolder); v != nil {
		if len([]rune(*v)) > maxAccountHolderLen {
			return out, peoplePorts.ErrGuardianAccountHolderTooLong
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
