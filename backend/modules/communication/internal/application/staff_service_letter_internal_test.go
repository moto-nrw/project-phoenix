package application

import (
	"errors"
	"testing"
)

// letterInput builds a minimal valid Elternbrief input. Every test starts from
// this and changes exactly the field under test, so a failure names one cause.
func letterInput() Input {
	return Input{
		Title:        "Ausflug am Freitag",
		Body:         "Liebe Eltern, am Freitag fahren wir in den Zoo.",
		DeliveryMode: StaffParentAnnouncementDeliveryLetter,
		Targets: []TargetInput{
			{TargetType: StaffAnnouncementTargetSchoolAll},
		},
	}
}

// A letter must leave normalization with BOTH channels on, whatever the client
// sent. This is the core promise of #2384: publishing cannot complete with only
// one channel, and the API achieves that by forcing rather than rejecting.
func TestNormalizeDeliveryLetterForcesBothChannels(t *testing.T) {
	t.Parallel()

	cases := map[string]Input{
		"flags omitted": letterInput(),
		"flags explicitly off": func() Input {
			in := letterInput()
			in.SendEmail = false
			in.RequiresAcknowledgement = false
			return in
		}(),
		"only e-mail set": func() Input {
			in := letterInput()
			in.SendEmail = true
			return in
		}(),
		"only acknowledgement set": func() Input {
			in := letterInput()
			in.RequiresAcknowledgement = true
			return in
		}(),
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			if err := normalizeDelivery(&in); err != nil {
				t.Fatalf("normalizeDelivery: unexpected error: %v", err)
			}
			if !in.SendEmail {
				t.Error("send_email must be forced on for an Elternbrief")
			}
			if !in.RequiresAcknowledgement {
				t.Error("requires_acknowledgement must be forced on for an Elternbrief")
			}
		})
	}
}

// The safe default matters more than the happy path: an omitted audience must
// never widen the mail beyond the portal audience.
func TestNormalizeDeliveryDefaults(t *testing.T) {
	t.Parallel()

	in := Input{
		Title:   "Info",
		Body:    "Text",
		Targets: []TargetInput{{TargetType: StaffAnnouncementTargetSchoolAll}},
	}
	if err := normalizeDelivery(&in); err != nil {
		t.Fatalf("normalizeDelivery: unexpected error: %v", err)
	}
	if in.DeliveryMode != StaffParentAnnouncementDeliveryStandard {
		t.Errorf("delivery_mode = %q, want %q", in.DeliveryMode, StaffParentAnnouncementDeliveryStandard)
	}
	if in.EmailAudience != StaffEmailAudiencePortalOnly {
		t.Errorf("email_audience = %q, want %q", in.EmailAudience, StaffEmailAudiencePortalOnly)
	}
	// A plain Mitteilung must keep its opt-in semantics — normalization may not
	// turn on channels the author did not ask for.
	if in.SendEmail || in.RequiresAcknowledgement {
		t.Error("a standard Mitteilung must not have channels forced on")
	}
}

func TestNormalizeDeliveryRejectsUnknownValues(t *testing.T) {
	t.Parallel()

	t.Run("delivery_mode", func(t *testing.T) {
		in := letterInput()
		in.DeliveryMode = "brief"
		if err := normalizeDelivery(&in); !errors.Is(err, ErrValidation) {
			t.Fatalf("error = %v, want ErrValidation", err)
		}
	})
	t.Run("email_audience", func(t *testing.T) {
		in := letterInput()
		in.EmailAudience = "everyone"
		if err := normalizeDelivery(&in); !errors.Is(err, ErrValidation) {
			t.Fatalf("error = %v, want ErrValidation", err)
		}
	})
}

// A poll asks a question per child, a letter demands an acknowledgement per
// child. One row cannot carry both completion semantics, so the combination is
// refused in v1 (mirrored by chk_parent_announcements_letter_not_poll).
func TestNormalizeDeliveryRejectsLetterPoll(t *testing.T) {
	t.Parallel()

	for _, responseType := range []string{
		StaffParentAnnouncementResponseSingleChoice,
		StaffParentAnnouncementResponseMultiChoice,
	} {
		in := letterInput()
		in.ResponseType = responseType
		if err := normalizeDelivery(&in); !errors.Is(err, ErrValidation) {
			t.Errorf("response_type %q: error = %v, want ErrValidation", responseType, err)
		}
	}
	// "none" and "" are both plain-Mitteilung spellings and must pass.
	for _, responseType := range []string{"", StaffParentAnnouncementResponseNone} {
		in := letterInput()
		in.ResponseType = responseType
		if err := normalizeDelivery(&in); err != nil {
			t.Errorf("response_type %q: unexpected error: %v", responseType, err)
		}
	}
}

// Guardians reached only through an open enrollment have no student link, so
// "the letter is fulfilled for this child" is undefined for them — they would
// sit in the recipient matrix as permanently outstanding.
func TestNormalizeDeliveryRejectsLetterToPendingEnrollment(t *testing.T) {
	t.Parallel()

	in := letterInput()
	in.Targets = append(in.Targets, TargetInput{
		TargetType: StaffAnnouncementTargetPendingEnrollment,
	})
	if err := normalizeDelivery(&in); !errors.Is(err, ErrValidation) {
		t.Fatalf("error = %v, want ErrValidation", err)
	}

	// The same target stays legal for a plain Mitteilung.
	std := in
	std.DeliveryMode = StaffParentAnnouncementDeliveryStandard
	if err := normalizeDelivery(&std); err != nil {
		t.Fatalf("standard Mitteilung to pending enrollment: unexpected error: %v", err)
	}
}

// Widening the audience while sending no mail at all is a contradiction that
// almost certainly means a client lost a flag — persisting it would store a
// setting that silently does nothing.
func TestNormalizeDeliveryRejectsWideAudienceWithoutEmail(t *testing.T) {
	t.Parallel()

	in := Input{
		Title:         "Info",
		Body:          "Text",
		EmailAudience: StaffEmailAudienceAllContacts,
		SendEmail:     false,
		Targets:       []TargetInput{{TargetType: StaffAnnouncementTargetSchoolAll}},
	}
	if err := normalizeDelivery(&in); !errors.Is(err, ErrValidation) {
		t.Fatalf("error = %v, want ErrValidation", err)
	}

	// A letter forces send_email on, so the wide audience is fine there even
	// though the client never set the flag itself.
	letter := letterInput()
	letter.EmailAudience = StaffEmailAudienceAllContacts
	if err := normalizeDelivery(&letter); err != nil {
		t.Fatalf("letter with all_contacts: unexpected error: %v", err)
	}
}

func TestNormalizeDeliveryRejectsWideAudienceForStandardAnnouncement(t *testing.T) {
	t.Parallel()

	in := Input{
		Title:         "Info",
		Body:          "Text",
		DeliveryMode:  StaffParentAnnouncementDeliveryStandard,
		EmailAudience: StaffEmailAudienceAllContacts,
		SendEmail:     true,
		Targets:       []TargetInput{{TargetType: StaffAnnouncementTargetSchoolAll}},
	}
	if err := normalizeDelivery(&in); !errors.Is(err, ErrValidation) {
		t.Fatalf("error = %v, want ErrValidation", err)
	}
}

// normalizeInput is the real entry point; this pins that it runs the delivery
// rules rather than leaving them to a caller that might forget.
func TestNormalizeInputAppliesDeliveryRules(t *testing.T) {
	t.Parallel()

	in := letterInput()
	if _, err := normalizeInput(&in); err != nil {
		t.Fatalf("normalizeInput: unexpected error: %v", err)
	}
	if !in.SendEmail || !in.RequiresAcknowledgement {
		t.Error("normalizeInput must apply the Elternbrief channel rules")
	}
	if in.EmailAudience != StaffEmailAudiencePortalOnly {
		t.Errorf("email_audience = %q, want the narrow default", in.EmailAudience)
	}
}

func TestAnnouncementLetterHelpers(t *testing.T) {
	t.Parallel()

	letter := &StaffParentAnnouncement{
		DeliveryMode:  StaffParentAnnouncementDeliveryLetter,
		EmailAudience: StaffEmailAudienceAllContacts,
	}
	if !letter.IsLetter() {
		t.Error("IsLetter() = false for a letter")
	}
	if !letter.ReachesContactsWithoutPortal() {
		t.Error("ReachesContactsWithoutPortal() = false for all_contacts")
	}

	standard := &StaffParentAnnouncement{
		DeliveryMode:  StaffParentAnnouncementDeliveryStandard,
		EmailAudience: StaffEmailAudiencePortalOnly,
	}
	if standard.IsLetter() {
		t.Error("IsLetter() = true for a standard Mitteilung")
	}
	if standard.ReachesContactsWithoutPortal() {
		t.Error("ReachesContactsWithoutPortal() = true for portal_only")
	}
}
