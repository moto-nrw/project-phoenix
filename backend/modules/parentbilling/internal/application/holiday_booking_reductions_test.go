package application

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

func TestHolidayExportsApplyReductionsWithinEachChild(t *testing.T) {
	t.Parallel()
	part := func(child int64, kost1 string, cents int64) parentbilling.ExportBookingPart {
		p := holidayPart(kost1, "", cents, calendar.NewDate(2026, time.October, 5))
		p.ChildID = child
		if cents < 0 {
			p.WeekStartsOn, p.WeekEndsOn = nil, nil
		}
		return p
	}
	for _, c := range []struct {
		name  string
		parts []parentbilling.ExportBookingPart
		want  []int64
	}{
		{"sibling discount", []parentbilling.ExportBookingPart{part(1, "ISP", 5000), part(2, "BIR", 5000), part(2, "BIR", -2500)}, []int64{5000, 2500}},
		{"fully exempt child", []parentbilling.ExportBookingPart{part(1, "ISP", 5000), part(2, "BIR", 5000), part(2, "BIR", -5000)}, []int64{5000}},
		{"discount across the child's weeks", []parentbilling.ExportBookingPart{part(1, "ISP", 5000), part(2, "BIR", 3000), part(2, "FER", 2000), part(2, "BIR", -2500)}, []int64{5000, 1500, 1000}},
	} {
		t.Run(c.name, func(t *testing.T) {
			var total int64
			for _, p := range c.parts {
				total += p.AmountCents
			}
			for _, sign := range []int64{1, -1} {
				parts := append([]parentbilling.ExportBookingPart(nil), c.parts...)
				for i := range parts {
					parts[i].AmountCents *= sign
				}
				invoice := parentbilling.ExportInvoice{Number: "FE-1", TotalCents: sign * total, Parts: parts}
				resolver := newBookingResolver(parentbilling.BookingAssignments{}, "4400")
				batch := datevBatch{}
				batch.add(invoice, resolver)
				require.Len(t, batch.bookings, len(c.want))
				mapping := parentbilling.SollstellungMapping{Columns: []parentbilling.SollstellungColumn{{Field: parentbilling.SollFieldCostCenter}}}
				rows, _, missing := sollRows(mapping, []parentbilling.ExportInvoice{invoice}, resolver)
				require.Empty(t, missing)
				require.Len(t, rows, len(c.want))
				for i, cents := range c.want {
					assert.Equal(t, cents, batch.bookings[i].AmountCents)
					assert.Equal(t, sign*cents, rows[i].line.amountCents)
					assert.Equal(t, batch.bookings[i].CostCenter1, rows[i].line.accounts.CostCenter)
					assert.Equal(t, c.parts[i].Accounts.CostCenter, rows[i].line.accounts.CostCenter)
				}
			}
		})
	}
}

func TestHolidayFallbackProtocolOnlyNamesExportedBookings(t *testing.T) {
	t.Parallel()
	day := calendar.NewDate(2026, time.October, 5)
	fallback := holidayPart("FER", parentbilling.CostCenterFallbackNoSchoolCostCenter, 5000, day)
	fallback.ChildID = 2
	exempt := fallback
	exempt.AmountCents, exempt.WeekStartsOn, exempt.WeekEndsOn = -5000, nil, nil
	host := holidayPart("ISP", "", 5000, day)
	host.ChildID = 1
	for _, c := range []struct {
		name  string
		total int64
		parts []parentbilling.ExportBookingPart
	}{
		{"zero invoice", 0, []parentbilling.ExportBookingPart{fallback, exempt}},
		{"exempt child", 5000, []parentbilling.ExportBookingPart{host, fallback, exempt}},
		{"rounded zero group", 1, []parentbilling.ExportBookingPart{host, fallback}},
	} {
		t.Run(c.name, func(t *testing.T) {
			invoice := parentbilling.ExportInvoice{Number: "FE-1", TotalCents: c.total, Parts: c.parts}
			resolver := newBookingResolver(parentbilling.BookingAssignments{}, "4400")
			batch := datevBatch{}
			batch.add(invoice, resolver)
			assert.Empty(t, batch.fallbacks)
			mapping := parentbilling.SollstellungMapping{Columns: []parentbilling.SollstellungColumn{{Field: parentbilling.SollFieldCostCenter}}}
			rows, _, missing := sollRows(mapping, []parentbilling.ExportInvoice{invoice}, resolver)
			require.Empty(t, missing)
			assert.Empty(t, sollCostCenterFallbacks(mapping, rows))
		})
	}
}
