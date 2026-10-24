package messaging

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/care"
)

// declarationFeedFacts are the batched reads behind the declaration items of
// one feed page: five statements whatever the page size, none at all when the
// page carries no Erklärung.
type declarationFeedFacts struct {
	settings    map[int64]AnnouncementDeclarationSettings
	versions    map[int64]*DeclarationVersion
	children    map[int64][]*DeclarationChild
	signers     map[int64][]*DeclarationSigner
	submissions []*DeclarationSubmission
}

// attachDeclarationData builds the declaration part of each Erklärung item
// for the reading account: its reached children, whether it may declare for
// each, the child's state and the actions it may take next.
func (s *Service) attachDeclarationData(ctx context.Context, accountID int64, items []*AnnouncementFeedItem) error {
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		if item.DeliveryMode == ParentAnnouncementDeliveryDeclaration {
			ids = append(ids, item.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	facts, err := s.loadDeclarationFeedFacts(ctx, accountID, ids)
	if err != nil {
		return err
	}
	now := time.Now()
	for _, item := range items {
		if item.DeliveryMode == ParentAnnouncementDeliveryDeclaration {
			item.Declaration = buildFeedDeclaration(s.Declarations, accountID, item, facts, now)
		}
	}
	return nil
}

func (s *Service) loadDeclarationFeedFacts(ctx context.Context, accountID int64, ids []int64) (*declarationFeedFacts, error) {
	repo := s.AnnouncementRepo
	facts := &declarationFeedFacts{
		children: map[int64][]*DeclarationChild{}, signers: map[int64][]*DeclarationSigner{},
	}
	var err error
	if facts.settings, err = repo.DeclarationSettings(ctx, ids); err != nil {
		return nil, fmt.Errorf("parent: load declaration settings: %w", err)
	}
	if facts.versions, err = repo.LatestDeclarationVersions(ctx, ids); err != nil {
		return nil, fmt.Errorf("parent: load declaration versions: %w", err)
	}
	children, err := repo.DeclarationChildrenForAccount(ctx, accountID, ids)
	if err != nil {
		return nil, fmt.Errorf("parent: load declaration children: %w", err)
	}
	seen := map[int64]bool{}
	studentIDs := []int64{}
	for _, c := range children {
		facts.children[c.AnnouncementID] = append(facts.children[c.AnnouncementID], c)
		if !seen[c.StudentID] {
			seen[c.StudentID] = true
			studentIDs = append(studentIDs, c.StudentID)
		}
	}
	signers, err := repo.DeclarationSignersForStudents(ctx, studentIDs)
	if err != nil {
		return nil, fmt.Errorf("parent: load declaration signers: %w", err)
	}
	for _, signer := range signers {
		facts.signers[signer.StudentID] = append(facts.signers[signer.StudentID], signer)
	}
	if facts.submissions, err = repo.ListDeclarationSubmissionsForStudents(ctx, ids, studentIDs); err != nil {
		return nil, fmt.Errorf("parent: load declaration history: %w", err)
	}
	return facts, nil
}

// latestOnVersion maps student → account → latest submission on a version.
func (f *declarationFeedFacts) latestOnVersion(announcementID, versionID int64) map[int64]map[int64]*DeclarationSubmission {
	out := map[int64]map[int64]*DeclarationSubmission{}
	for _, sub := range f.submissions { // newest first
		if sub.AnnouncementID != announcementID || sub.VersionID != versionID || sub.AccountID == nil {
			continue
		}
		if out[sub.StudentID] == nil {
			out[sub.StudentID] = map[int64]*DeclarationSubmission{}
		}
		if _, ok := out[sub.StudentID][*sub.AccountID]; !ok {
			out[sub.StudentID][*sub.AccountID] = sub
		}
	}
	return out
}

func buildFeedDeclaration(policy DeclarationPolicy, accountID int64, item *AnnouncementFeedItem, facts *declarationFeedFacts, now time.Time) *AnnouncementFeedDeclaration {
	settings := facts.settings[item.ID]
	rules := feedDeclarationRules(settings, item.ResponseDeadline)
	out := &AnnouncementFeedDeclaration{
		Kind: settings.Kind, Signers: settings.Signers, Revocable: settings.Revocable,
		RequiresPassword: settings.RequiresPassword, RequiresSignature: settings.RequiresSignature, Deadline: item.ResponseDeadline,
		Closed: policy.Closed(rules.Deadline, now), Version: facts.versions[item.ID],
		Children: []*AnnouncementFeedDeclarationChild{},
	}
	latest := map[int64]map[int64]*DeclarationSubmission{}
	if out.Version != nil {
		latest = facts.latestOnVersion(item.ID, out.Version.ID)
	}
	for _, c := range facts.children[item.ID] {
		out.Children = append(out.Children, buildFeedDeclarationChild(policy, accountID, c, rules, facts.signers[c.StudentID], latest[c.StudentID], now))
	}
	return out
}

func buildFeedDeclarationChild(policy DeclarationPolicy, accountID int64, c *DeclarationChild, rules declarationRules, signers []*DeclarationSigner, latest map[int64]*DeclarationSubmission, now time.Time) *AnnouncementFeedDeclarationChild {
	child := &AnnouncementFeedDeclarationChild{
		StudentID: c.StudentID, FirstName: c.FirstName, LastName: c.LastName,
		OtherSigners: []DeclarationSignerState{},
	}
	eligible := make([]int64, 0, len(signers))
	actions := map[int64]string{}
	for _, signer := range signers {
		eligible = append(eligible, signer.AccountID)
		state := DeclarationSignerState{AccountID: signer.AccountID, FirstName: signer.FirstName, LastName: signer.LastName}
		if sub, ok := latest[signer.AccountID]; ok {
			action, at := sub.Action, sub.SubmittedAt
			state.Action, state.SubmittedAt = &action, &at
			actions[signer.AccountID] = action
		}
		if signer.AccountID == accountID {
			child.CanSubmit = true
			child.MyAction, child.MySubmittedAt = state.Action, state.SubmittedAt
			continue
		}
		child.OtherSigners = append(child.OtherSigners, state)
	}
	child.State = policy.ChildState(rules.Signers, rules.Deadline, eligible, actions, now)
	child.AllowedActions = policy.AllowedActions(rules.Revocable, rules.Deadline, child.CanSubmit, actions[accountID], now)
	return child
}

// DeclarationProof is what the guardian's proof document shows: the versions
// they declared on and their own declarations for the child.
type DeclarationProof struct {
	Title       string
	SchoolName  string
	ChildName   string
	Kind        string
	Versions    map[int64]*DeclarationVersion
	Submissions []*DeclarationSubmission
	// Signatures are the signatures of the signed answers, by id (#4785).
	Signatures map[int64]*DeclarationSignature
	// IntegrityOK is false when a shown version or answer no longer matches
	// its stored checksum.
	IntegrityOK bool
}

// DeclarationProof returns the account's own proof for one child. It needs a
// current guardian link with portal access and at least one own declaration;
// it stays available after the Erklärung expired, because a proof that
// vanishes with the notice is no proof.
func (s *Service) DeclarationProof(ctx context.Context, accountID, announcementID, studentID int64) (*DeclarationProof, error) {
	if accountID <= 0 || announcementID <= 0 || studentID <= 0 {
		return nil, fmt.Errorf("parent: account_id, announcement_id and student_id must be positive")
	}
	child, err := s.resolvePermittedChild(ctx, accountID, studentID, care.GuardianPermissionPortalAccess)
	if err != nil {
		return nil, ErrAnnouncementNotFound
	}
	proof := &DeclarationProof{SchoolName: child.SchoolName, ChildName: child.StudentName, Versions: map[int64]*DeclarationVersion{}}
	err = s.Runtime.WithinAdmin(ctx, func(adminCtx context.Context) error {
		return s.loadDeclarationProof(adminCtx, proof, accountID, announcementID, studentID, child.TenantID)
	})
	if err != nil {
		return nil, err
	}
	return proof, nil
}

func (s *Service) loadDeclarationProof(ctx context.Context, proof *DeclarationProof, accountID, announcementID, studentID, tenantID int64) error {
	a, err := s.AnnouncementRepo.FindByID(ctx, announcementID)
	if err != nil {
		return fmt.Errorf("parent: load declaration: %w", err)
	}
	if a == nil || !a.IsDeclaration() || a.GetTenantID() != tenantID {
		return ErrAnnouncementNotFound
	}
	proof.Title, proof.Kind = a.Title, a.Declaration.Kind
	history, err := s.AnnouncementRepo.ListDeclarationSubmissionsForStudents(ctx, []int64{announcementID}, []int64{studentID})
	if err != nil {
		return fmt.Errorf("parent: load declaration history: %w", err)
	}
	for _, sub := range history {
		if sub.AccountID != nil && *sub.AccountID == accountID {
			proof.Submissions = append(proof.Submissions, sub)
		}
	}
	if len(proof.Submissions) == 0 {
		return ErrAnnouncementNotFound
	}
	versions, err := s.AnnouncementRepo.ListDeclarationVersions(ctx, tenantID, announcementID)
	if err != nil {
		return fmt.Errorf("parent: load declaration versions: %w", err)
	}
	for _, v := range versions {
		proof.Versions[v.ID] = v
	}
	if err := s.loadProofSignatures(ctx, proof, tenantID); err != nil {
		return fmt.Errorf("parent: load declaration signatures: %w", err)
	}
	proof.IntegrityOK = s.declarationProofIntact(proof)
	return nil
}

// declarationProofIntact recomputes the checksums of the guardian's answers
// and of the versions they answered on.
func (s *Service) declarationProofIntact(proof *DeclarationProof) bool {
	for _, sub := range proof.Submissions {
		if s.declarationRecordHash(sub) != sub.RecordHash {
			return false
		}
		v, ok := proof.Versions[sub.VersionID]
		if !ok || s.declarationContentHash(v) != v.ContentHash || sub.ContentHash != v.ContentHash {
			return false
		}
		if !proofSignatureIntact(proof, sub) {
			return false
		}
	}
	return true
}

// countOwedReadDeclarations counts the Erklärungen the account has opened but
// still owes an answer for. Unread ones are already in the unread count; an
// opened one must keep the badge until it is answered, like an open poll.
func (s *Service) countOwedReadDeclarations(ctx context.Context, accountID int64, tenantIDs []int64) (int, error) {
	read, err := s.AnnouncementRepo.ReadOpenDeclarations(ctx, accountID, tenantIDs)
	if err != nil || len(read) == 0 {
		return 0, err
	}
	items := make([]*AnnouncementFeedItem, 0, len(read))
	ids := make([]int64, 0, len(read))
	for id, deadline := range read {
		items = append(items, &AnnouncementFeedItem{ID: id, ResponseDeadline: deadline})
		ids = append(ids, id)
	}
	facts, err := s.loadDeclarationFeedFacts(ctx, accountID, ids)
	if err != nil {
		return 0, err
	}
	now := time.Now()
	owed := 0
	for _, item := range items {
		if declarationOwed(s.Declarations, buildFeedDeclaration(s.Declarations, accountID, item, facts, now), now) {
			owed++
		}
	}
	return owed, nil
}

// declarationOwed reports whether the account still owes an answer for one of
// its children.
func declarationOwed(policy DeclarationPolicy, d *AnnouncementFeedDeclaration, now time.Time) bool {
	for _, c := range d.Children {
		mine := ""
		if c.MyAction != nil {
			mine = *c.MyAction
		}
		if policy.OwesAction(d.Deadline, c.CanSubmit, mine, c.State, now) {
			return true
		}
	}
	return false
}
