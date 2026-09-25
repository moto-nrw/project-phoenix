package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// payerFieldTrue and payerFieldFalse are the ledger values of the payer flag.
const (
	payerFieldTrue  = "true"
	payerFieldFalse = "false"
)

// linkAdditionalGuardians materializes the co-guardians stored on the
// request (enrollment.request_guardians) as additional student-guardian
// links for the student. Co-guardians are mapped identically to the primary
// guardian except IsPrimary=false; they are contact-only, so there is
// deliberately NO account attach and NO invitation here.
//
// Each co-guardian resolves to a guardian profile exactly once per request:
// the first child approval stamps guardian_profile_id back on the
// request_guardians row and later child approvals reuse it. Children are
// approved one at a time, and an email-less co-guardian cannot be deduped by
// email — the stamp is what prevents duplicate profiles.
func (d *Decisions) linkAdditionalGuardians(
	ctx context.Context,
	request *enrollmentModels.Request,
	studentID int64,
) error {
	if d.deps.Guardians == nil {
		return nil
	}
	extras, err := d.deps.Guardians.RequestGuardians(ctx, []int64{request.ID})
	if err != nil {
		return fmt.Errorf("list additional guardians: %w", err)
	}
	for _, extra := range extras {
		if err := d.linkAdditionalGuardian(ctx, extra, studentID); err != nil {
			return err
		}
	}
	return nil
}

func (d *Decisions) linkAdditionalGuardian(ctx context.Context, extra *enrollment.RequestGuardian, studentID int64) error {
	profileID, err := d.resolveAdditionalGuardianProfile(ctx, extra)
	if err != nil {
		return err
	}
	rel := &StudentGuardian{
		StudentID:          studentID,
		GuardianProfileID:  profileID,
		RelationshipType:   "guardian",
		IsPrimary:          false,
		IsEmergencyContact: true,
		CanPickup:          true,
	}
	d.deps.People.Rules.ApplyGuardianRole(rel, GuardianRoleEmergency)
	if err := d.deps.People.Rules.ValidateStudentGuardian(rel); err != nil {
		return fmt.Errorf("validate co-guardian student_guardian: %w", err)
	}
	if err := d.upsertContactStudentGuardianLink(ctx, rel); err != nil {
		return fmt.Errorf("link co-guardian student_guardian: %w", err)
	}
	// Persist the co-guardian's phone number, mirroring the primary guardian.
	// A co-guardian can be a phone-only contact (no email), so this is the
	// ONLY reachable detail downstream contact views have. Non-fatal: a phone
	// write failure must not roll back an otherwise-valid approval.
	if extra.Phone != nil {
		if err := d.createGuardianPhoneNumber(ctx, profileID, *extra.Phone); err != nil {
			d.logger().Warn("decision: persist co-guardian phone failed",
				slog.Int64("guardian_profile_id", profileID),
				slog.String("error", err.Error()))
		}
	}
	return nil
}

// reconcilePrimaryGuardianLink resolves the request's primary guardian profile
// and makes sure the student carries it as the primary student-guardian
// relationship, creating the link when the student has none (imported /
// manually created children, and children whose original enrollment predates
// the guardian link).
//
// pruneStalePrimary decides what happens to a DIFFERENT guardian who currently
// holds the primary link:
//
//   - true (admin edit sync): the edit rewrites this request's guardian, so the
//     link it produced earlier is repointed / removed — the old profile was the
//     same submission's answer and is now wrong.
//   - false (existing-student re-enrollment): the matched student may carry a
//     primary guardian from an ENTIRELY different source — last year's approval,
//     an import, the other parent. Deleting or repointing that row would strip a
//     real guardian of their pickup authority and parent-portal access because
//     the other parent happened to submit this year's renewal. The submitted
//     guardian still becomes primary (the DB trigger
//     enforce_single_primary_student_guardian demotes the previous holder to
//     is_primary=false), but their link and permissions survive (#1663).
func (d *Decisions) reconcilePrimaryGuardianLink(
	ctx context.Context,
	request *enrollmentModels.Request,
	studentID int64,
	pruneStalePrimary bool,
	reviewedBy int64,
) (*GuardianProfile, error) {
	guardian, _, err := d.resolveGuardianProfile(ctx, request)
	if err != nil {
		return nil, err
	}
	links := d.deps.People.StudentGuardians
	if links == nil {
		return guardian, nil
	}
	current, err := links.GuardianLinksOfStudent(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("decision: list approved child guardians: %w", err)
	}
	var primaryLink, currentLink *StudentGuardian
	for _, link := range current {
		if link == nil {
			continue
		}
		if link.IsPrimary {
			primaryLink = link
		}
		if link.GuardianProfileID == guardian.ID {
			currentLink = link
		}
	}
	switch {
	case currentLink != nil:
		err = d.promoteCurrentGuardianLink(ctx, primaryLink, currentLink, pruneStalePrimary, reviewedBy)
	case pruneStalePrimary && primaryLink != nil:
		err = d.repointPrimaryGuardianLink(ctx, primaryLink, guardian.ID, reviewedBy)
	default:
		err = d.createMissingPrimaryGuardianLink(ctx, studentID, guardian.ID)
	}
	if err != nil {
		return nil, err
	}
	return guardian, nil
}

// setPrimaryGuardianLink marks a link as the primary guardian relationship
// with the primary guardian role.
func (d *Decisions) setPrimaryGuardianLink(link *StudentGuardian) {
	link.RelationshipType = "guardian"
	link.IsPrimary = true
	link.IsEmergencyContact = true
	link.CanPickup = true
	d.deps.People.Rules.ApplyGuardianRole(link, GuardianRolePrimary)
}

// promoteCurrentGuardianLink makes the guardian's existing link the primary
// one; when pruning, the stale primary link hands over the payer flag and is
// removed.
func (d *Decisions) promoteCurrentGuardianLink(ctx context.Context, primaryLink, currentLink *StudentGuardian, prune bool, reviewedBy int64) error {
	links := d.deps.People.StudentGuardians
	stale := prune && primaryLink != nil && primaryLink.ID != currentLink.ID
	payerTransferred := false
	if stale && primaryLink.IsPayer {
		if err := d.recordPayerTransfer(ctx, primaryLink, currentLink.GuardianProfileID, reviewedBy); err != nil {
			return err
		}
		primaryLink.IsPayer = false
		if err := links.UpdateGuardianLink(ctx, primaryLink); err != nil {
			return fmt.Errorf("decision: clear stale payer link: %w", err)
		}
		currentLink.IsPayer = true
		payerTransferred = true
	}
	d.setPrimaryGuardianLink(currentLink)
	if err := d.deps.People.Rules.ValidateStudentGuardian(currentLink); err != nil {
		return fmt.Errorf("decision: validate current primary guardian link: %w", err)
	}
	if err := links.UpdateGuardianLink(ctx, currentLink); err != nil {
		return fmt.Errorf("decision: update current primary guardian link: %w", err)
	}
	if !stale {
		return nil
	}
	if !payerTransferred {
		if err := d.recordPayerRemoval(ctx, primaryLink, reviewedBy); err != nil {
			return err
		}
	}
	if err := links.DeleteGuardianLink(ctx, primaryLink.ID); err != nil {
		return fmt.Errorf("decision: remove stale primary guardian link: %w", err)
	}
	return nil
}

// repointPrimaryGuardianLink points the stale primary link at the submitted
// guardian.
func (d *Decisions) repointPrimaryGuardianLink(ctx context.Context, primaryLink *StudentGuardian, guardianProfileID, reviewedBy int64) error {
	if err := d.recordPayerTransfer(ctx, primaryLink, guardianProfileID, reviewedBy); err != nil {
		return err
	}
	primaryLink.GuardianProfileID = guardianProfileID
	d.setPrimaryGuardianLink(primaryLink)
	if err := d.deps.People.Rules.ValidateStudentGuardian(primaryLink); err != nil {
		return fmt.Errorf("decision: validate primary guardian link: %w", err)
	}
	if err := d.deps.People.StudentGuardians.UpdateGuardianLink(ctx, primaryLink); err != nil {
		return fmt.Errorf("decision: update primary guardian link: %w", err)
	}
	return nil
}

// createMissingPrimaryGuardianLink links the submitted guardian as primary to
// a student that carries no link for them.
func (d *Decisions) createMissingPrimaryGuardianLink(ctx context.Context, studentID, guardianProfileID int64) error {
	rel := &StudentGuardian{StudentID: studentID, GuardianProfileID: guardianProfileID}
	d.setPrimaryGuardianLink(rel)
	if err := d.deps.People.Rules.ValidateStudentGuardian(rel); err != nil {
		return fmt.Errorf("decision: validate missing primary guardian link: %w", err)
	}
	if err := d.deps.People.StudentGuardians.CreateGuardianLink(ctx, rel); err != nil {
		return fmt.Errorf("decision: create missing primary guardian link: %w", err)
	}
	return nil
}

// reconcileApprovedChildGuardians relinks the request's current co-guardians
// and unlinks those an edit removed, returning the profiles it keeps.
func (d *Decisions) reconcileApprovedChildGuardians(
	ctx context.Context,
	request *enrollmentModels.Request,
	studentID int64,
	previousGuardians []*enrollment.RequestGuardian,
	reviewedBy int64,
) (map[int64]bool, error) {
	currentProfileIDs := map[int64]bool{}
	if d.deps.Guardians == nil || d.deps.People.StudentGuardians == nil {
		return currentProfileIDs, nil
	}
	if err := d.linkAdditionalGuardians(ctx, request, studentID); err != nil {
		return currentProfileIDs, fmt.Errorf("decision: relink additional guardians: %w", err)
	}
	current, err := d.deps.Guardians.RequestGuardians(ctx, []int64{request.ID})
	if err != nil {
		return currentProfileIDs, fmt.Errorf("decision: list current additional guardians: %w", err)
	}
	if err := d.collectCoGuardianProfileIDs(ctx, current, currentProfileIDs); err != nil {
		return currentProfileIDs, err
	}
	if err := d.deleteRemovedStudentGuardianLinks(ctx, studentID, stampedGuardianProfileIDs(previousGuardians), currentProfileIDs, reviewedBy); err != nil {
		return currentProfileIDs, fmt.Errorf("decision: unlink removed additional guardians: %w", err)
	}
	return currentProfileIDs, nil
}

// collectCoGuardianProfileIDs resolves the profile of every current
// co-guardian into ids, stopping at the first failure.
func (d *Decisions) collectCoGuardianProfileIDs(ctx context.Context, rows []*enrollment.RequestGuardian, ids map[int64]bool) error {
	for _, row := range rows {
		if row == nil {
			continue
		}
		profileID, err := d.resolveAdditionalGuardianProfile(ctx, row)
		if err != nil {
			return err
		}
		if profileID > 0 {
			ids[profileID] = true
		}
	}
	return nil
}

// stampedGuardianProfileIDs returns the profiles the co-guardian rows were
// already resolved to.
func stampedGuardianProfileIDs(rows []*enrollment.RequestGuardian) map[int64]bool {
	ids := map[int64]bool{}
	for _, row := range rows {
		if row != nil && row.GuardianProfileID != nil && *row.GuardianProfileID > 0 {
			ids[*row.GuardianProfileID] = true
		}
	}
	return ids
}

func (d *Decisions) recordPayerRemoval(ctx context.Context, link *StudentGuardian, reviewedBy int64) error {
	if link == nil || !link.IsPayer {
		return nil
	}
	if d.deps.Payers == nil || reviewedBy <= 0 {
		return fmt.Errorf("decision: payer removal requires a financial audit actor")
	}
	if err := d.deps.Payers.RecordPayerChange(ctx, PayerChange{
		GuardianProfileID: link.GuardianProfileID, StudentID: link.StudentID, ChangedBy: reviewedBy,
		OldValue: payerFieldTrue, NewValue: payerFieldFalse,
		Note: "Erziehungsberechtigte Person vom Kind entfernt",
	}); err != nil {
		return fmt.Errorf("decision: write payer removal audit: %w", err)
	}
	return nil
}

// recordPayerTransfer records both sides before a relationship row is pointed
// at another guardian. Keeping is_payer on the row would otherwise silently
// turn the new guardian into the payer.
func (d *Decisions) recordPayerTransfer(ctx context.Context, link *StudentGuardian, newGuardianProfileID, reviewedBy int64) error {
	if link == nil || !link.IsPayer || link.GuardianProfileID == newGuardianProfileID {
		return nil
	}
	if d.deps.Payers == nil || reviewedBy <= 0 {
		return fmt.Errorf("decision: payer transfer requires a financial audit actor")
	}
	for _, change := range []PayerChange{
		{GuardianProfileID: link.GuardianProfileID, StudentID: link.StudentID, ChangedBy: reviewedBy, OldValue: payerFieldTrue, NewValue: payerFieldFalse, Note: "Zahlungskonto auf andere erziehungsberechtigte Person übertragen"},
		{GuardianProfileID: newGuardianProfileID, StudentID: link.StudentID, ChangedBy: reviewedBy, OldValue: payerFieldFalse, NewValue: payerFieldTrue, Note: "Zahlungskonto von anderer erziehungsberechtigter Person übernommen"},
	} {
		if err := d.deps.Payers.RecordPayerChange(ctx, change); err != nil {
			return fmt.Errorf("decision: write payer transfer audit: %w", err)
		}
	}
	return nil
}

// deleteRemovedStudentGuardianLinks removes the contact links of profiles an
// edit dropped. Primary links and full guardians are never removed here.
func (d *Decisions) deleteRemovedStudentGuardianLinks(ctx context.Context, studentID int64, previous, keep map[int64]bool, reviewedBy int64) error {
	links := d.deps.People.StudentGuardians
	if len(previous) == 0 || links == nil {
		return nil
	}
	current, err := links.GuardianLinksOfStudent(ctx, studentID)
	if err != nil {
		return err
	}
	for _, link := range current {
		if link == nil || link.IsPrimary || d.deps.People.Rules.IsFullGuardianRole(link.GuardianRole) {
			continue
		}
		if !previous[link.GuardianProfileID] || keep[link.GuardianProfileID] {
			continue
		}
		if err := d.recordPayerRemoval(ctx, link, reviewedBy); err != nil {
			return err
		}
		if err := links.DeleteGuardianLink(ctx, link.ID); err != nil {
			return err
		}
	}
	return nil
}

func mergeGuardianProfileKeepSets(sets ...map[int64]bool) map[int64]bool {
	out := map[int64]bool{}
	for _, set := range sets {
		for id := range set {
			if id > 0 {
				out[id] = true
			}
		}
	}
	return out
}

// contactProfileIDsFromPreviousSnapshot resolves the profiles the child's
// contact list named before an edit.
func (d *Decisions) contactProfileIDsFromPreviousSnapshot(
	ctx context.Context,
	snapshot map[string]any,
	child *RequestChild,
	studentID int64,
	fieldKey string,
) (map[int64]bool, error) {
	out := map[int64]bool{}
	if snapshot == nil || child == nil || d.deps.People.GuardianProfiles == nil {
		return out, nil
	}
	entries, ok := snapshotContactEntries(snapshot, child.ID, fieldKey)
	if !ok {
		return out, nil
	}
	emails := make([]string, 0, len(entries))
	for _, entry := range entries {
		emails = append(emails, strings.TrimSpace(strings.ToLower(entry.Email)))
	}
	profilesByEmail, err := d.guardianProfilesByEmails(ctx, emails)
	if err != nil {
		return out, err
	}
	phoneOnlyProfiles, err := d.loadPhoneOnlyContactProfiles(ctx, studentID)
	if err != nil {
		return out, err
	}
	for _, entry := range entries {
		for _, profile := range matchSnapshotContact(entry, profilesByEmail, phoneOnlyProfiles) {
			if profile != nil && profile.ID > 0 {
				out[profile.ID] = true
			}
		}
	}
	return out, nil
}

// matchSnapshotContact finds the profiles one stored contact entry named: by
// email, or by phone number for an entry without one.
func matchSnapshotContact(entry enrollment.ContactEntry, profilesByEmail map[string]*GuardianProfile, phoneOnly *phoneOnlyContactProfiles) []*GuardianProfile {
	email := strings.TrimSpace(strings.ToLower(entry.Email))
	if email == "" {
		return phoneOnly.match(entry)
	}
	return []*GuardianProfile{profilesByEmail[email]}
}

// snapshotContactEntries reads the contact list of one child from a stored
// request snapshot; ok is false when the snapshot has no readable list.
func snapshotContactEntries(snapshot map[string]any, childID int64, fieldKey string) ([]enrollment.ContactEntry, bool) {
	childRow := snapshotChildByID(snapshot, childID)
	if childRow == nil {
		return nil, false
	}
	raw := mapFromAny(childRow["custom_data"])[fieldKey]
	if raw == nil {
		return nil, false
	}
	var entries []enrollment.ContactEntry
	if err := decodeStructured(raw, &entries); err != nil {
		return nil, false
	}
	return entries, true
}

// upsertContactStudentGuardianLink links a contact to the student or updates
// the contact's existing link. A primary link or a full guardian keeps its
// relationship untouched.
func (d *Decisions) upsertContactStudentGuardianLink(ctx context.Context, rel *StudentGuardian) error {
	if rel == nil {
		return errors.New("contact student guardian link cannot be nil")
	}
	if err := d.deps.People.Rules.ValidateStudentGuardian(rel); err != nil {
		return err
	}
	links := d.deps.People.StudentGuardians
	existing, err := d.contactLinkForUpdate(ctx, rel)
	if err != nil || existing == nil {
		return err
	}
	if existing.IsPrimary || d.deps.People.Rules.IsFullGuardianRole(existing.GuardianRole) {
		return nil
	}
	existing.RelationshipType = rel.RelationshipType
	existing.IsEmergencyContact = rel.IsEmergencyContact
	existing.CanPickup = rel.CanPickup
	existing.EmergencyPriority = rel.EmergencyPriority
	existing.GuardianRole = rel.GuardianRole
	existing.Permissions = rel.Permissions
	updated, err := links.UpdateGuardianLinkRole(ctx, existing)
	if err != nil {
		return err
	}
	if updated == 0 {
		return d.deps.People.ErrGuardianLinkNotFound
	}
	return nil
}

// contactLinkForUpdate locks the contact's existing link, inserting it when
// it is missing. It answers nil when the insert created the link.
func (d *Decisions) contactLinkForUpdate(ctx context.Context, rel *StudentGuardian) (*StudentGuardian, error) {
	links := d.deps.People.StudentGuardians
	existing, err := links.GuardianLinkForUpdate(ctx, rel.StudentID, rel.GuardianProfileID)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, d.deps.People.ErrGuardianLinkNotFound) {
		return nil, err
	}
	inserted, err := links.LinkGuardianIfMissing(ctx, rel)
	if err != nil || inserted {
		return nil, err
	}
	return links.GuardianLinkForUpdate(ctx, rel.StudentID, rel.GuardianProfileID)
}
