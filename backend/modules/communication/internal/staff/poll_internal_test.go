package announcement

import (
	"errors"
	"testing"
	"time"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
)

// Unit tests for normalizePollOptions — the validation that decides whether an
// announcement is a plain Mitteilung or a well-formed poll (#1371). Pure input
// handling, no database.

func TestNormalizePollOptions_PlainAnnouncementStaysPlain(t *testing.T) {
	t.Parallel()

	in := Input{}
	options, err := normalizePollOptions(&in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(options) != 0 {
		t.Fatalf("expected no options, got %d", len(options))
	}
	if in.ResponseType != usersModels.ParentAnnouncementResponseNone {
		t.Fatalf("expected response type to normalize to none, got %q", in.ResponseType)
	}
}

func TestNormalizePollOptions_ClearsDeadlineOnPlainAnnouncement(t *testing.T) {
	t.Parallel()

	deadline := time.Now().Add(48 * time.Hour)
	in := Input{ResponseDeadline: &deadline}
	if _, err := normalizePollOptions(&in); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if in.ResponseDeadline != nil {
		t.Fatal("expected a deadline without a response type to be cleared")
	}
}

func TestNormalizePollOptions_RejectsOptionsWithoutResponseType(t *testing.T) {
	t.Parallel()

	in := Input{Options: []string{"Ja", "Nein"}}
	if _, err := normalizePollOptions(&in); !errors.Is(err, ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestNormalizePollOptions_TrimsAndPositionsOptions(t *testing.T) {
	t.Parallel()

	in := Input{
		ResponseType: usersModels.ParentAnnouncementResponseSingleChoice,
		Options:      []string{"  Ja  ", "Nein", "   "},
	}
	options, err := normalizePollOptions(&in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(options) != 2 {
		t.Fatalf("expected the blank option to be dropped, got %d options", len(options))
	}
	if options[0].Label != "Ja" || options[0].Position != 0 {
		t.Fatalf("unexpected first option %+v", options[0])
	}
	if options[1].Label != "Nein" || options[1].Position != 1 {
		t.Fatalf("unexpected second option %+v", options[1])
	}
}

func TestNormalizePollOptions_ClearsAcknowledgementForPoll(t *testing.T) {
	t.Parallel()

	in := Input{
		ResponseType:            usersModels.ParentAnnouncementResponseSingleChoice,
		Options:                 []string{"Ja", "Nein"},
		RequiresAcknowledgement: true,
	}
	if _, err := normalizePollOptions(&in); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if in.RequiresAcknowledgement {
		t.Fatal("expected a poll response to replace acknowledgement")
	}
}

func TestNormalizePollOptions_RejectsDuplicateLabels(t *testing.T) {
	t.Parallel()

	in := Input{
		ResponseType: usersModels.ParentAnnouncementResponseSingleChoice,
		Options:      []string{"Ja", "ja"},
	}
	if _, err := normalizePollOptions(&in); !errors.Is(err, ErrValidation) {
		t.Fatalf("expected duplicate labels to be rejected, got %v", err)
	}
}

func TestNormalizePollOptions_RejectsTooFewOptions(t *testing.T) {
	t.Parallel()

	in := Input{
		ResponseType: usersModels.ParentAnnouncementResponseSingleChoice,
		Options:      []string{"Ja"},
	}
	if _, err := normalizePollOptions(&in); !errors.Is(err, ErrValidation) {
		t.Fatalf("expected a single option to be rejected, got %v", err)
	}
}

func TestNormalizePollOptions_RejectsTooManyOptions(t *testing.T) {
	t.Parallel()

	labels := make([]string, 0, maxPollOptions+1)
	for i := 0; i <= maxPollOptions; i++ {
		labels = append(labels, string(rune('A'+i)))
	}
	in := Input{
		ResponseType: usersModels.ParentAnnouncementResponseMultiChoice,
		Options:      labels,
	}
	if _, err := normalizePollOptions(&in); !errors.Is(err, ErrValidation) {
		t.Fatalf("expected more than %d options to be rejected, got %v", maxPollOptions, err)
	}
}

func TestNormalizePollOptions_RejectsPastDeadline(t *testing.T) {
	t.Parallel()

	past := time.Now().Add(-time.Hour)
	in := Input{
		ResponseType:     usersModels.ParentAnnouncementResponseSingleChoice,
		Options:          []string{"Ja", "Nein"},
		ResponseDeadline: &past,
	}
	if _, err := normalizePollOptions(&in); !errors.Is(err, ErrValidation) {
		t.Fatalf("expected a past deadline to be rejected, got %v", err)
	}
}

// A deadline after the announcement stops being visible would be unreachable:
// parents can only answer a card they can still see.
func TestNormalizePollOptions_RejectsDeadlineAfterExpiry(t *testing.T) {
	t.Parallel()

	expires := time.Now().Add(24 * time.Hour)
	deadline := expires.Add(time.Hour)
	in := Input{
		ResponseType:     usersModels.ParentAnnouncementResponseSingleChoice,
		Options:          []string{"Ja", "Nein"},
		ExpiresAt:        &expires,
		ResponseDeadline: &deadline,
	}
	if _, err := normalizePollOptions(&in); !errors.Is(err, ErrValidation) {
		t.Fatalf("expected a deadline after expiry to be rejected, got %v", err)
	}
}

func TestNormalizePollOptions_RejectsUnknownResponseType(t *testing.T) {
	t.Parallel()

	in := Input{ResponseType: "ranking", Options: []string{"Ja", "Nein"}}
	if _, err := normalizePollOptions(&in); !errors.Is(err, ErrValidation) {
		t.Fatalf("expected an unknown response type to be rejected, got %v", err)
	}
}

// AcceptsResponsesAt is what gates every answer write; a closed poll must stop
// accepting while staying readable.
func TestAcceptsResponsesAt(t *testing.T) {
	t.Parallel()

	now := time.Now()
	past := now.Add(-time.Minute)
	future := now.Add(time.Minute)

	plain := &usersModels.ParentAnnouncement{ResponseType: usersModels.ParentAnnouncementResponseNone}
	if plain.AcceptsResponsesAt(now) {
		t.Fatal("a plain announcement must not accept answers")
	}
	open := &usersModels.ParentAnnouncement{ResponseType: usersModels.ParentAnnouncementResponseSingleChoice}
	if !open.AcceptsResponsesAt(now) {
		t.Fatal("a poll without a deadline must accept answers")
	}
	stillOpen := &usersModels.ParentAnnouncement{
		ResponseType:     usersModels.ParentAnnouncementResponseSingleChoice,
		ResponseDeadline: &future,
	}
	if !stillOpen.AcceptsResponsesAt(now) {
		t.Fatal("a poll before its deadline must accept answers")
	}
	closed := &usersModels.ParentAnnouncement{
		ResponseType:     usersModels.ParentAnnouncementResponseSingleChoice,
		ResponseDeadline: &past,
	}
	if closed.AcceptsResponsesAt(now) {
		t.Fatal("a poll past its deadline must not accept answers")
	}
}

// A poll cannot target open applications: those guardians have no enrolled
// child, so there would be nothing for them to answer for.
func TestNormalizeInput_RejectsPendingEnrollmentTargetForPoll(t *testing.T) {
	t.Parallel()

	in := Input{
		Title:        "Kommt Ihr Kind?",
		Body:         "Bitte um Rückmeldung.",
		ResponseType: usersModels.ParentAnnouncementResponseSingleChoice,
		Targets: []TargetInput{
			{TargetType: usersModels.AnnouncementTargetPendingEnrollment},
		},
	}
	if _, err := normalizeInput(&in); !errors.Is(err, ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

// The same target stays valid for a plain Mitteilung, which applicants can read.
func TestNormalizeInput_AllowsPendingEnrollmentTargetForAnnouncement(t *testing.T) {
	t.Parallel()

	in := Input{
		Title: "Anmeldung läuft",
		Body:  "Die Anmeldephase endet am Freitag.",
		Targets: []TargetInput{
			{TargetType: usersModels.AnnouncementTargetPendingEnrollment},
		},
	}
	targets, err := normalizeInput(&in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("expected the target to survive, got %d", len(targets))
	}
}
