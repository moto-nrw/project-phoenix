package announcement

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	deliveryPolicy "github.com/moto-nrw/project-phoenix/modules/delivery/devices"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func declarationTestInput() Input {
	return Input{
		Title: "Ausflug", Body: "Text", DeliveryMode: ParentAnnouncementDeliveryDeclaration,
		Targets:     []TargetInput{{TargetType: AnnouncementTargetSchoolAll}},
		Declaration: AnnouncementDeclarationSettings{Kind: DeclarationKindConsent},
	}
}

func TestNormalizeDeclarationCompletesAndRefuses(t *testing.T) {
	t.Parallel()

	in := declarationTestInput()
	in.Declaration.Kind = ""
	in.RequiresAcknowledgement = true
	_, err := normalizeInput(&in)
	require.NoError(t, err)
	assert.Equal(t, DeclarationKindConsent, in.Declaration.Kind, "an Einverständnis is the only kind")
	assert.Equal(t, DeclarationSignersAny, in.Declaration.Signers, "one guardian is the default")
	assert.False(t, in.RequiresAcknowledgement, "a declaration is not a read confirmation")

	cases := map[string]func(in *Input){
		// A read confirmation is the Elternbrief's Lesebestätigung.
		"no read confirmation": func(in *Input) { in.Declaration.Kind = "acknowledgement" },
		"unknown kind":         func(in *Input) { in.Declaration.Kind = "signature" },
		"not also a poll":      func(in *Input) { in.ResponseType = ParentAnnouncementResponseSingleChoice },
		"no open enrollment": func(in *Input) {
			in.Targets = []TargetInput{{TargetType: AnnouncementTargetPendingEnrollment}}
		},
		"deadline in the past": func(in *Input) {
			past := time.Now().Add(-time.Hour)
			in.ResponseDeadline = &past
		},
		"deadline after expiry": func(in *Input) {
			expires := time.Now().Add(24 * time.Hour)
			deadline := expires.Add(time.Hour)
			in.ExpiresAt = &expires
			in.ResponseDeadline = &deadline
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			in := declarationTestInput()
			mutate(&in)
			_, err := normalizeInput(&in)
			assert.True(t, errors.Is(err, ErrValidation), "got %v", err)
		})
	}

	standard := declarationTestInput()
	standard.DeliveryMode = ""
	standard.Declaration.Revocable = true
	_, err = normalizeInput(&standard)
	require.NoError(t, err)
	assert.Equal(t, AnnouncementDeclarationSettings{}, standard.Declaration,
		"another mode never keeps declaration settings")
}

func TestDeclarationMailNamesTheDeadlineButNotTheText(t *testing.T) {
	t.Parallel()
	deadline := time.Date(2026, 10, 15, 21, 59, 0, 0, time.UTC)
	a := &ParentAnnouncement{
		Title: "Ausflug", Body: "Geheimer Text", DeliveryMode: ParentAnnouncementDeliveryDeclaration,
		ResponseDeadline: &deadline,
	}
	spec := publishMailSpec(a)
	assert.Equal(t, declarationEmailKicker, spec.kicker)
	assert.Contains(t, spec.intro(""), "15.10.2026")
	assert.Empty(t, spec.body)
}

// declarationRepo is an in-memory announcement store for the staff lifecycle
// of an Erklärung; everything the lifecycle does not touch panics through the
// embedded nil interface.
type declarationRepo struct {
	ParentAnnouncementRepository
	announcement *ParentAnnouncement
	versions     []*DeclarationVersion
	submissions  []*DeclarationSubmission
	deleted      bool
}

func (r *declarationRepo) FindByID(context.Context, int64) (*ParentAnnouncement, error) {
	copied := *r.announcement
	return &copied, nil
}

func (r *declarationRepo) FindByIDForUpdate(ctx context.Context, id int64) (*ParentAnnouncement, error) {
	return r.FindByID(ctx, id)
}

func (r *declarationRepo) PublishIfDraft(_ context.Context, _ int64, at time.Time) (bool, error) {
	if r.announcement.PublishedAt != nil {
		return false, nil
	}
	r.announcement.PublishedAt = &at
	return true, nil
}

func (r *declarationRepo) SetPublished(_ context.Context, _ int64, at *time.Time) error {
	r.announcement.PublishedAt = at
	return nil
}

func (r *declarationRepo) Update(_ context.Context, a *ParentAnnouncement) error {
	r.announcement = a
	return nil
}

func (r *declarationRepo) ReplaceTargets(context.Context, int64, int64, []*ParentAnnouncementTarget) error {
	return nil
}

func (r *declarationRepo) ReplaceOptions(context.Context, int64, int64, []*ParentAnnouncementOption) error {
	return nil
}

func (r *declarationRepo) ListTargets(context.Context, int64) ([]*ParentAnnouncementTarget, error) {
	return nil, nil
}

func (r *declarationRepo) AudienceRecipients(context.Context, int64, int64) ([]*AnnouncementRecipientStatus, error) {
	return nil, nil
}

func (r *declarationRepo) Delete(context.Context, int64) error {
	r.deleted = true
	return nil
}

func (r *declarationRepo) LatestDeclarationVersion(context.Context, int64, int64) (*DeclarationVersion, error) {
	if len(r.versions) == 0 {
		return nil, nil
	}
	return r.versions[len(r.versions)-1], nil
}

func (r *declarationRepo) ListDeclarationVersions(context.Context, int64, int64) ([]*DeclarationVersion, error) {
	out := make([]*DeclarationVersion, 0, len(r.versions))
	for i := len(r.versions) - 1; i >= 0; i-- {
		out = append(out, r.versions[i])
	}
	return out, nil
}

func (r *declarationRepo) InsertDeclarationVersion(_ context.Context, v *DeclarationVersion) error {
	v.ID = int64(len(r.versions) + 100)
	r.versions = append(r.versions, v)
	return nil
}

func (r *declarationRepo) CountDeclarationSubmissions(context.Context, int64, int64) (int, error) {
	return len(r.submissions), nil
}

func (r *declarationRepo) ListDeclarationSubmissions(context.Context, int64, int64) ([]*DeclarationSubmission, error) {
	return r.submissions, nil
}

func (r *declarationRepo) DeclarationChildren(context.Context, int64, int64) ([]*DeclarationChild, error) {
	return []*DeclarationChild{{StudentID: 5, FirstName: "Mia", LastName: "Muster"}}, nil
}

func (r *declarationRepo) DeclarationSigners(context.Context, int64, []int64) ([]*DeclarationSigner, error) {
	return []*DeclarationSigner{{StudentID: 5, AccountID: 24, FirstName: "Klaus", LastName: "Schneider"}}, nil
}

type digestStub struct{ *stubPurger }

// securityFingerprints binds the declaration fingerprints to the Security
// Runtime, as the root does.
type securityFingerprints struct{}

func (securityFingerprints) Content(content []byte) string {
	return securityruntime.Fingerprint(content)
}

func (securityFingerprints) Reader(r io.Reader) (string, int64, error) {
	return securityruntime.ReaderFingerprint(r)
}

func (digestStub) AttachmentDigests(context.Context, int64, func(io.Reader) (string, int64, error)) ([]AttachmentDigest, error) {
	return []AttachmentDigest{{AttachmentID: 7, Filename: "Erlaubnis.pdf", SizeBytes: 3, SHA256: "ab"}}, nil
}

func TestDeclarationLifecycleVersionsProofAndDeleteGuard(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	announcement := &ParentAnnouncement{
		Title: "Ausflug", Body: "Freitag", Priority: ParentAnnouncementPriorityInfo, Active: true,
		DeliveryMode: ParentAnnouncementDeliveryDeclaration,
		Declaration:  AnnouncementDeclarationSettings{Kind: DeclarationKindConsent, Signers: DeclarationSignersAny},
	}
	announcement.ID = 9
	announcement.SetTenantID(3)
	repo := &declarationRepo{announcement: announcement}
	svc := NewService(ServiceConfig{Repo: repo, Settings: newsOn{}, Fingerprints: securityFingerprints{}, Attachments: digestStub{&stubPurger{}}}, testpkg.TenantTransactionRuntime{}, deliveryPolicy.ParentDeliveryPolicy{}).(*service)

	_, err := svc.Publish(ctx, announcement.ID)
	require.NoError(t, err)
	require.Len(t, repo.versions, 1)
	first := repo.versions[0]
	assert.Equal(t, 1, first.VersionNo)
	assert.Equal(t, securityruntime.Fingerprint(first.CanonicalContent()), first.ContentHash)
	require.Len(t, first.Attachments, 1)

	// Unchanged republication keeps the version.
	_, err = svc.Unpublish(ctx, announcement.ID)
	require.NoError(t, err)
	_, err = svc.Publish(ctx, announcement.ID)
	require.NoError(t, err)
	assert.Len(t, repo.versions, 1)

	// A declaration, then a correction: version 2, the proof stays.
	account := int64(24)
	sub := &DeclarationSubmission{
		TenantID: 3, AnnouncementID: 9, VersionID: first.ID, StudentID: 5, AccountID: &account,
		SignerName: "Klaus Schneider", GuardianRole: "primary_guardian", Action: DeclarationActionAgreed,
		Method: DeclarationMethodSimpleElectronic, ContentHash: first.ContentHash,
		SubmittedAt: time.Date(2026, 9, 27, 8, 0, 0, 0, time.UTC),
	}
	sub.RecordHash = securityruntime.Fingerprint(sub.CanonicalRecord())
	repo.submissions = []*DeclarationSubmission{sub}

	status, err := svc.ParentDeclarationStatus(ctx, announcement.ID)
	require.NoError(t, err)
	assert.True(t, status.IntegrityAllGood)
	assert.Equal(t, DeclarationStateAgreed, status.Children[0].State)

	_, err = svc.Unpublish(ctx, announcement.ID)
	require.NoError(t, err)
	repo.announcement.Body = "Donnerstag"
	_, err = svc.Publish(ctx, announcement.ID)
	require.NoError(t, err)
	require.Len(t, repo.versions, 2)
	assert.Equal(t, 2, repo.versions[1].VersionNo)

	status, err = svc.ParentDeclarationStatus(ctx, announcement.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, status.CurrentVersion.VersionNo)
	assert.Equal(t, DeclarationStateOpen, status.Children[0].State, "a declaration on version 1 does not settle version 2")
	require.Len(t, status.Submissions, 1)
	assert.Equal(t, 1, status.Submissions[0].VersionNo)

	// A row changed behind the application's back is reported.
	sub.SignerName = "Jemand anderes"
	status, err = svc.ParentDeclarationStatus(ctx, announcement.ID)
	require.NoError(t, err)
	assert.False(t, status.Submissions[0].IntegrityOK)
	assert.False(t, status.IntegrityAllGood)

	// Attachments are fixed once a version exists, and the proof blocks deletion.
	_, err = svc.Unpublish(ctx, announcement.ID)
	require.NoError(t, err)
	editable, err := svc.AnnouncementEditable(ctx, announcement.ID)
	require.NoError(t, err)
	assert.False(t, editable)
	require.ErrorIs(t, svc.Delete(ctx, announcement.ID), ErrDeclarationHasSubmissions)
	assert.False(t, repo.deleted)
}

func TestDeclarationWithSubmissionsKeepsItsDeliveryMode(t *testing.T) {
	t.Parallel()
	announcement := &ParentAnnouncement{
		Title: "Ausflug", Body: "Freitag", Priority: ParentAnnouncementPriorityInfo, Active: true,
		DeliveryMode: ParentAnnouncementDeliveryDeclaration,
		Declaration:  AnnouncementDeclarationSettings{Kind: DeclarationKindConsent, Signers: DeclarationSignersAny},
	}
	announcement.ID = 9
	announcement.SetTenantID(3)
	repo := &declarationRepo{announcement: announcement, submissions: []*DeclarationSubmission{{ID: 1}}}
	svc := NewService(ServiceConfig{Repo: repo, Settings: newsOn{}}, testpkg.TenantTransactionRuntime{}, deliveryPolicy.ParentDeliveryPolicy{}).(*service)

	in := declarationTestInput()
	in.DeliveryMode = ParentAnnouncementDeliveryStandard
	_, err := svc.Update(context.Background(), announcement.ID, in)
	require.ErrorIs(t, err, ErrDeclarationHasSubmissions)
	assert.Equal(t, ParentAnnouncementDeliveryDeclaration, repo.announcement.DeliveryMode)
}

func TestDeclarationWithSubmissionsKeepsItsRules(t *testing.T) {
	t.Parallel()
	deadline := time.Now().Add(72 * time.Hour)
	announcement := &ParentAnnouncement{
		Title: "Ausflug", Body: "Freitag", Priority: ParentAnnouncementPriorityInfo, Active: true,
		DeliveryMode:     ParentAnnouncementDeliveryDeclaration,
		ResponseDeadline: &deadline,
		Declaration: AnnouncementDeclarationSettings{
			Kind: DeclarationKindConsent, Signers: DeclarationSignersAny,
		},
	}
	announcement.ID = 9
	announcement.SetTenantID(3)
	repo := &declarationRepo{announcement: announcement, submissions: []*DeclarationSubmission{{ID: 1}}}
	svc := NewService(ServiceConfig{Repo: repo, Settings: newsOn{}}, testpkg.TenantTransactionRuntime{}, deliveryPolicy.ParentDeliveryPolicy{}).(*service)

	in := declarationTestInput()
	in.Declaration.Signers = DeclarationSignersAll
	nextDeadline := deadline.Add(time.Hour)
	in.ResponseDeadline = &nextDeadline
	_, err := svc.Update(context.Background(), announcement.ID, in)

	require.ErrorIs(t, err, ErrDeclarationHasSubmissions)
	assert.Equal(t, DeclarationSignersAny, repo.announcement.Declaration.Signers)
	assert.Equal(t, deadline, *repo.announcement.ResponseDeadline)
}

type newsOn struct{ Settings }

func (newsOn) ParentNewsEnabled(context.Context) (bool, error) { return true, nil }

func TestDeclarationKeepsItsDeadlineThroughTheFullNormalisation(t *testing.T) {
	t.Parallel()
	deadline := time.Now().Add(72 * time.Hour)
	in := declarationTestInput()
	in.ResponseDeadline = &deadline
	_, err := normalizeInput(&in)
	require.NoError(t, err)
	_, err = normalizePollOptions(&in)
	require.NoError(t, err)
	require.NotNil(t, in.ResponseDeadline, "the Frist of an Erklärung must survive the poll normalisation")

	mitteilung := declarationTestInput()
	mitteilung.DeliveryMode = ""
	mitteilung.ResponseDeadline = &deadline
	_, err = normalizeInput(&mitteilung)
	require.NoError(t, err)
	_, err = normalizePollOptions(&mitteilung)
	require.NoError(t, err)
	assert.Nil(t, mitteilung.ResponseDeadline)
}
