package messaging

import (
	"context"
	"fmt"
	"time"
)

// Consents to an older wording (#6303). A submission is bound to one version
// of the declaration (its id and the content hash of its wording). When the
// school changes the text, the child state counts only the new version, but
// the guardian's consent to the old wording was given and stays revocable
// (Art. 7 Abs. 3 DSGVO). The feed shows that wording, the revocation is
// recorded against it, and one revocation ends every own consent to the
// declaration for the child (revokeOwnConsents).

// standingOwnConsents maps version id to the account's latest submission on
// that version, for the versions where that latest action is a consent.
func standingOwnConsents(history []*DeclarationSubmission, announcementID, studentID, accountID int64) map[int64]*DeclarationSubmission {
	latest := map[int64]*DeclarationSubmission{}
	for _, sub := range history { // newest first
		if sub.AnnouncementID != announcementID || sub.StudentID != studentID ||
			sub.AccountID == nil || *sub.AccountID != accountID {
			continue
		}
		if _, ok := latest[sub.VersionID]; !ok {
			latest[sub.VersionID] = sub
		}
	}
	for versionID, sub := range latest {
		if sub.Action != DeclarationActionAgreed {
			delete(latest, versionID)
		}
	}
	return latest
}

// loadEarlierVersions loads all versions of the declarations where the
// account still has a consent to an older wording for one of its children.
// Usually there is none, and no statement runs.
func (s *Service) loadEarlierVersions(ctx context.Context, facts *declarationFeedFacts, accountID int64) error {
	facts.earlier = map[int64]map[int64]*DeclarationVersion{}
	for announcementID, children := range facts.children {
		current := facts.versions[announcementID]
		if current == nil || !hasEarlierConsent(facts.submissions, current, children, accountID) {
			continue
		}
		versions, err := s.AnnouncementRepo.ListDeclarationVersions(ctx, current.TenantID, announcementID)
		if err != nil {
			return fmt.Errorf("parent: load declaration versions: %w", err)
		}
		byID := make(map[int64]*DeclarationVersion, len(versions))
		for _, v := range versions {
			byID[v.ID] = v
		}
		facts.earlier[announcementID] = byID
	}
	return nil
}

func hasEarlierConsent(history []*DeclarationSubmission, current *DeclarationVersion, children []*DeclarationChild, accountID int64) bool {
	for _, c := range children {
		for versionID := range standingOwnConsents(history, current.AnnouncementID, c.StudentID, accountID) {
			if versionID != current.ID {
				return true
			}
		}
	}
	return false
}

// earlierConsent returns the account's newest standing consent to an older
// wording of the declaration for the child, or nil.
func (f *declarationFeedFacts) earlierConsent(policy DeclarationPolicy, rules declarationRules, announcementID, studentID, accountID int64, now time.Time) *DeclarationEarlierConsent {
	versions, current := f.earlier[announcementID], f.versions[announcementID]
	if versions == nil || current == nil {
		return nil
	}
	var best *DeclarationEarlierConsent
	for versionID, sub := range standingOwnConsents(f.submissions, announcementID, studentID, accountID) {
		v := versions[versionID]
		if versionID == current.ID || v == nil || (best != nil && v.VersionNo < best.Version.VersionNo) {
			continue
		}
		// An older wording takes no new answer: only its revocation.
		best = &DeclarationEarlierConsent{
			Version: v, SubmittedAt: sub.SubmittedAt,
			AllowedActions: declarationActionsFor(policy, rules, false, sub.Action, now),
		}
	}
	return best
}

// presentEndedDeclarations shapes a declaration the school no longer shows
// (withdrawn, deactivated, expired, or its school switched parent news off):
// it keeps the children the guardian answered or still has a consent for,
// shows the wording they answered and drops what belonged to the live
// publication (#6290, #6303).
func presentEndedDeclarations(items []*AnnouncementFeedItem) []*AnnouncementFeedItem {
	for _, item := range items {
		if !item.Ended || item.Declaration == nil {
			continue
		}
		d := item.Declaration
		kept := d.Children[:0]
		for _, c := range d.Children {
			if c.MyAction != nil || c.EarlierConsent != nil {
				kept = append(kept, c)
			}
		}
		d.Children = kept
		showEarlierConsentAsTheAnswer(d)
		showEndedDeclarationAsAnswered(item)
	}
	return items
}

// showEarlierConsentAsTheAnswer presents an ended declaration nobody here
// answered in its current wording, but which holds an own consent to an older
// wording, as that wording with that consent: the guardian sees what they
// revoke.
func showEarlierConsentAsTheAnswer(d *AnnouncementFeedDeclaration) {
	var version *DeclarationVersion
	for _, c := range d.Children {
		if c.MyAction != nil {
			return
		}
		if version == nil && c.EarlierConsent != nil {
			version = c.EarlierConsent.Version
		}
	}
	if version == nil {
		return
	}
	d.Version = version
	for _, c := range d.Children {
		ec := c.EarlierConsent
		if ec == nil || ec.Version.ID != version.ID {
			continue
		}
		action, at := DeclarationActionAgreed, ec.SubmittedAt
		c.MyAction, c.MySubmittedAt, c.AllowedActions = &action, &at, ec.AllowedActions
		// The child state and the other guardians belong to the current
		// wording; for this answer the state is the own consent.
		c.State, c.OtherSigners, c.EarlierConsent = DeclarationActionAgreed, []DeclarationSignerState{}, nil
	}
}

// declarationVersionByID returns one version of the declaration, or
// ErrDeclarationVersionChanged when the declaration has no such version.
func (s *Service) declarationVersionByID(ctx context.Context, tenantID, announcementID, versionID int64) (*DeclarationVersion, error) {
	versions, err := s.AnnouncementRepo.ListDeclarationVersions(ctx, tenantID, announcementID)
	if err != nil {
		return nil, fmt.Errorf("parent: load declaration versions: %w", err)
	}
	for _, v := range versions {
		if v.ID == versionID {
			return v, nil
		}
	}
	return nil, ErrDeclarationVersionChanged
}

// revokeOwnConsents records the revocation on the version the guardian
// revoked and on every other version of the declaration where their consent
// for the child still stands. Consents to an older and a newer wording of the
// same declaration are one consent for the guardian: one revocation ends
// them, each with a row bound to its own wording, so every proof stays exact.
// It runs under the child lock and returns the row of the revoked version.
func (s *Service) revokeOwnConsents(ctx context.Context, tenantID, announcementID, accountID int64, target *DeclarationVersion, signer *DeclarationSignerContext, in DeclarationInput, passwordConfirmed bool, now time.Time) (*DeclarationSubmission, bool, error) {
	history, err := s.AnnouncementRepo.ListDeclarationSubmissionsForStudents(ctx, []int64{announcementID}, []int64{in.StudentID})
	if err != nil {
		return nil, false, fmt.Errorf("parent: load declaration history: %w", err)
	}
	standing := standingOwnConsents(history, announcementID, in.StudentID, accountID)
	delete(standing, target.ID)
	versions := []*DeclarationVersion{target}
	if len(standing) > 0 {
		all, err := s.AnnouncementRepo.ListDeclarationVersions(ctx, tenantID, announcementID)
		if err != nil {
			return nil, false, fmt.Errorf("parent: load declaration versions: %w", err)
		}
		for _, v := range all {
			if _, ok := standing[v.ID]; ok {
				versions = append(versions, v)
			}
		}
	}
	var revoked *DeclarationSubmission
	for _, v := range versions {
		submission := s.newDeclarationSubmission(tenantID, announcementID, accountID, v, signer, in, passwordConfirmed, nil, now)
		if err := s.AnnouncementRepo.InsertDeclarationSubmission(ctx, submission); err != nil {
			return nil, false, fmt.Errorf("parent: record declaration: %w", err)
		}
		submission.VersionNo = v.VersionNo
		if revoked == nil {
			revoked = submission
		}
	}
	return revoked, true, nil
}
