package application

import (
	"context"
	"strings"
	"testing"
	"time"

	appointmentcap "github.com/moto-nrw/project-phoenix/modules/appointments"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func appointmentOutboxRow(kind string) *ports.EmailOutbox {
	return &ports.EmailOutbox{
		Kind: kind,
		Payload: map[string]any{
			apptPayloadRecipient:  "erika@example.com",
			apptPayloadFirstName:  "Erika",
			apptPayloadLastName:   "Mustermann",
			apptPayloadTitle:      "Elternabend",
			apptPayloadWhen:       "02.04.2026, 18:00–19:30 Uhr",
			apptPayloadLocation:   "Aula",
			apptPayloadSchoolName: "OGS Musterschule",
			apptPayloadPortalURL:  "https://parents.example.com",
		},
	}
}

// The greeting ends in a comma, so the intro has to continue that sentence AND
// name the sender itself. It used to be a fragment with a "die {Schule}: "
// prefix glued on by the template, which read "die OGS Musterschule: ein Termin
// wurde abgesagt."
func TestAppointmentRenderer_IntroIsOneSentence(t *testing.T) {
	t.Parallel()

	render := NewAppointmentRenderer(EmailConfig{})

	for _, kind := range []string{
		ports.EmailKindAppointmentPublished,
		ports.EmailKindAppointmentUpdated,
		ports.EmailKindAppointmentCancelled,
		ports.EmailKindAppointmentReminder,
	} {
		msg, err := render(context.Background(), appointmentOutboxRow(kind))
		require.NoError(t, err, kind)
		content := msg.Content
		intro, ok := content["IntroText"].(string)
		require.True(t, ok, kind)

		assert.Contains(t, intro, "OGS Musterschule", "the sender belongs inside the sentence: %s", kind)
		assert.NotContains(t, intro, ":", "a colon means the sentence was assembled from fragments: %s", kind)
		assert.True(t, strings.HasSuffix(intro, "."), "intro must be a full sentence: %s -> %q", kind, intro)
	}
}

func TestAppointmentRenderer_IntroWithoutSchoolName(t *testing.T) {
	t.Parallel()

	// Branding is best-effort; without a school name the sentence still has to
	// stand on its own rather than starting with a dangling "die ".
	render := NewAppointmentRenderer(EmailConfig{})

	for _, kind := range []string{
		ports.EmailKindAppointmentPublished,
		ports.EmailKindAppointmentUpdated,
		ports.EmailKindAppointmentCancelled,
		ports.EmailKindAppointmentReminder,
	} {
		row := appointmentOutboxRow(kind)
		delete(row.Payload, "school_name")

		msg, err := render(context.Background(), row)
		require.NoError(t, err, kind)
		content := msg.Content
		intro, ok := content["IntroText"].(string)
		require.True(t, ok)

		assert.NotContains(t, intro, "die  ", kind)
		assert.False(t, strings.HasPrefix(intro, "die "), "no school name means no dangling article: %s -> %q", kind, intro)
		assert.True(t, strings.HasSuffix(intro, "."), kind)
	}
}

func TestAppointmentRenderer_PerKindCopy(t *testing.T) {
	t.Parallel()

	render := NewAppointmentRenderer(EmailConfig{})

	cases := []struct {
		kind    string
		subject string
	}{
		{ports.EmailKindAppointmentPublished, "Neuer Termin"},
		{ports.EmailKindAppointmentUpdated, "Termin geändert"},
		{ports.EmailKindAppointmentCancelled, "Termin abgesagt"},
		{ports.EmailKindAppointmentReminder, "Erinnerung"},
	}
	for _, tc := range cases {
		msg, err := render(context.Background(), appointmentOutboxRow(tc.kind))
		require.NoError(t, err, tc.kind)
		require.NotNil(t, msg)
		assert.Equal(t, "erika@example.com", msg.To)
		assert.Equal(t, "appointment-notification.html", msg.Template)
		// School name prefixes the subject, then the per-kind verb.
		assert.Contains(t, msg.Subject, "OGS Musterschule")
		assert.Contains(t, msg.Subject, tc.subject)
		assert.Contains(t, msg.Subject, "Elternabend")
		content := msg.Content
		assert.Equal(t, "Elternabend", content["Title"])
		assert.Equal(t, "Aula", content["Location"])
		assert.Equal(t, "02.04.2026, 18:00–19:30 Uhr", content["WhenText"])
	}
}

func TestAppointmentWhenText(t *testing.T) {
	t.Parallel()

	clock := func(h, m int) time.Time {
		return normalizeWallClock(time.Date(2026, 1, 1, h, m, 0, 0, time.UTC))
	}
	timed := &appointmentcap.Appointment{
		StartDate: appointmentcap.NewDate(2026, 4, 2),
		EndDate:   appointmentcap.NewDate(2026, 4, 2),
		StartTime: clock(18, 0),
		EndTime:   clock(19, 30),
	}
	assert.Equal(t, "02.04.2026, 18:00–19:30 Uhr", appointmentWhenText(timed))

	// A multi-day timed appointment shows both dates so the times aren't read as
	// a single-day range.
	multiDay := &appointmentcap.Appointment{
		StartDate: appointmentcap.NewDate(2026, 6, 3),
		EndDate:   appointmentcap.NewDate(2026, 6, 4),
		StartTime: clock(18, 0),
		EndTime:   clock(9, 0),
	}
	assert.Equal(t, "03.06.2026, 18:00 Uhr – 04.06.2026, 09:00 Uhr", appointmentWhenText(multiDay))

	assert.Equal(t, "02.04.2026 (ganztägig)", appointmentWhenText(&appointmentcap.Appointment{
		StartDate: appointmentcap.NewDate(2026, 4, 2),
		EndDate:   appointmentcap.NewDate(2026, 4, 2),
		AllDay:    true,
	}))

	assert.Equal(t, "02.04.2026 – 04.04.2026 (ganztägig)", appointmentWhenText(&appointmentcap.Appointment{
		StartDate: appointmentcap.NewDate(2026, 4, 2),
		EndDate:   appointmentcap.NewDate(2026, 4, 4),
		AllDay:    true,
	}))
}

func TestAppointmentRenderer_MissingRecipient(t *testing.T) {
	t.Parallel()

	render := NewAppointmentRenderer(EmailConfig{})
	row := appointmentOutboxRow(ports.EmailKindAppointmentPublished)
	delete(row.Payload, apptPayloadRecipient)

	_, err := render(context.Background(), row)
	require.Error(t, err)
}

func TestAppointmentRenderer_SubjectWithoutSchool(t *testing.T) {
	t.Parallel()

	render := NewAppointmentRenderer(EmailConfig{})
	row := appointmentOutboxRow(ports.EmailKindAppointmentPublished)
	delete(row.Payload, apptPayloadSchoolName)

	msg, err := render(context.Background(), row)
	require.NoError(t, err)
	assert.Equal(t, "Neuer Termin: Elternabend", msg.Subject)
}
