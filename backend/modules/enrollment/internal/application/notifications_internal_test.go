package application

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type notificationSettingsStub struct {
	mode    string
	err     error
	term    string
	termErr error
}

func (s notificationSettingsStub) NotifyPerDecision(context.Context) (string, error) {
	return s.mode, s.err
}

func (s notificationSettingsStub) CareFacilityTerm(context.Context) (string, error) {
	return s.term, s.termErr
}

type modePinStub struct {
	pinned string
	err    error
	calls  []string
}

func (s *modePinStub) PinDecisionNotificationMode(_ context.Context, _ int64, proposed string) (string, error) {
	s.calls = append(s.calls, proposed)
	if s.pinned != "" {
		return s.pinned, s.err
	}
	return proposed, s.err
}

type decisionOutboxStub struct {
	mails []Mail
	err   error
}

func (s *decisionOutboxStub) EnqueueMail(_ context.Context, mail Mail) error {
	s.mails = append(s.mails, mail)
	return s.err
}

type schoolDirectoryStub struct {
	school *enrollment.School
	err    error
}

func (s schoolDirectoryStub) FindSchool(context.Context, int64) (*enrollment.School, error) {
	return s.school, s.err
}

// testFingerprint stands in for Security Runtime's content fingerprint: any
// stable, content-sensitive identity keeps the idempotency contract.
func testFingerprint(content []byte) string { return fmt.Sprintf("%x", content) }

func newNotificationsForTest(deps NotificationDependencies) *Notifications {
	if deps.Settings == nil {
		// The mail header reads the school's care-facility term (#6042);
		// an empty term keeps the header's "OGS" stand-in.
		deps.Settings = notificationSettingsStub{}
	}
	if deps.Fingerprint == nil {
		deps.Fingerprint = testFingerprint
	}
	if deps.Branding == nil {
		deps.Branding = func(baseURL, image string) (string, string) {
			return image, baseURL + "/images/moto-logo-mit-schriftzug.png"
		}
	}
	return NewNotifications(deps)
}

func TestResolveDecisionNotificationMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		settings NotificationSettings
		want     string
		wantErr  bool
	}{
		{name: "nil uses registry digest default", want: notifyPerDecisionDigest},
		{name: "empty uses registry digest default", settings: notificationSettingsStub{}, want: notifyPerDecisionDigest},
		{name: "immediate override", settings: notificationSettingsStub{mode: notifyPerDecisionImmediate}, want: notifyPerDecisionImmediate},
		{name: "resolution error", settings: notificationSettingsStub{err: errors.New("unavailable")}, wantErr: true},
		{name: "unknown mode", settings: notificationSettingsStub{mode: "later"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newNotificationsForTest(NotificationDependencies{Settings: tt.settings}).resolveDecisionNotificationMode(context.Background())
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDecisionDigestIdempotencyKeyTracksCanonicalStatusVector(t *testing.T) {
	t.Parallel()

	notifications := newNotificationsForTest(NotificationDependencies{})
	firstReview := time.Date(2026, time.July, 10, 12, 0, 0, 0, time.UTC)
	secondReview := firstReview.Add(time.Minute)
	children := []enrollment.DecisionChild{
		{ID: 12, Status: enrollment.ChildStatusRejected, ReviewedAt: &firstReview},
		{ID: 11, Status: enrollment.ChildStatusWaitlisted, ReviewedAt: &firstReview},
	}

	first := notifications.decisionDigestIdempotencyKey(9, children)
	reordered := notifications.decisionDigestIdempotencyKey(9, []enrollment.DecisionChild{children[1], children[0]})
	assert.Equal(t, first, reordered, "slice order must not change the material state key")

	children[1].Status = enrollment.ChildStatusApproved
	promoted := notifications.decisionDigestIdempotencyKey(9, children)
	assert.NotEqual(t, first, promoted, "a later status transition must create a new key")
	assert.Equal(t, promoted, notifications.decisionDigestIdempotencyKey(9, children), "same-state retries must remain stable")

	children[1].Status = enrollment.ChildStatusWaitlisted
	children[1].ReviewedAt = &secondReview
	repeatedStatus := notifications.decisionDigestIdempotencyKey(9, children)
	assert.NotEqual(t, first, repeatedStatus, "reopening and deciding the same final status is a new generation")
	previousGeneration := children[1]
	previousGeneration.ReviewedAt = &firstReview
	assert.NotEqual(t, notifications.decisionEmailIdempotencyKey(9, previousGeneration), notifications.decisionEmailIdempotencyKey(9, children[1]))
}

func decisionNotice(children ...enrollment.DecisionChild) enrollment.DecisionNotice {
	return enrollment.DecisionNotice{
		Request: enrollment.DecisionRequest{
			ID: 9, TenantID: 3, GuardianFirstName: "Anna", GuardianLastName: "Beispiel",
			GuardianEmail: "anna@example.test", StatusToken: "tok",
		},
		Children:   children,
		Phase:      &enrollment.Phase{Name: "Schuljahr 2026/27"},
		ParentsURL: "https://parents.example.test",
	}
}

func TestNotifyDecisionsPinsTheModeAndSendsOneDigestOnceResolved(t *testing.T) {
	t.Parallel()
	pin := &modePinStub{}
	outbox := &decisionOutboxStub{}
	notifications := newNotificationsForTest(NotificationDependencies{Modes: pin, Outbox: outbox})

	pending := decisionNotice(
		enrollment.DecisionChild{ID: 1, FirstName: "Lara", LastName: "B", Status: enrollment.ChildStatusApproved},
		enrollment.DecisionChild{ID: 2, FirstName: "Tim", LastName: "B", Status: enrollment.ChildStatusSubmitted},
	)
	mode, err := notifications.NotifyDecisions(context.Background(), pending)
	require.NoError(t, err)
	assert.Equal(t, notifyPerDecisionDigest, mode)
	assert.Equal(t, []string{notifyPerDecisionDigest}, pin.calls)
	assert.Empty(t, outbox.mails, "a digest waits until every child is resolved")

	resolved := decisionNotice(
		enrollment.DecisionChild{ID: 1, FirstName: "Lara", LastName: "B", Status: enrollment.ChildStatusApproved},
		enrollment.DecisionChild{ID: 2, FirstName: "Tim", LastName: "B", Status: enrollment.ChildStatusWithdrawn},
	)
	_, err = notifications.NotifyDecisions(context.Background(), resolved)
	require.NoError(t, err)
	require.Len(t, outbox.mails, 1)
	mail := outbox.mails[0]
	assert.Equal(t, enrollment.MailKindDecisionDigest, mail.Kind)
	assert.Equal(t, int64(9), mail.RelatedEntityID)
	assert.Equal(t, []string{"Lara B"}, mail.Payload["approved_names"])
	assert.Equal(t, []string{"Tim B"}, mail.Payload["withdrawn_names"])
	assert.Equal(t, []string{}, mail.Payload["rejected_names"])
	assert.Equal(t, "https://parents.example.test/anmeldung/status/tok", mail.Payload[enrollment.EnrollmentPayloadStatusURL])
	assert.Equal(t, "anna@example.test", mail.Payload[enrollment.EnrollmentPayloadRecipientEmail])
	assert.Contains(t, mail.IdempotencyKey, "enrollment-decision-digest:9:")
}

func TestNotifyDecisionsSendsImmediateMailsForTheGivenChildren(t *testing.T) {
	t.Parallel()
	mode := notifyPerDecisionImmediate
	outbox := &decisionOutboxStub{}
	notifications := newNotificationsForTest(NotificationDependencies{Outbox: outbox})
	reason := "Kapazität erschöpft"
	notice := decisionNotice(
		enrollment.DecisionChild{ID: 1, FirstName: "Lara", LastName: "B", Status: enrollment.ChildStatusRejected, StatusReason: &reason},
		enrollment.DecisionChild{ID: 2, FirstName: "Tim", LastName: "B", Status: enrollment.ChildStatusApproved},
		enrollment.DecisionChild{ID: 3, FirstName: "Mia", LastName: "B", Status: enrollment.ChildStatusUnderReview},
	)
	notice.Request.NotificationMode = &mode
	notice.Phase.ShowStatusReasonToParent = true
	notice.ImmediateChildIDs = map[int64]struct{}{1: {}, 3: {}}

	_, err := notifications.NotifyDecisions(context.Background(), notice)
	require.NoError(t, err)
	require.Len(t, outbox.mails, 1, "only parent-visible decisions of the named children are mailed")
	mail := outbox.mails[0]
	assert.Equal(t, enrollment.MailKindRejected, mail.Kind)
	assert.Equal(t, []string{"Lara B"}, mail.Payload[enrollment.EnrollmentPayloadChildNames])
	assert.Equal(t, reason, mail.Payload[enrollment.EnrollmentPayloadStatusReason])
	assert.Contains(t, mail.IdempotencyKey, "enrollment-decision:9:1:")
}

func TestNotifyDecisionsRequiresItsCollaborators(t *testing.T) {
	t.Parallel()
	immediate := notifyPerDecisionImmediate
	resolved := decisionNotice(enrollment.DecisionChild{ID: 1, Status: enrollment.ChildStatusApproved})
	resolved.Request.NotificationMode = &immediate
	resolved.ImmediateChildIDs = map[int64]struct{}{1: {}}
	_, err := newNotificationsForTest(NotificationDependencies{}).NotifyDecisions(context.Background(), resolved)
	assert.EqualError(t, err, "decision: outbox is required for immediate parent notification")

	unpinned := decisionNotice(enrollment.DecisionChild{ID: 1, Status: enrollment.ChildStatusApproved})
	_, err = newNotificationsForTest(NotificationDependencies{}).NotifyDecisions(context.Background(), unpinned)
	assert.EqualError(t, err, "decision: request repository is required to pin notification mode")

	badPin := decisionNotice(enrollment.DecisionChild{ID: 1, Status: enrollment.ChildStatusApproved})
	_, err = newNotificationsForTest(NotificationDependencies{Modes: &modePinStub{pinned: "later"}}).NotifyDecisions(context.Background(), badPin)
	assert.EqualError(t, err, `decision: unsupported pinned notification mode "later"`)

	_, err = newNotificationsForTest(NotificationDependencies{}).NotifyDecisions(context.Background(), enrollment.DecisionNotice{})
	assert.EqualError(t, err, "decision: request is required for notification mode")
}

func TestSchoolBrand(t *testing.T) {
	t.Parallel()
	school := &enrollment.School{Name: "OGS Sonnenschule", Settings: `{"loginImageUrl":"https://cdn.example.test/logo.png"}`}
	gts := notificationSettingsStub{term: "GTS"}

	brand, err := newNotificationsForTest(NotificationDependencies{Schools: schoolDirectoryStub{school: school}, Settings: gts}).SchoolBrand(context.Background(), 3, "https://parents.example.test")
	require.NoError(t, err)
	assert.Equal(t, "OGS Sonnenschule", brand.SchoolName)
	assert.Equal(t, "https://cdn.example.test/logo.png", brand.LogoURL)
	assert.Equal(t, "https://parents.example.test/images/moto-logo-mit-schriftzug.png", brand.MotoLogoURL)
	assert.Equal(t, "GTS", brand.CareFacilityTerm, "the school's own term reaches the mail header (#6042)")

	for label, deps := range map[string]NotificationDependencies{
		"no directory":   {Settings: gts},
		"lookup failure": {Schools: schoolDirectoryStub{err: errors.New("down")}, Settings: gts},
		"missing school": {Schools: schoolDirectoryStub{}, Settings: gts},
		"deleted school": {Schools: schoolDirectoryStub{school: &enrollment.School{Name: "Alt", Deleted: true}}, Settings: gts},
	} {
		brand, err := newNotificationsForTest(deps).SchoolBrand(context.Background(), 3, "https://parents.example.test")
		require.NoError(t, err, label)
		assert.Empty(t, brand.SchoolName, label)
		assert.Empty(t, brand.LogoURL, label)
		assert.Equal(t, "https://parents.example.test/images/moto-logo-mit-schriftzug.png", brand.MotoLogoURL, label)
		assert.Equal(t, "GTS", brand.CareFacilityTerm, label)
	}
	brand, err = newNotificationsForTest(NotificationDependencies{Schools: schoolDirectoryStub{school: school}, Settings: gts}).SchoolBrand(context.Background(), 0, "")
	require.NoError(t, err)
	assert.Empty(t, brand.SchoolName, "a zero tenant has no school")
	assert.Empty(t, brand.CareFacilityTerm, "a zero tenant has no setting either")
}

// An unreadable term fails the mail instead of sending another school's
// wording or a guessed default; a missing settings port is a wiring error.
func TestSchoolBrandPropagatesTheTermReadFailure(t *testing.T) {
	t.Parallel()
	school := &enrollment.School{Name: "Grundschule Barnstorf"}

	_, err := newNotificationsForTest(NotificationDependencies{Schools: schoolDirectoryStub{school: school}, Settings: notificationSettingsStub{termErr: errors.New("db down")}}).SchoolBrand(context.Background(), 3, "")
	require.ErrorContains(t, err, "db down")

	require.PanicsWithValue(t, "enrollment notifications: settings are required", func() {
		NewNotifications(NotificationDependencies{
			Schools:  schoolDirectoryStub{school: school},
			Branding: func(string, string) (string, string) { return "", "" },
		})
	})
}

// schoolLoginImageURL extracts the school's brand logo URL from the JSON
// settings blob on platform.schools.settings. It is pure (no DB) and must
// fail closed on malformed input — a broken settings JSON means "no school
// logo", not a crash inside the outbox worker.
func TestSchoolLoginImageURL(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", schoolLoginImageURL(""))
	assert.Equal(t, "", schoolLoginImageURL("   "))
	assert.Equal(t, "", schoolLoginImageURL("{not valid"))
	assert.Equal(t, "", schoolLoginImageURL(`{"otherKey":"x"}`))
	assert.Equal(t, "", schoolLoginImageURL(`{"loginImageUrl":42}`))
	assert.Equal(t, "/uploads/login-images/2_V3EjlEtM.jpg", schoolLoginImageURL(`{"loginImageUrl":"/uploads/login-images/2_V3EjlEtM.jpg"}`))
	assert.Equal(t, "/uploads/login-images/x.jpg", schoolLoginImageURL(`{"loginImageUrl":"  /uploads/login-images/x.jpg  "}`))
}
