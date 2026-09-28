package announcement

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	declarations "github.com/moto-nrw/project-phoenix/services/parentmessaging"
)

// Erklärungen (#3430): the staff side. An Erklärung is an announcement in
// delivery mode "declaration". Publishing freezes a version (wording and
// attachment digests); guardians declare per child through the parent flow;
// staff reads the status and exports the proof here.

// ErrDeclarationHasSubmissions refuses to delete a declaration that carries
// proof. Retracting it stays possible.
var (
	ErrDeclarationHasSubmissions = errors.New("announcement: guardians already declared on this Erklärung")
	// ErrNotDeclaration reports a declaration read on another mode.
	ErrNotDeclaration = errors.New("announcement: not an Erklärung")
)

// DeclarationStatus is the staff view of an Erklärung: versions, each
// reached child with its state and signers, and the full history, every
// stored hash re-checked.
type DeclarationStatus struct {
	Title            string
	Settings         usersModels.AnnouncementDeclarationSettings
	Deadline         *time.Time
	CurrentVersion   *DeclarationVersionView
	Versions         []DeclarationVersionView
	ChildrenTotal    int
	Summary          map[string]int
	Children         []DeclarationChildView
	Submissions      []DeclarationSubmissionView
	GeneratedAt      time.Time
	IntegrityAllGood bool
}

// DeclarationAttachmentView is one frozen attachment digest.
type DeclarationAttachmentView struct {
	Filename    string
	ContentType string
	SizeBytes   int64
	SHA256      string
}

// DeclarationVersionView is one frozen publication.
type DeclarationVersionView struct {
	ID          int64
	VersionNo   int
	Title       string
	Body        string
	Kind        string
	ContentHash string
	PublishedAt time.Time
	Attachments []DeclarationAttachmentView
	IntegrityOK bool
}

// DeclarationSubmissionView is one stored declaration.
type DeclarationSubmissionView struct {
	ID                int64
	StudentID         int64
	StudentFirstName  string
	StudentLastName   string
	AccountID         *int64
	SignerName        string
	GuardianRole      string
	Action            string
	Method            string
	PasswordConfirmed bool
	VersionNo         int
	ContentHash       string
	RecordHash        string
	SubmittedAt       time.Time
	IntegrityOK       bool
}

// DeclarationSignerView is one eligible guardian of a child.
type DeclarationSignerView struct {
	AccountID   int64
	FirstName   string
	LastName    string
	Action      *string
	SubmittedAt *time.Time
}

// DeclarationChildView is one reached child.
type DeclarationChildView struct {
	StudentID   int64
	FirstName   string
	LastName    string
	SchoolClass string
	State       string
	Signers     []DeclarationSignerView
}

// DeclarationSupport is the Erklärung part of the staff service.
type DeclarationSupport interface {
	ParentDeclarationStatus(ctx context.Context, id int64) (*DeclarationStatus, error)
	DeclarationFrozen(ctx context.Context, a *usersModels.ParentAnnouncement) (bool, error)
}

const declarationReminderKicker = "Einverständnis: Ihre Antwort fehlt"

// normalizeDeclaration validates and completes the declaration settings. It
// runs inside normalizeDelivery, after the delivery mode is known.
func normalizeDeclaration(in *Input) error {
	if in.DeliveryMode != usersModels.ParentAnnouncementDeliveryDeclaration {
		in.Declaration = usersModels.AnnouncementDeclarationSettings{}
		return nil
	}
	// An Einverständnis is the only kind; the stored kind keeps every proof
	// naming what was asked.
	if in.Declaration.Kind == "" {
		in.Declaration.Kind = usersModels.DeclarationKindConsent
	}
	if in.Declaration.Signers == "" {
		in.Declaration.Signers = usersModels.DeclarationSignersAny
	}
	d := in.Declaration
	if err := declarations.ValidateDeclarationSettings(d.Kind, d.Signers); err != nil {
		return fmt.Errorf("%w: declaration settings (kind %q, signers %q, revocable %t)", ErrValidation, d.Kind, d.Signers, d.Revocable)
	}
	// A declaration is answered per child by guardians who may declare; it is
	// neither a poll nor a read confirmation, and an applicant without a
	// linked child has nothing to declare for.
	if in.ResponseType != "" && in.ResponseType != usersModels.ParentAnnouncementResponseNone {
		return fmt.Errorf("%w: an Erklärung cannot also be a poll", ErrValidation)
	}
	for _, t := range in.Targets {
		if t.TargetType == usersModels.AnnouncementTargetPendingEnrollment {
			return fmt.Errorf("%w: an Erklärung cannot target pending enrollments", ErrValidation)
		}
	}
	if in.EmailAudience != usersModels.EmailAudiencePortalOnly {
		return fmt.Errorf("%w: an Erklärung is mailed to the portal audience only", ErrValidation)
	}
	in.ResponseType = usersModels.ParentAnnouncementResponseNone
	in.RequiresAcknowledgement = false
	if in.ResponseDeadline != nil && !in.ResponseDeadline.After(time.Now()) {
		return fmt.Errorf("%w: the deadline must lie in the future", ErrValidation)
	}
	// Parents cannot answer an Einverständnis after it disappears from their
	// feed, so its Frist may not outlast the announcement itself.
	if in.ResponseDeadline != nil && in.ExpiresAt != nil && in.ResponseDeadline.After(*in.ExpiresAt) {
		return fmt.Errorf("%w: response_deadline must not be after expires_at", ErrValidation)
	}
	return nil
}

// settlePublication re-checks the committed row after the atomic publish flip
// and freezes an Erklärung's version in the same transaction. A concurrent
// edit may have moved expires_at, the deadline or the reminder into the past
// between the pre-publish check and the flip; publishing then is invalid
// (invisible to parents, immutable), so it fails with a 5xx and the tenant
// tx rolls the flip back for staff to retry.
func (s *service) settlePublication(ctx context.Context, fresh *usersModels.ParentAnnouncement) error {
	now := time.Now()
	if fresh.ExpiresAt != nil && !fresh.ExpiresAt.After(now) {
		return fmt.Errorf("announcement: draft expired concurrently during publish; rolled back")
	}
	if fresh.ResponseDeadline != nil && !fresh.ResponseDeadline.After(now) {
		return fmt.Errorf("announcement: draft response deadline elapsed concurrently during publish; rolled back")
	}
	if err := validateReminder(fresh.ReminderAt, fresh.ExpiresAt, now); err != nil {
		return err
	}
	return s.freezeDeclaration(ctx, fresh)
}

// freezeDeclaration writes the version guardians will declare on, inside the
// publish transaction. A republication without a change to title, body, kind
// or attachments keeps the current version, so declarations already given
// stay valid; any change makes a new version and asks everybody again.
func (s *service) freezeDeclaration(ctx context.Context, a *usersModels.ParentAnnouncement) error {
	if !a.IsDeclaration() {
		return nil
	}
	if s.attachments == nil {
		return fmt.Errorf("announcement: attachment digests are not wired; refusing to publish declaration %d", a.ID)
	}
	digests, err := s.attachments.AttachmentDigests(ctx, a.ID, authorize.ReaderFingerprint)
	if err != nil {
		return fmt.Errorf("announcement: digest declaration attachments: %w", err)
	}
	version := &usersModels.DeclarationVersion{
		TenantID: a.GetTenantID(), AnnouncementID: a.ID, Title: a.Title, Body: a.Body,
		Kind: a.Declaration.Kind, PublishedAt: time.Now(),
		Attachments: make([]usersModels.DeclarationAttachmentDigest, 0, len(digests)),
	}
	for _, d := range digests {
		version.Attachments = append(version.Attachments, usersModels.DeclarationAttachmentDigest(d))
	}
	version.ContentHash = DeclarationContentHash(version)
	latest, err := s.repo.LatestDeclarationVersion(ctx, a.GetTenantID(), a.ID)
	if err != nil {
		return fmt.Errorf("announcement: load declaration version: %w", err)
	}
	if latest != nil && latest.ContentHash == version.ContentHash {
		return nil
	}
	version.VersionNo = 1
	if latest != nil {
		version.VersionNo = latest.VersionNo + 1
	}
	if err := s.repo.InsertDeclarationVersion(ctx, version); err != nil {
		return fmt.Errorf("announcement: freeze declaration version: %w", err)
	}
	s.logger.Info("parent declaration version frozen",
		slog.Int64("announcement_id", a.ID),
		slog.Int("version_no", version.VersionNo),
		slog.Int("attachment_count", len(version.Attachments)),
	)
	return nil
}

// guardDeclarationRetention keeps evidence reachable. A declaration with
// submissions cannot be deleted or changed into another delivery mode, because
// the status and proof routes deliberately address it as a declaration.
func (s *service) guardDeclarationRetention(ctx context.Context, a *usersModels.ParentAnnouncement, remainsDeclaration bool) error {
	if !a.IsDeclaration() || remainsDeclaration {
		return nil
	}
	count, err := s.repo.CountDeclarationSubmissions(ctx, a.GetTenantID(), a.ID)
	if err != nil {
		return fmt.Errorf("announcement: count declaration submissions: %w", err)
	}
	if count > 0 {
		return ErrDeclarationHasSubmissions
	}
	return nil
}

// guardDeclarationDelete refuses to delete a declaration with submissions.
// The foreign keys would refuse it too; this turns the refusal into a clear
// conflict instead of a server error.
func (s *service) guardDeclarationDelete(ctx context.Context, a *usersModels.ParentAnnouncement) error {
	return s.guardDeclarationRetention(ctx, a, false)
}

// guardDeclarationModeChange keeps a withdrawn declaration as a declaration
// after somebody has answered it, so its status and proof remain reachable.
func (s *service) guardDeclarationModeChange(ctx context.Context, a *usersModels.ParentAnnouncement, deliveryMode string) error {
	return s.guardDeclarationRetention(ctx, a, deliveryMode == usersModels.ParentAnnouncementDeliveryDeclaration)
}

// DeclarationFrozen reports whether a declaration has been published before,
// which fixes its attachments.
func (s *service) DeclarationFrozen(ctx context.Context, a *usersModels.ParentAnnouncement) (bool, error) {
	if !a.IsDeclaration() {
		return false, nil
	}
	version, err := s.repo.LatestDeclarationVersion(ctx, a.GetTenantID(), a.ID)
	if err != nil {
		return false, fmt.Errorf("announcement: load declaration version: %w", err)
	}
	return version != nil, nil
}

func declarationRules(a *usersModels.ParentAnnouncement) declarations.DeclarationRules {
	return declarations.DeclarationRules{
		Kind: a.Declaration.Kind, Signers: a.Declaration.Signers,
		Revocable: a.Declaration.Revocable, Deadline: a.ResponseDeadline,
	}
}

// declarationFacts is everything the status is derived from.
type declarationFacts struct {
	announcement *usersModels.ParentAnnouncement
	versions     []*usersModels.DeclarationVersion
	children     []*usersModels.DeclarationChild
	signers      map[int64][]*usersModels.DeclarationSigner
	submissions  []*usersModels.DeclarationSubmission
}

func (s *service) loadDeclarationFacts(ctx context.Context, id int64) (*declarationFacts, error) {
	a, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("announcement: load declaration: %w", err)
	}
	if a == nil {
		return nil, ErrNotFound
	}
	if !a.IsDeclaration() {
		return nil, ErrNotDeclaration
	}
	tenantID := a.GetTenantID()
	facts := &declarationFacts{announcement: a, signers: map[int64][]*usersModels.DeclarationSigner{}}
	if facts.versions, err = s.repo.ListDeclarationVersions(ctx, tenantID, id); err != nil {
		return nil, fmt.Errorf("announcement: list declaration versions: %w", err)
	}
	if facts.children, err = s.repo.DeclarationChildren(ctx, tenantID, id); err != nil {
		return nil, fmt.Errorf("announcement: declaration children: %w", err)
	}
	studentIDs := make([]int64, 0, len(facts.children))
	for _, c := range facts.children {
		studentIDs = append(studentIDs, c.StudentID)
	}
	signers, err := s.repo.DeclarationSigners(ctx, tenantID, studentIDs)
	if err != nil {
		return nil, fmt.Errorf("announcement: declaration signers: %w", err)
	}
	for _, signer := range signers {
		facts.signers[signer.StudentID] = append(facts.signers[signer.StudentID], signer)
	}
	if facts.submissions, err = s.repo.ListDeclarationSubmissions(ctx, tenantID, id); err != nil {
		return nil, fmt.Errorf("announcement: list declaration submissions: %w", err)
	}
	return facts, nil
}

// currentVersion is the newest version, or nil before the first publication.
func (f *declarationFacts) currentVersion() *usersModels.DeclarationVersion {
	if len(f.versions) == 0 {
		return nil
	}
	return f.versions[0]
}

// latestActions maps student → account → latest action on the current
// version. Submissions arrive newest first.
func (f *declarationFacts) latestActions() map[int64]map[int64]*usersModels.DeclarationSubmission {
	out := map[int64]map[int64]*usersModels.DeclarationSubmission{}
	current := f.currentVersion()
	if current == nil {
		return out
	}
	for _, sub := range f.submissions {
		if sub.VersionID != current.ID || sub.AccountID == nil {
			continue
		}
		byAccount := out[sub.StudentID]
		if byAccount == nil {
			byAccount = map[int64]*usersModels.DeclarationSubmission{}
			out[sub.StudentID] = byAccount
		}
		if _, seen := byAccount[*sub.AccountID]; !seen {
			byAccount[*sub.AccountID] = sub
		}
	}
	return out
}

// ParentDeclarationStatus assembles the staff view: versions, each reached
// child with its state and signers, and the full history with an integrity
// check of every stored hash.
func (s *service) ParentDeclarationStatus(ctx context.Context, id int64) (*DeclarationStatus, error) {
	facts, err := s.loadDeclarationFacts(ctx, id)
	if err != nil {
		return nil, err
	}
	a := facts.announcement
	now := time.Now()
	out := &DeclarationStatus{
		Title:       a.Title,
		Settings:    a.Declaration,
		Deadline:    a.ResponseDeadline,
		Summary:     map[string]int{},
		GeneratedAt: now,
	}
	out.IntegrityAllGood = true
	versionNo := map[int64]int{}
	for _, v := range facts.versions {
		version := publicVersion(v)
		out.IntegrityAllGood = out.IntegrityAllGood && version.IntegrityOK
		out.Versions = append(out.Versions, version)
		versionNo[v.ID] = v.VersionNo
	}
	if len(out.Versions) > 0 {
		current := out.Versions[0]
		out.CurrentVersion = &current
	}
	out.Children = declarationChildStatuses(facts, now)
	out.ChildrenTotal = len(out.Children)
	names := map[int64]*usersModels.DeclarationChild{}
	for _, c := range facts.children {
		names[c.StudentID] = c
	}
	for _, c := range out.Children {
		out.Summary[c.State]++
	}
	for _, sub := range facts.submissions {
		record := publicSubmission(sub, versionNo[sub.VersionID], names[sub.StudentID])
		out.IntegrityAllGood = out.IntegrityAllGood && record.IntegrityOK
		out.Submissions = append(out.Submissions, record)
	}
	return out, nil
}

func declarationChildStatuses(facts *declarationFacts, now time.Time) []DeclarationChildView {
	rules := declarationRules(facts.announcement)
	latest := facts.latestActions()
	out := make([]DeclarationChildView, 0, len(facts.children))
	for _, c := range facts.children {
		status := DeclarationChildView{
			StudentID: c.StudentID, FirstName: c.FirstName, LastName: c.LastName, SchoolClass: c.SchoolClass,
			Signers: []DeclarationSignerView{},
		}
		eligible := make([]int64, 0, len(facts.signers[c.StudentID]))
		actions := map[int64]string{}
		for _, signer := range facts.signers[c.StudentID] {
			eligible = append(eligible, signer.AccountID)
			row := DeclarationSignerView{
				AccountID: signer.AccountID, FirstName: signer.FirstName, LastName: signer.LastName,
			}
			if sub, ok := latest[c.StudentID][signer.AccountID]; ok {
				action, at := sub.Action, sub.SubmittedAt
				row.Action, row.SubmittedAt = &action, &at
				actions[signer.AccountID] = action
			}
			status.Signers = append(status.Signers, row)
		}
		status.State = declarations.DeclarationChildState(rules, eligible, actions, now)
		out = append(out, status)
	}
	return out
}

func publicVersion(v *usersModels.DeclarationVersion) DeclarationVersionView {
	version := DeclarationVersionView{
		ID: v.ID, VersionNo: v.VersionNo, Title: v.Title, Body: v.Body, Kind: v.Kind,
		ContentHash: v.ContentHash, PublishedAt: v.PublishedAt,
		Attachments: make([]DeclarationAttachmentView, 0, len(v.Attachments)),
		IntegrityOK: DeclarationContentHash(v) == v.ContentHash,
	}
	for _, a := range v.Attachments {
		version.Attachments = append(version.Attachments, DeclarationAttachmentView{
			Filename: a.Filename, ContentType: a.ContentType, SizeBytes: a.SizeBytes, SHA256: a.SHA256,
		})
	}
	return version
}

func publicSubmission(sub *usersModels.DeclarationSubmission, versionNo int, child *usersModels.DeclarationChild) DeclarationSubmissionView {
	record := DeclarationSubmissionView{
		ID: sub.ID, StudentID: sub.StudentID, AccountID: sub.AccountID, SignerName: sub.SignerName,
		GuardianRole: sub.GuardianRole, Action: sub.Action, Method: sub.Method,
		PasswordConfirmed: sub.PasswordConfirmed, VersionNo: versionNo, ContentHash: sub.ContentHash,
		RecordHash: sub.RecordHash, SubmittedAt: sub.SubmittedAt,
		IntegrityOK: DeclarationRecordHash(sub) == sub.RecordHash,
	}
	if child != nil {
		record.StudentFirstName, record.StudentLastName = child.FirstName, child.LastName
	}
	return record
}

// remindDeclaration reaches every guardian who still owes a declaration for
// a child that is not settled yet. Guardians who already declared are left
// alone, and so are children somebody else already settled under "any".
func (s *service) remindDeclaration(ctx context.Context, a *usersModels.ParentAnnouncement) (int, error) {
	if !a.IsPublished() || !a.Active {
		return 0, ErrNotPublished
	}
	now := time.Now()
	if a.ExpiresAt != nil && !a.ExpiresAt.After(now) {
		return 0, ErrNotPublished
	}
	enabled, err := s.newsEnabled(ctx)
	if err != nil {
		return 0, fmt.Errorf("announcement: resolve news flag: %w", err)
	}
	if !enabled {
		return 0, ErrNewsDisabled
	}
	facts, err := s.loadDeclarationFacts(ctx, a.ID)
	if err != nil {
		return 0, err
	}
	recipients := declarationReminderRecipients(facts, now)
	if len(recipients) == 0 {
		return 0, nil
	}
	intro := "für Ihr Kind fehlt noch Ihre Antwort. Bitte stimmen Sie im Eltern-Portal zu oder lehnen Sie ab."
	emailed, err := s.enqueueReminderEmails(ctx, a, recipients, declarationReminderKicker, intro, s.letterPortalURL(a.ID))
	if err != nil {
		return 0, err
	}
	delivered := map[int64]struct{}{}
	for _, accountID := range emailed {
		delivered[accountID] = struct{}{}
	}
	s.logger.Info("parent declaration reminder sent",
		slog.Int64("announcement_id", a.ID),
		slog.Int("recipient_count", len(delivered)),
	)
	return len(delivered), nil
}

func declarationReminderRecipients(facts *declarationFacts, now time.Time) []*usersModels.AnnouncementPollReminderRecipient {
	rules := declarationRules(facts.announcement)
	latest := facts.latestActions()
	states := map[int64]string{}
	for _, c := range declarationChildStatuses(facts, now) {
		states[c.StudentID] = c.State
	}
	byAccount := map[int64]*usersModels.AnnouncementPollReminderRecipient{}
	for studentID, signers := range facts.signers {
		for _, signer := range signers {
			mine := ""
			if sub, ok := latest[studentID][signer.AccountID]; ok {
				mine = sub.Action
			}
			if !declarations.DeclarationOwesAction(rules, true, mine, states[studentID], now) || signer.Email == "" {
				continue
			}
			if _, ok := byAccount[signer.AccountID]; !ok {
				byAccount[signer.AccountID] = &usersModels.AnnouncementPollReminderRecipient{
					AccountID: signer.AccountID, Email: strings.ToLower(signer.Email),
					FirstName: signer.FirstName, LastName: signer.LastName, PortalLocale: signer.PortalLocale,
				}
			}
		}
	}
	out := make([]*usersModels.AnnouncementPollReminderRecipient, 0, len(byAccount))
	for _, r := range byAccount {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AccountID < out[j].AccountID })
	return out
}

// declarationMailSpec tells the family that an answer is expected. The mail
// never carries the text: what they declare on is what the portal shows.
func declarationMailSpec(a *usersModels.ParentAnnouncement, spec mailSpec) mailSpec {
	if !a.IsDeclaration() {
		return spec
	}
	spec.kicker = declarationEmailKicker
	intro := "wir bitten Sie um Ihr Einverständnis. Sie können im Eltern-Portal zustimmen oder ablehnen."
	if a.ResponseDeadline != nil {
		intro = fmt.Sprintf("wir bitten Sie bis zum %s um Ihr Einverständnis. Sie können im Eltern-Portal zustimmen oder ablehnen.",
			a.ResponseDeadline.In(berlinLocation()).Format("02.01.2006"))
	}
	spec.intro = func(string) string { return intro }
	return spec
}

func berlinLocation() *time.Location {
	location, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		return time.FixedZone("CET", 60*60)
	}
	return location
}

// DeclarationContentHash is the SHA-256 over a version's canonical content.
func DeclarationContentHash(version *usersModels.DeclarationVersion) string {
	return authorize.ContentFingerprint(version.CanonicalContent())
}

// DeclarationRecordHash is the SHA-256 over a submission's canonical record.
// The parent flow computes the same value when it stores the row.
func DeclarationRecordHash(submission *usersModels.DeclarationSubmission) string {
	return authorize.ContentFingerprint(submission.CanonicalRecord())
}

// deadlineWithoutPoll is the deadline an announcement without a poll keeps:
// an Erklärung has its own Frist (#3430), every other mode none.
func deadlineWithoutPoll(in *Input) *time.Time {
	if in.DeliveryMode == usersModels.ParentAnnouncementDeliveryDeclaration {
		return in.ResponseDeadline
	}
	return nil
}
