package domain

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
)

func TestQualificationStatusOnTheCalendar(t *testing.T) {
	t.Parallel()
	today := calendar.NewDate(2026, 11, 2)
	cases := []struct {
		name      string
		expiresOn string
		want      string
	}{
		{"no expiry date stays valid", "", QualificationStatusValid},
		{"expired yesterday", "2026-11-01", QualificationStatusExpired},
		{"expiry day still counts as valid until the end, inside the lead", "2026-11-02", QualificationStatusExpiringSoon},
		{"first day of the eight-week lead", "2026-12-28", QualificationStatusExpiringSoon},
		{"one day before the lead", "2026-12-29", QualificationStatusValid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, QualificationStatus(tc.expiresOn, DefaultReminderLeadWeeks, today))
		})
	}
}

func TestDueQualificationReminderFiresExactlyOnTheLeadDay(t *testing.T) {
	t.Parallel()
	q := StaffQualification{ExpiresOn: "2026-12-28"} // lead of 8 weeks starts 2026-11-02
	_, due := DueQualificationReminder(q, 8, calendar.NewDate(2026, 11, 1))
	assert.False(t, due, "no reminder the day before the lead starts")

	stage, due := DueQualificationReminder(q, 8, calendar.NewDate(2026, 11, 2))
	assert.True(t, due)
	assert.Equal(t, QualificationReminderLead, stage)

	q.LeadRemindedFor = q.ExpiresOn
	_, due = DueQualificationReminder(q, 8, calendar.NewDate(2026, 11, 3))
	assert.False(t, due, "the lead reminder goes out once per expiry date")
}

func TestDueQualificationReminderOnTheExpiryDay(t *testing.T) {
	t.Parallel()
	q := StaffQualification{ExpiresOn: "2026-11-02", LeadRemindedFor: "2026-11-02"}
	stage, due := DueQualificationReminder(q, 8, calendar.NewDate(2026, 11, 2))
	assert.True(t, due)
	assert.Equal(t, QualificationReminderExpiry, stage)

	q.ExpiryRemindedFor = q.ExpiresOn
	_, due = DueQualificationReminder(q, 8, calendar.NewDate(2026, 11, 2))
	assert.False(t, due, "no second expiry reminder")
}

func TestDueQualificationReminderCatchesUpAWeekAtMost(t *testing.T) {
	t.Parallel()
	q := StaffQualification{ExpiresOn: "2026-11-02"}
	stage, due := DueQualificationReminder(q, 8, calendar.NewDate(2026, 11, 8))
	assert.True(t, due, "a missed expiry day is caught up within the week")
	assert.Equal(t, QualificationReminderExpiry, stage)
	_, due = DueQualificationReminder(q, 8, calendar.NewDate(2026, 11, 9))
	assert.False(t, due, "a long-expired entry does not remind again")
}

func TestDueQualificationReminderRearmsForANewExpiryDate(t *testing.T) {
	t.Parallel()
	q := StaffQualification{ExpiresOn: "2028-11-02", LeadRemindedFor: "2026-11-02", ExpiryRemindedFor: "2026-11-02"}
	stage, due := DueQualificationReminder(q, 8, calendar.NewDate(2028, 9, 7))
	assert.True(t, due)
	assert.Equal(t, QualificationReminderLead, stage)
}

func TestDueQualificationReminderWithoutLeadSendsOnlyOnTheExpiryDay(t *testing.T) {
	t.Parallel()
	q := StaffQualification{ExpiresOn: "2026-11-10"}
	_, due := DueQualificationReminder(q, 0, calendar.NewDate(2026, 11, 9))
	assert.False(t, due)
	_, due = DueQualificationReminder(q, 0, calendar.NewDate(2026, 11, 10))
	assert.True(t, due)
}

func TestQualificationTypeNormalizeRejectsOutOfRange(t *testing.T) {
	t.Parallel()
	_, err := QualificationType{Name: "  "}.Normalize()
	assert.Error(t, err)
	_, err = QualificationType{Name: "Hygiene", ReminderLeadWeeks: 53}.Normalize()
	assert.Error(t, err)
	_, err = QualificationType{Name: "Hygiene", ValidityMonths: months(0)}.Normalize()
	assert.Error(t, err)
	got, err := QualificationType{Name: "  Hygiene ", ReminderLeadWeeks: 8}.Normalize()
	assert.NoError(t, err)
	assert.Equal(t, "Hygiene", got.Name)
}
