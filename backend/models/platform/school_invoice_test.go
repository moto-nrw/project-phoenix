package platform_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validInvoice() *platform.SchoolInvoice {
	invoice := &platform.SchoolInvoice{
		PeriodLabel:   "Januar 2026",
		InvoiceNumber: "R-2026-001",
		AmountCents:   19900,
		DueDate:       timezone.NewDate(2026, time.January, 31),
		Status:        platform.InvoiceStatusOpen,
		Note:          "",
	}
	invoice.SetTenantID(42)
	return invoice
}

func TestValidInvoiceStatus(t *testing.T) {
	t.Parallel()

	assert.True(t, platform.ValidInvoiceStatus(platform.InvoiceStatusOpen))
	assert.True(t, platform.ValidInvoiceStatus(platform.InvoiceStatusPaid))
	assert.True(t, platform.ValidInvoiceStatus(platform.InvoiceStatusCancelled))

	assert.False(t, platform.ValidInvoiceStatus(""))
	assert.False(t, platform.ValidInvoiceStatus("überfällig"),
		"overdue is derived from the due date, never stored")
	assert.False(t, platform.ValidInvoiceStatus("paid"))
}

func TestSchoolInvoiceValidate_AcceptsWellFormedRow(t *testing.T) {
	t.Parallel()

	require.NoError(t, validInvoice().Validate())
}

func TestSchoolInvoiceValidate_Rejects(t *testing.T) {
	t.Parallel()

	paidOn := timezone.NewDate(2026, time.February, 2)

	tests := []struct {
		name    string
		mutate  func(*platform.SchoolInvoice)
		wantErr error
	}{
		{
			name:    "empty period label",
			mutate:  func(i *platform.SchoolInvoice) { i.PeriodLabel = "" },
			wantErr: platform.ErrInvoicePeriodLabelRequired,
		},
		{
			name:    "whitespace-only period label",
			mutate:  func(i *platform.SchoolInvoice) { i.PeriodLabel = "   " },
			wantErr: platform.ErrInvoicePeriodLabelRequired,
		},
		{
			name:    "negative amount",
			mutate:  func(i *platform.SchoolInvoice) { i.AmountCents = -1 },
			wantErr: platform.ErrInvoiceAmountNegative,
		},
		{
			name:    "missing due date",
			mutate:  func(i *platform.SchoolInvoice) { i.DueDate = timezone.Date{} },
			wantErr: platform.ErrInvoiceDueDateRequired,
		},
		{
			name:    "unknown status",
			mutate:  func(i *platform.SchoolInvoice) { i.Status = "vielleicht" },
			wantErr: platform.ErrInvoiceUnknownStatus,
		},
		{
			name: "paid without payment date",
			mutate: func(i *platform.SchoolInvoice) {
				i.Status = platform.InvoiceStatusPaid
				i.PaidOn = nil
			},
			wantErr: platform.ErrInvoicePaidOnRequired,
		},
		{
			name: "payment date on an open invoice",
			mutate: func(i *platform.SchoolInvoice) {
				i.Status = platform.InvoiceStatusOpen
				i.PaidOn = &paidOn
			},
			wantErr: platform.ErrInvoicePaidOnNotAllowed,
		},
		{
			name: "payment date on a cancelled invoice",
			mutate: func(i *platform.SchoolInvoice) {
				i.Status = platform.InvoiceStatusCancelled
				i.PaidOn = &paidOn
			},
			wantErr: platform.ErrInvoicePaidOnNotAllowed,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			invoice := validInvoice()
			tc.mutate(invoice)

			err := invoice.Validate()
			require.Error(t, err)
			assert.ErrorIs(t, err, tc.wantErr)
		})
	}
}

func TestSchoolInvoiceValidate_AcceptsPaidWithDate(t *testing.T) {
	t.Parallel()

	paidOn := timezone.NewDate(2026, time.January, 28)
	invoice := validInvoice()
	invoice.Status = platform.InvoiceStatusPaid
	invoice.PaidOn = &paidOn

	require.NoError(t, invoice.Validate())
}

func TestSchoolInvoiceValidate_RejectsOverlongNote(t *testing.T) {
	t.Parallel()

	invoice := validInvoice()
	invoice.Note = string(make([]rune, platform.MaxInvoiceNoteLen+1))

	err := invoice.Validate()
	require.Error(t, err)
	assert.ErrorIs(t, err, platform.ErrInvoiceNoteTooLong)
}

func TestSchoolInvoiceValidate_AcceptsNoteAtLimit(t *testing.T) {
	t.Parallel()

	invoice := validInvoice()
	runes := make([]rune, platform.MaxInvoiceNoteLen)
	for i := range runes {
		runes[i] = 'ä' // multi-byte: the limit counts runes, not bytes
	}
	invoice.Note = string(runes)

	require.NoError(t, invoice.Validate())
}

func TestSchoolInvoiceIsOverdueOn(t *testing.T) {
	t.Parallel()

	due := timezone.NewDate(2026, time.January, 31)
	paidOn := timezone.NewDate(2026, time.January, 20)

	tests := []struct {
		name      string
		status    string
		paidOn    *timezone.Date
		reference timezone.Date
		want      bool
	}{
		{
			name:      "open and past due",
			status:    platform.InvoiceStatusOpen,
			reference: timezone.NewDate(2026, time.February, 1),
			want:      true,
		},
		{
			name:      "open and due today is not yet overdue",
			status:    platform.InvoiceStatusOpen,
			reference: due,
			want:      false,
		},
		{
			name:      "open and due in the future",
			status:    platform.InvoiceStatusOpen,
			reference: timezone.NewDate(2026, time.January, 30),
			want:      false,
		},
		{
			name:      "paid invoices are never overdue",
			status:    platform.InvoiceStatusPaid,
			paidOn:    &paidOn,
			reference: timezone.NewDate(2027, time.January, 1),
			want:      false,
		},
		{
			name:      "cancelled invoices are never overdue",
			status:    platform.InvoiceStatusCancelled,
			reference: timezone.NewDate(2027, time.January, 1),
			want:      false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			invoice := validInvoice()
			invoice.DueDate = due
			invoice.Status = tc.status
			invoice.PaidOn = tc.paidOn

			assert.Equal(t, tc.want, invoice.IsOverdueOn(tc.reference))
		})
	}
}

// The DATE columns must be modelled as timezone.Date, not time.Time — a
// Berlin-midnight instant binds one day early through bun. TestDateColumnTypes
// enforces this repo-wide; this pins the round trip for these two fields.
func TestSchoolInvoiceDatesSerializeAsCalendarDays(t *testing.T) {
	t.Parallel()

	paidOn := timezone.NewDate(2026, time.February, 2)
	invoice := validInvoice()
	invoice.Status = platform.InvoiceStatusPaid
	invoice.PaidOn = &paidOn

	assert.Equal(t, "2026-01-31", invoice.DueDate.String())
	assert.Equal(t, "2026-02-02", invoice.PaidOn.String())
}
