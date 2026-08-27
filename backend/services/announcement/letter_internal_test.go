package announcement

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	platformService "github.com/moto-nrw/project-phoenix/services/platform"
)

// --- doubles -----------------------------------------------------------------
//
// Deliberately separate from the fakes in service_publish_internal_test.go: that
// fakeOutbox returns an outbox row with id 0, which cannot express "this mail was
// queued and here is its id" — the exact link the delivery matrix is built on.

type letterRepo struct {
	usersModels.ParentAnnouncementRepository
	announcement *usersModels.ParentAnnouncement
	recipients   []*usersModels.AnnouncementDeliveryRecipient
	children     []*usersModels.AnnouncementLetterChildStatus
	reminders    []*usersModels.AnnouncementPollReminderRecipient
}

func (r *letterRepo) FindByID(_ context.Context, _ int64) (*usersModels.ParentAnnouncement, error) {
	return r.announcement, nil
}

func (r *letterRepo) ListTargets(_ context.Context, _ int64) ([]*usersModels.ParentAnnouncementTarget, error) {
	return nil, nil
}

func (r *letterRepo) ListOptions(_ context.Context, _ int64) ([]*usersModels.ParentAnnouncementOption, error) {
	return nil, nil
}

func (r *letterRepo) PublishIfDraft(_ context.Context, _ int64, publishedAt time.Time) (bool, error) {
	r.announcement.PublishedAt = &publishedAt
	return true, nil
}

func (r *letterRepo) SetPublished(_ context.Context, _ int64, publishedAt *time.Time) error {
	r.announcement.PublishedAt = publishedAt
	return nil
}

func (r *letterRepo) SchoolName(_ context.Context, _ int64) (string, error) {
	return "OGS Testschule", nil
}

func (r *letterRepo) AudienceRecipients(_ context.Context, _, _ int64) ([]*usersModels.AnnouncementRecipientStatus, error) {
	return nil, nil
}

func (r *letterRepo) ResolveDeliveryRecipients(_ context.Context, _, _ int64) ([]*usersModels.AnnouncementDeliveryRecipient, error) {
	return r.recipients, nil
}

// letterOutbox hands out increasing ids so a delivery row can point at a real
// outbox entry.
type letterOutbox struct {
	requests []platformService.EnqueueRequest
	nextID   int64
	cancels  int
}

func (o *letterOutbox) Enqueue(_ context.Context, req platformService.EnqueueRequest) (*platformModels.EmailOutbox, error) {
	o.requests = append(o.requests, req)
	o.nextID++
	row := &platformModels.EmailOutbox{}
	row.ID = o.nextID
	return row, nil
}

func (o *letterOutbox) CancelPendingByRelatedEntity(_ context.Context, _ string, _ int64, _ string) (int64, error) {
	o.cancels++
	return 0, nil
}

type letterDeliveries struct {
	rows     []*platformModels.EmailDelivery
	list     []*platformModels.EmailDeliveryStatus
	replaced int
	deleted  int
	attached map[int64]int64
}

func (d *letterDeliveries) ReplaceForEntity(_ context.Context, _ int64, _ string, _ int64, rows []*platformModels.EmailDelivery) error {
	d.replaced++
	d.rows = rows
	return nil
}

func (d *letterDeliveries) DeleteForEntity(_ context.Context, _ int64, _ string, _ int64) (int64, error) {
	d.deleted++
	n := int64(len(d.rows))
	d.rows = nil
	return n, nil
}

func (d *letterDeliveries) ListForEntity(_ context.Context, _ int64, _ string, _ int64) ([]*platformModels.EmailDeliveryStatus, error) {
	return d.list, nil
}

func (d *letterDeliveries) AttachOutbox(_ context.Context, _ int64, deliveryID, outboxID int64) error {
	if d.attached == nil {
		d.attached = make(map[int64]int64)
	}
	d.attached[deliveryID] = outboxID
	return nil
}

func contact(profileID int64, accountID *int64, email string, portal bool) *usersModels.AnnouncementDeliveryRecipient {
	return &usersModels.AnnouncementDeliveryRecipient{
		GuardianProfileID: profileID,
		AccountID:         accountID,
		FirstName:         "Vor",
		LastName:          "Nach",
		Email:             email,
		HasPortalAccess:   portal,
	}
}

func acct(id int64) *int64 { return &id }

func letterDraft(mode, audience string) *usersModels.ParentAnnouncement {
	a := &usersModels.ParentAnnouncement{
		Title:                   "Ausflug",
		Body:                    "Liebe Eltern, am Freitag fahren wir in den Zoo.",
		Priority:                usersModels.ParentAnnouncementPriorityInfo,
		SendEmail:               true,
		RequiresAcknowledgement: mode == usersModels.ParentAnnouncementDeliveryLetter,
		DeliveryMode:            mode,
		EmailAudience:           audience,
		Active:                  true,
	}
	a.ID = 42
	a.TenantID = 7
	return a
}

func newLetterService(repo *letterRepo, outbox *letterOutbox, deliveries *letterDeliveries) Service {
	return NewService(ServiceConfig{
		Repo:       repo,
		Settings:   &fakeSettings{enabled: true},
		Notifier:   &fakeNotifier{},
		Outbox:     outbox,
		Deliveries: deliveries,
		ParentsURL: "https://eltern.example.test",
		Logger:     slog.Default(),
	})
}

// --- tests -------------------------------------------------------------------

// The whole point of the Elternbrief: the parent can read the letter without
// opening the app, and is told where the binding confirmation happens.
func TestPublishLetterMailCarriesBodyAndAckHint(t *testing.T) {
	t.Parallel()

	repo := &letterRepo{
		announcement: letterDraft(usersModels.ParentAnnouncementDeliveryLetter, usersModels.EmailAudiencePortalOnly),
		recipients:   []*usersModels.AnnouncementDeliveryRecipient{contact(1, acct(11), "mama@example.test", true)},
	}
	outbox := &letterOutbox{}
	svc := newLetterService(repo, outbox, &letterDeliveries{})

	if _, err := svc.Publish(context.Background(), 42); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if len(outbox.requests) != 1 {
		t.Fatalf("queued %d mails, want 1", len(outbox.requests))
	}
	payload := outbox.requests[0].Payload

	if got := payload[emailPayloadBody]; got != repo.announcement.Body {
		t.Errorf("body = %q, want the announcement body", got)
	}
	if got, _ := payload[emailPayloadAckRequired].(bool); !got {
		t.Error("ack_required must be true for an Elternbrief")
	}
	if got, _ := payload[emailPayloadKicker].(string); got != letterEmailKicker {
		t.Errorf("kicker = %q, want %q", got, letterEmailKicker)
	}
	// The link must point at the letter, not at the portal root.
	url, _ := payload[emailPayloadPortalURL].(string)
	if !strings.Contains(url, "brief=42") {
		t.Errorf("portal_url = %q, want a deep link containing brief=42", url)
	}
	if key := outbox.requests[0].IdempotencyKey; key == "" || !strings.Contains(key, "mama@example.test") {
		t.Errorf("idempotency key = %q, want one scoped to the address", key)
	}
}

// A plain Mitteilung must keep the pre-#2384 behaviour exactly: no body in the
// mail, no delivery rows.
func TestPublishStandardAnnouncementStaysUntracked(t *testing.T) {
	t.Parallel()

	repo := &letterRepo{
		announcement: letterDraft(usersModels.ParentAnnouncementDeliveryStandard, usersModels.EmailAudiencePortalOnly),
		recipients:   []*usersModels.AnnouncementDeliveryRecipient{contact(1, acct(11), "mama@example.test", true)},
	}
	deliveries := &letterDeliveries{}
	// ResolveAudienceEmails is what the untracked path uses; the embedded nil
	// interface would panic if the tracked path were taken by mistake, which is
	// exactly the regression this test guards.
	svc := newLetterService(repo, &letterOutbox{}, deliveries)

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected the untracked path (which this fake does not implement)")
		}
		if deliveries.replaced != 0 {
			t.Errorf("delivery rows written %d times, want 0 for a plain Mitteilung", deliveries.replaced)
		}
	}()
	_, _ = svc.Publish(context.Background(), 42)
}

// The matrix has to show the people who get nothing — that is the data the
// school needs in order to fix it.
func TestPublishLetterRecordsUnreachableRecipients(t *testing.T) {
	t.Parallel()

	repo := &letterRepo{
		announcement: letterDraft(usersModels.ParentAnnouncementDeliveryLetter, usersModels.EmailAudiencePortalOnly),
		recipients: []*usersModels.AnnouncementDeliveryRecipient{
			contact(1, acct(11), "mama@example.test", true), // ok
			contact(2, acct(12), "", true),                  // keine Adresse
			contact(3, nil, "opa@example.test", false),      // kein Portalzugang
		},
	}
	outbox := &letterOutbox{}
	deliveries := &letterDeliveries{}
	svc := newLetterService(repo, outbox, deliveries)

	if _, err := svc.Publish(context.Background(), 42); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if len(outbox.requests) != 1 {
		t.Fatalf("queued %d mails, want 1 (only the reachable guardian)", len(outbox.requests))
	}
	if len(deliveries.rows) != 3 {
		t.Fatalf("recorded %d delivery rows, want 3 (everyone addressed)", len(deliveries.rows))
	}
	want := map[int64]string{
		1: platformModels.ReachabilityOK,
		2: platformModels.ReachabilityNoEmail,
		3: platformModels.ReachabilityNoPortal,
	}
	for _, row := range deliveries.rows {
		got := row.Reachability
		if exp := want[*row.GuardianProfileID]; got != exp {
			t.Errorf("guardian %d: reachability = %q, want %q", *row.GuardianProfileID, got, exp)
		}
		if got == platformModels.ReachabilityOK && !row.Queued() {
			t.Errorf("guardian %d: reachable but no outbox row linked", *row.GuardianProfileID)
		}
		if got != platformModels.ReachabilityOK && row.Queued() {
			t.Errorf("guardian %d: unreachable but a mail was queued", *row.GuardianProfileID)
		}
	}
}

// all_contacts is the school saying "this content may leave the portal". The
// person still cannot acknowledge, and the row must keep saying so.
func TestPublishLetterAllContactsMailsGuardiansWithoutPortal(t *testing.T) {
	t.Parallel()

	repo := &letterRepo{
		announcement: letterDraft(usersModels.ParentAnnouncementDeliveryLetter, usersModels.EmailAudienceAllContacts),
		recipients: []*usersModels.AnnouncementDeliveryRecipient{
			contact(1, acct(11), "mama@example.test", true),
			contact(3, nil, "opa@example.test", false),
		},
	}
	outbox := &letterOutbox{}
	deliveries := &letterDeliveries{}
	svc := newLetterService(repo, outbox, deliveries)

	if _, err := svc.Publish(context.Background(), 42); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if len(outbox.requests) != 2 {
		t.Fatalf("queued %d mails, want 2 (portal and non-portal guardian)", len(outbox.requests))
	}
	for _, row := range deliveries.rows {
		if !row.Queued() {
			t.Errorf("guardian %d: expected a queued mail with all_contacts", *row.GuardianProfileID)
		}
		if *row.GuardianProfileID == 3 && row.Reachability != platformModels.ReachabilityNoPortal {
			t.Errorf("guardian 3: reachability = %q, want no_portal despite the queued mail", row.Reachability)
		}
	}
}

// Two guardians of the same child often share one mailbox. One mail, two rows,
// both correctly reported as sent — sending twice to the same address would look
// like spam and break "genau ein E-Mail-Versand pro adressierter Adresse".
func TestPublishLetterSharedAddressSendsOnce(t *testing.T) {
	t.Parallel()

	repo := &letterRepo{
		announcement: letterDraft(usersModels.ParentAnnouncementDeliveryLetter, usersModels.EmailAudiencePortalOnly),
		recipients: []*usersModels.AnnouncementDeliveryRecipient{
			contact(1, acct(11), "familie@example.test", true),
			contact(2, acct(12), "  Familie@Example.test ", true),
		},
	}
	outbox := &letterOutbox{}
	deliveries := &letterDeliveries{}
	svc := newLetterService(repo, outbox, deliveries)

	if _, err := svc.Publish(context.Background(), 42); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if len(outbox.requests) != 1 {
		t.Fatalf("queued %d mails, want 1 for a shared address", len(outbox.requests))
	}
	if len(deliveries.rows) != 2 {
		t.Fatalf("recorded %d delivery rows, want 2", len(deliveries.rows))
	}
	first, second := deliveries.rows[0], deliveries.rows[1]
	if !first.Queued() || !second.Queued() {
		t.Fatal("both guardians must be linked to the queued mail")
	}
	if *first.OutboxID != *second.OutboxID {
		t.Errorf("outbox ids %d and %d differ; both rows describe the same mail",
			*first.OutboxID, *second.OutboxID)
	}
}

// Retracting must clear the matrix: a republish resolves the audience live, and
// a stale row would describe a wording parents never saw.
func TestUnpublishDropsDeliveryRows(t *testing.T) {
	t.Parallel()

	repo := &letterRepo{
		announcement: letterDraft(usersModels.ParentAnnouncementDeliveryLetter, usersModels.EmailAudiencePortalOnly),
		recipients:   []*usersModels.AnnouncementDeliveryRecipient{contact(1, acct(11), "mama@example.test", true)},
	}
	deliveries := &letterDeliveries{}
	svc := newLetterService(repo, &letterOutbox{}, deliveries)

	if _, err := svc.Publish(context.Background(), 42); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if len(deliveries.rows) == 0 {
		t.Fatal("precondition: expected delivery rows after publish")
	}
	if _, err := svc.Unpublish(context.Background(), 42); err != nil {
		t.Fatalf("Unpublish: %v", err)
	}
	if deliveries.deleted == 0 {
		t.Error("Unpublish must drop the delivery rows")
	}
	if len(deliveries.rows) != 0 {
		t.Errorf("%d delivery rows survived the retraction", len(deliveries.rows))
	}
}

// The correction path (unpublish → edit → republish) must send again, while a
// retry of the same publication must not.
func TestLetterIdempotencyKeyChangesWithPublication(t *testing.T) {
	t.Parallel()

	a := letterDraft(usersModels.ParentAnnouncementDeliveryLetter, usersModels.EmailAudiencePortalOnly)
	first := time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)
	a.PublishedAt = &first
	keyA := letterIdempotencyKey(a, "mama@example.test")
	if keyA != letterIdempotencyKey(a, "mama@example.test") {
		t.Error("same publication must produce a stable key")
	}
	if keyA == letterIdempotencyKey(a, "papa@example.test") {
		t.Error("different addresses must produce different keys")
	}
	second := first.Add(time.Nanosecond)
	a.PublishedAt = &second
	if keyA == letterIdempotencyKey(a, "mama@example.test") {
		t.Error("a republication must produce a new key so the correction is delivered")
	}
}

// --- PR 3: Statusmatrix -------------------------------------------------------

func (r *letterRepo) LetterChildStatuses(_ context.Context, _, _ int64) ([]*usersModels.AnnouncementLetterChildStatus, error) {
	return r.children, nil
}

func ackedChild(id int64, name string, at *time.Time, by string) *usersModels.AnnouncementLetterChildStatus {
	c := &usersModels.AnnouncementLetterChildStatus{
		StudentID: id, FirstName: name, LastName: "Kind", SchoolClass: "2a",
		CanConfirm:     true,
		AcknowledgedAt: at,
	}
	if at != nil {
		c.AckFirstName = by
		c.AckLastName = "Nach"
	}
	return c
}

// The summary is what a school reads first, so ChildrenOpen has to be the number
// they can act on — not "recipients who did not click".
func TestLetterStatusSummaryCountsChildrenNotRecipients(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 19, 9, 0, 0, 0, time.UTC)
	repo := &letterRepo{
		announcement: letterDraft(usersModels.ParentAnnouncementDeliveryLetter, usersModels.EmailAudiencePortalOnly),
		children: []*usersModels.AnnouncementLetterChildStatus{
			ackedChild(1, "Anna", &now, "Mama"),
			ackedChild(2, "Ben", nil, ""),
			ackedChild(3, "Cem", nil, ""),
		},
	}
	deliveries := &letterDeliveries{}
	svc := newLetterService(repo, &letterOutbox{}, deliveries)

	status, err := svc.LetterStatus(context.Background(), 42)
	if err != nil {
		t.Fatalf("LetterStatus: %v", err)
	}
	if status.Summary.ChildrenTotal != 3 {
		t.Errorf("ChildrenTotal = %d, want 3", status.Summary.ChildrenTotal)
	}
	if status.Summary.ChildrenFulfilled != 1 {
		t.Errorf("ChildrenFulfilled = %d, want 1", status.Summary.ChildrenFulfilled)
	}
	if status.Summary.ChildrenOpen != 2 {
		t.Errorf("ChildrenOpen = %d, want 2 — that is who the school follows up with",
			status.Summary.ChildrenOpen)
	}
	if !status.Children[0].Fulfilled() || status.Children[0].AckFirstName != "Mama" {
		t.Error("a fulfilled child must name who confirmed it")
	}
}

// E-mail state and moto state are different facts. A person can be counted as
// "no portal access" and still have a delivered mail — collapsing the two is the
// failure this separation exists to prevent.
func TestLetterStatusReportsBothChannelsSeparately(t *testing.T) {
	t.Parallel()

	sent := "sent"
	repo := &letterRepo{
		announcement: letterDraft(usersModels.ParentAnnouncementDeliveryLetter, usersModels.EmailAudienceAllContacts),
	}
	deliveries := &letterDeliveries{list: []*platformModels.EmailDeliveryStatus{
		{FirstName: "Mama", EmailStatus: sent, Reachability: platformModels.ReachabilityOK},
		{FirstName: "Opa", EmailStatus: sent, Reachability: platformModels.ReachabilityNoPortal},
		{FirstName: "Oma", EmailStatus: platformModels.ReachabilityNoEmail, Reachability: platformModels.ReachabilityNoEmail},
		{FirstName: "Papa", EmailStatus: "failed", Reachability: platformModels.ReachabilityOK},
	}}
	svc := newLetterService(repo, &letterOutbox{}, deliveries)

	status, err := svc.LetterStatus(context.Background(), 42)
	if err != nil {
		t.Fatalf("LetterStatus: %v", err)
	}
	if status.Summary.EmailsSent != 2 {
		t.Errorf("EmailsSent = %d, want 2", status.Summary.EmailsSent)
	}
	if status.Summary.EmailsFailed != 1 {
		t.Errorf("EmailsFailed = %d, want 1", status.Summary.EmailsFailed)
	}
	if status.Summary.WithoutPortal != 1 {
		t.Errorf("WithoutPortal = %d, want 1", status.Summary.WithoutPortal)
	}
	if status.Summary.WithoutEmail != 1 {
		t.Errorf("WithoutEmail = %d, want 1", status.Summary.WithoutEmail)
	}
	// The person without portal access still got their mail.
	if status.Recipients[1].EmailStatus != sent {
		t.Error("a guardian without portal access must still report their real e-mail status")
	}
}

func TestLetterStatusNotFound(t *testing.T) {
	t.Parallel()

	repo := &letterRepo{announcement: nil}
	svc := newLetterService(repo, &letterOutbox{}, &letterDeliveries{})
	if _, err := svc.LetterStatus(context.Background(), 42); err != ErrNotFound {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

// --- PR 4: Erinnern und Nachversenden ----------------------------------------

func (r *letterRepo) UnacknowledgedReminderRecipients(_ context.Context, _, _ int64) ([]*usersModels.AnnouncementPollReminderRecipient, error) {
	return r.reminders, nil
}

// A reminder for a letter must go out, addressed to the people who still owe a
// confirmation — and it must say what is missing, not repeat the letter.
func TestRemindOutstandingLetterMailsUnconfirmedFamilies(t *testing.T) {
	t.Parallel()

	published := time.Now().Add(-time.Hour)
	a := letterDraft(usersModels.ParentAnnouncementDeliveryLetter, usersModels.EmailAudiencePortalOnly)
	a.PublishedAt = &published
	repo := &letterRepo{
		announcement: a,
		reminders: []*usersModels.AnnouncementPollReminderRecipient{
			{AccountID: 11, Email: "mama@example.test", FirstName: "Mama", LastName: "Nach"},
		},
	}
	outbox := &letterOutbox{}
	svc := newLetterService(repo, outbox, &letterDeliveries{})

	count, err := svc.RemindOutstanding(context.Background(), 42)
	if err != nil {
		t.Fatalf("RemindOutstanding: %v", err)
	}
	if count != 1 {
		t.Errorf("reminded %d, want 1", count)
	}
	if len(outbox.requests) != 1 {
		t.Fatalf("queued %d reminder mails, want 1", len(outbox.requests))
	}
	payload := outbox.requests[0].Payload
	if got, _ := payload[emailPayloadKicker].(string); got != letterReminderKicker {
		t.Errorf("kicker = %q, want the reminder kicker", got)
	}
	// A reminder is a nudge, not a re-send of the letter.
	if body, _ := payload[emailPayloadBody].(string); body != "" {
		t.Error("a reminder must not repeat the letter body")
	}
	if intro, _ := payload[emailPayloadIntro].(string); !strings.Contains(intro, "Bestätigung") {
		t.Errorf("intro = %q, want it to name the missing confirmation", intro)
	}
}

func TestRemindOutstandingLetterDoesNotUsePollPush(t *testing.T) {
	t.Parallel()

	published := time.Now().Add(-time.Hour)
	a := letterDraft(usersModels.ParentAnnouncementDeliveryLetter, usersModels.EmailAudiencePortalOnly)
	a.PublishedAt = &published
	repo := &letterRepo{
		announcement: a,
		reminders: []*usersModels.AnnouncementPollReminderRecipient{
			{AccountID: 11, Email: "mama@example.test"},
		},
	}
	notifier := &fakeNotifier{}
	svc := NewService(ServiceConfig{
		Repo:       repo,
		Settings:   &fakeSettings{enabled: true},
		Notifier:   notifier,
		Outbox:     &letterOutbox{},
		Deliveries: &letterDeliveries{},
		ParentsURL: "https://eltern.example.test",
		Logger:     slog.Default(),
	})

	if _, err := svc.RemindOutstanding(context.Background(), a.ID); err != nil {
		t.Fatalf("RemindOutstanding: %v", err)
	}
	if len(notifier.events) != 0 {
		t.Fatalf("sent %d push event(s), want none for a letter reminder", len(notifier.events))
	}
}

func TestRemindOutstandingRefusesAcknowledgementStandardAnnouncement(t *testing.T) {
	t.Parallel()

	published := time.Now().Add(-time.Hour)
	a := letterDraft(usersModels.ParentAnnouncementDeliveryStandard, usersModels.EmailAudiencePortalOnly)
	a.RequiresAcknowledgement = true
	a.PublishedAt = &published
	svc := newLetterService(&letterRepo{announcement: a}, &letterOutbox{}, &letterDeliveries{})

	if _, err := svc.RemindOutstanding(context.Background(), a.ID); err != ErrNothingOutstanding {
		t.Fatalf("error = %v, want ErrNothingOutstanding", err)
	}
}

// Reminding about a notice that never asked for anything would be pure noise.
func TestRemindOutstandingRefusesAnnouncementWithoutAcknowledgement(t *testing.T) {
	t.Parallel()

	published := time.Now().Add(-time.Hour)
	a := letterDraft(usersModels.ParentAnnouncementDeliveryStandard, usersModels.EmailAudiencePortalOnly)
	a.RequiresAcknowledgement = false
	a.PublishedAt = &published
	svc := newLetterService(&letterRepo{announcement: a}, &letterOutbox{}, &letterDeliveries{})

	if _, err := svc.RemindOutstanding(context.Background(), 42); err != ErrNothingOutstanding {
		t.Fatalf("error = %v, want ErrNothingOutstanding", err)
	}
}

func TestRemindOutstandingRefusesUnpublished(t *testing.T) {
	t.Parallel()

	a := letterDraft(usersModels.ParentAnnouncementDeliveryLetter, usersModels.EmailAudiencePortalOnly)
	svc := newLetterService(&letterRepo{announcement: a}, &letterOutbox{}, &letterDeliveries{})

	if _, err := svc.RemindOutstanding(context.Background(), 42); err != ErrNotPublished {
		t.Fatalf("error = %v, want ErrNotPublished", err)
	}
}

// Resending must touch ONLY the failed mails. Re-sending to everyone would spam
// families whose letter arrived perfectly well.
func TestResendFailedEmailsOnlyRetriesFailures(t *testing.T) {
	t.Parallel()

	published := time.Now().Add(-time.Hour)
	a := letterDraft(usersModels.ParentAnnouncementDeliveryLetter, usersModels.EmailAudiencePortalOnly)
	a.PublishedAt = &published
	bad := "kaputt@example.test"
	good := "gut@example.test"
	deliveries := &letterDeliveries{list: []*platformModels.EmailDeliveryStatus{
		{DeliveryID: 1, FirstName: "Gut", RecipientEmail: &good, EmailStatus: "sent", Reachability: platformModels.ReachabilityOK},
		{DeliveryID: 2, FirstName: "Kaputt", RecipientEmail: &bad, EmailStatus: "failed", Reachability: platformModels.ReachabilityOK},
		{DeliveryID: 3, FirstName: "Ohne", RecipientEmail: nil, EmailStatus: platformModels.ReachabilityNoEmail, Reachability: platformModels.ReachabilityNoEmail},
	}}
	outbox := &letterOutbox{}
	svc := newLetterService(&letterRepo{announcement: a}, outbox, deliveries)

	count, err := svc.ResendFailedEmails(context.Background(), 42)
	if err != nil {
		t.Fatalf("ResendFailedEmails: %v", err)
	}
	if count != 1 {
		t.Fatalf("resent %d, want 1 (only the failed one)", count)
	}
	if got, _ := outbox.requests[0].Payload[emailPayloadRecipient].(string); got != bad {
		t.Errorf("resent to %q, want the failed address %q", got, bad)
	}
	// Without a distinct key the original publication's key would swallow the
	// retry as a duplicate and nothing would actually be sent.
	if key := outbox.requests[0].IdempotencyKey; !strings.Contains(key, "retry") {
		t.Errorf("idempotency key = %q, want a retry-scoped key", key)
	}
	if got := deliveries.attached[2]; got != outbox.nextID {
		t.Errorf("failed delivery linked to outbox %d, want %d", got, outbox.nextID)
	}
	// A recipient with no address must not be retried — nothing changed for them.
	for _, req := range outbox.requests {
		if addr, _ := req.Payload[emailPayloadRecipient].(string); addr == "" {
			t.Error("queued a mail with an empty address")
		}
	}
}

func TestResendFailedEmailsDeduplicatesSharedAddress(t *testing.T) {
	t.Parallel()

	published := time.Now().Add(-time.Hour)
	a := letterDraft(usersModels.ParentAnnouncementDeliveryLetter, usersModels.EmailAudiencePortalOnly)
	a.PublishedAt = &published
	address := "familie@example.test"
	deliveries := &letterDeliveries{list: []*platformModels.EmailDeliveryStatus{
		{DeliveryID: 1, RecipientEmail: &address, EmailStatus: "failed"},
		{DeliveryID: 2, RecipientEmail: &address, EmailStatus: "failed"},
	}}
	outbox := &letterOutbox{}
	svc := newLetterService(&letterRepo{announcement: a}, outbox, deliveries)

	count, err := svc.ResendFailedEmails(context.Background(), 42)
	if err != nil {
		t.Fatalf("ResendFailedEmails: %v", err)
	}
	if count != 1 || len(outbox.requests) != 1 {
		t.Fatalf("queued %d addresses in %d request(s), want one", count, len(outbox.requests))
	}
	if deliveries.attached[1] != deliveries.attached[2] {
		t.Errorf("shared address attached to different outbox rows: %v", deliveries.attached)
	}
}

// A child nobody can confirm for must NOT be counted as outstanding. Reminding
// changes nothing for them — the fix is portal access, not another e-mail. This
// is the split that stopped a school-wide letter to 116 children from reporting
// "6 offen" as if it were nearly done.
func TestLetterStatusSeparatesUnconfirmableChildrenFromOpenOnes(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 20, 9, 0, 0, 0, time.UTC)
	unreachable := func(id int64, name string) *usersModels.AnnouncementLetterChildStatus {
		return &usersModels.AnnouncementLetterChildStatus{
			StudentID: id, FirstName: name, LastName: "Kind", SchoolClass: "1a",
			CanConfirm: false,
		}
	}
	repo := &letterRepo{
		announcement: letterDraft(usersModels.ParentAnnouncementDeliveryLetter, usersModels.EmailAudiencePortalOnly),
		children: []*usersModels.AnnouncementLetterChildStatus{
			ackedChild(1, "Anna", &now, "Mama"), // bestätigt
			ackedChild(2, "Ben", nil, ""),       // offen, bestätigbar
			unreachable(3, "Cem"),               // kein Portalzugang
			unreachable(4, "Dora"),              // kein Portalzugang
		},
	}
	svc := newLetterService(repo, &letterOutbox{}, &letterDeliveries{})

	status, err := svc.LetterStatus(context.Background(), 42)
	if err != nil {
		t.Fatalf("LetterStatus: %v", err)
	}
	s := status.Summary
	if s.ChildrenTotal != 4 {
		t.Errorf("ChildrenTotal = %d, want 4 — every reached child stays visible", s.ChildrenTotal)
	}
	if s.ChildrenConfirmable != 2 {
		t.Errorf("ChildrenConfirmable = %d, want 2", s.ChildrenConfirmable)
	}
	if s.ChildrenFulfilled != 1 {
		t.Errorf("ChildrenFulfilled = %d, want 1", s.ChildrenFulfilled)
	}
	if s.ChildrenOpen != 1 {
		t.Errorf("ChildrenOpen = %d, want 1 — only the child a reminder can reach", s.ChildrenOpen)
	}
	if s.ChildrenWithoutPortal != 2 {
		t.Errorf("ChildrenWithoutPortal = %d, want 2", s.ChildrenWithoutPortal)
	}
	// The two gaps must never be summed into one "offen" number again.
	if s.ChildrenOpen+s.ChildrenFulfilled != s.ChildrenConfirmable {
		t.Error("open + fulfilled must account for exactly the confirmable children")
	}
}

// Outstanding is the predicate the reminder button and the "nur offene" filter
// both hang on, so it gets its own pin.
func TestLetterChildOutstanding(t *testing.T) {
	t.Parallel()

	now := time.Now()
	cases := []struct {
		name     string
		child    usersModels.AnnouncementLetterChildStatus
		wantOpen bool
	}{
		{"bestätigt", usersModels.AnnouncementLetterChildStatus{CanConfirm: true, AcknowledgedAt: &now}, false},
		{"offen", usersModels.AnnouncementLetterChildStatus{CanConfirm: true}, true},
		{"kein Portalzugang", usersModels.AnnouncementLetterChildStatus{CanConfirm: false}, false},
	}
	for _, tc := range cases {
		if got := tc.child.Outstanding(); got != tc.wantOpen {
			t.Errorf("%s: Outstanding() = %v, want %v", tc.name, got, tc.wantOpen)
		}
	}
}
