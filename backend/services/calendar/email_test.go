package calendar

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	calModels "github.com/moto-nrw/project-phoenix/models/calendar"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func appointmentOutboxRow(kind string) *platformModels.EmailOutbox {
	return &platformModels.EmailOutbox{
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

func TestAppointmentRenderer_PerKindCopy(t *testing.T) {
	render := NewAppointmentRenderer(EmailConfig{DefaultFrom: email.NewEmail("moto", "no-reply@example.com")})

	cases := []struct {
		kind    string
		subject string
	}{
		{platformModels.EmailKindAppointmentPublished, "Neuer Termin"},
		{platformModels.EmailKindAppointmentUpdated, "Termin geändert"},
		{platformModels.EmailKindAppointmentCancelled, "Termin abgesagt"},
		{platformModels.EmailKindAppointmentReminder, "Erinnerung"},
	}
	for _, tc := range cases {
		msg, err := render(context.Background(), appointmentOutboxRow(tc.kind))
		require.NoError(t, err, tc.kind)
		require.NotNil(t, msg)
		assert.Equal(t, "erika@example.com", msg.To.Address)
		assert.Equal(t, "appointment-notification.html", msg.Template)
		// School name prefixes the subject, then the per-kind verb.
		assert.Contains(t, msg.Subject, "OGS Musterschule")
		assert.Contains(t, msg.Subject, tc.subject)
		assert.Contains(t, msg.Subject, "Elternabend")
		content, ok := msg.Content.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "Elternabend", content["Title"])
		assert.Equal(t, "Aula", content["Location"])
		assert.Equal(t, "02.04.2026, 18:00–19:30 Uhr", content["WhenText"])
	}
}

func TestAppointmentWhenText(t *testing.T) {
	clock := func(h, m int) time.Time {
		return timezone.WallClock(time.Date(2026, 1, 1, h, m, 0, 0, time.UTC))
	}
	timed := &calModels.Appointment{
		StartDate: timezone.NewDate(2026, 4, 2),
		EndDate:   timezone.NewDate(2026, 4, 2),
		StartTime: clock(18, 0),
		EndTime:   clock(19, 30),
	}
	assert.Equal(t, "02.04.2026, 18:00–19:30 Uhr", appointmentWhenText(timed))

	// A multi-day timed appointment shows both dates so the times aren't read as
	// a single-day range.
	multiDay := &calModels.Appointment{
		StartDate: timezone.NewDate(2026, 6, 3),
		EndDate:   timezone.NewDate(2026, 6, 4),
		StartTime: clock(18, 0),
		EndTime:   clock(9, 0),
	}
	assert.Equal(t, "03.06.2026, 18:00 Uhr – 04.06.2026, 09:00 Uhr", appointmentWhenText(multiDay))

	assert.Equal(t, "02.04.2026 (ganztägig)", appointmentWhenText(&calModels.Appointment{
		StartDate: timezone.NewDate(2026, 4, 2),
		EndDate:   timezone.NewDate(2026, 4, 2),
		AllDay:    true,
	}))

	assert.Equal(t, "02.04.2026 – 04.04.2026 (ganztägig)", appointmentWhenText(&calModels.Appointment{
		StartDate: timezone.NewDate(2026, 4, 2),
		EndDate:   timezone.NewDate(2026, 4, 4),
		AllDay:    true,
	}))
}

func TestAppointmentRenderer_MissingRecipient(t *testing.T) {
	render := NewAppointmentRenderer(EmailConfig{})
	row := appointmentOutboxRow(platformModels.EmailKindAppointmentPublished)
	delete(row.Payload, apptPayloadRecipient)

	_, err := render(context.Background(), row)
	require.Error(t, err)
}

func TestAppointmentRenderer_SubjectWithoutSchool(t *testing.T) {
	render := NewAppointmentRenderer(EmailConfig{})
	row := appointmentOutboxRow(platformModels.EmailKindAppointmentPublished)
	delete(row.Payload, apptPayloadSchoolName)

	msg, err := render(context.Background(), row)
	require.NoError(t, err)
	assert.Equal(t, "Neuer Termin: Elternabend", msg.Subject)
}
