package application

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
)

func TestBookingLinesApplyRemovalsAndScopedFunding(t *testing.T) {
	t.Parallel()
	basisPoints := 5000
	fixed := int64(4000)
	part := func(kind, account string, cents int64, removed bool) parentbilling.ExportBookingPart {
		return parentbilling.ExportBookingPart{Kind: kind, AmountCents: cents, Removed: removed,
			Accounts: parentbilling.PositionAccounts{RevenueAccount: account, Frozen: true}}
	}
	for _, c := range []struct {
		name   string
		total  int64
		parts  []parentbilling.ExportBookingPart
		shares []parentbilling.CareInvoiceCostShare
		want   []bookingLine
	}{
		{name: "removed care position", total: 7000,
			parts: []parentbilling.ExportBookingPart{part("care", "4410", 3000, true), part("care", "4420", 7000, false)},
			want:  []bookingLine{{accounts: parentbilling.PositionAccounts{RevenueAccount: "4420"}, amountCents: 7000}}},
		{name: "removed meal position", total: 3200,
			parts: []parentbilling.ExportBookingPart{part("care", "4410", 3200, false), part("meal", "4420", 960, true)},
			want:  []bookingLine{{accounts: parentbilling.PositionAccounts{RevenueAccount: "4410"}, amountCents: 3200}}},
		{name: "removed care position retains care-only funding", total: 10000,
			parts: []parentbilling.ExportBookingPart{part("care", "4410", 10000, true),
				part("care", "4420", 10000, false), part("meal", "4430", 10000, false)},
			shares: []parentbilling.CareInvoiceCostShare{{Scope: "care", AmountCents: 10000}},
			want:   []bookingLine{{accounts: parentbilling.PositionAccounts{RevenueAccount: "4430"}, amountCents: 10000}}},
		{name: "removed care position leaves funding within surviving care positions", total: 15000,
			parts: []parentbilling.ExportBookingPart{part("care", "4410", 10000, true),
				part("care", "4420", 6000, false), part("care", "4430", 4000, false), part("meal", "4440", 10000, false)},
			shares: []parentbilling.CareInvoiceCostShare{{Scope: "care", AmountCents: 5000}},
			want: []bookingLine{{accounts: parentbilling.PositionAccounts{RevenueAccount: "4420"}, amountCents: 3000},
				{accounts: parentbilling.PositionAccounts{RevenueAccount: "4430"}, amountCents: 2000},
				{accounts: parentbilling.PositionAccounts{RevenueAccount: "4440"}, amountCents: 10000}}},
		{name: "removed meal position retains meal-only funding", total: 10000,
			parts: []parentbilling.ExportBookingPart{part("care", "4410", 10000, false),
				part("meal", "4420", 10000, true), part("meal", "4430", 10000, false)},
			shares: []parentbilling.CareInvoiceCostShare{{Scope: "meal", AmountCents: 10000}},
			want:   []bookingLine{{accounts: parentbilling.PositionAccounts{RevenueAccount: "4410"}, amountCents: 10000}}},
		{name: "removed care position retains both-scope percentage funding", total: 5000,
			parts: []parentbilling.ExportBookingPart{part("care", "4410", 10000, true),
				part("care", "4420", 10000, false), part("meal", "4430", 10000, false)},
			shares: []parentbilling.CareInvoiceCostShare{{Scope: "both", ShareBasisPoints: &basisPoints, AmountCents: 15000}},
			want:   []bookingLine{{accounts: parentbilling.PositionAccounts{RevenueAccount: "4430"}, amountCents: 5000}}},
		{name: "removed care exceeds care left after funding", total: 5000,
			parts: []parentbilling.ExportBookingPart{part("care", "4410", 10000, true),
				part("care", "4420", 10000, false), part("meal", "4430", 10000, false)},
			shares: []parentbilling.CareInvoiceCostShare{{Scope: "care", AmountCents: 15000}},
			want:   []bookingLine{{accounts: parentbilling.PositionAccounts{RevenueAccount: "4430"}, amountCents: 5000}}},
		{name: "cancellation retains funding after care removal", total: -10000,
			parts: []parentbilling.ExportBookingPart{part("care", "4410", 10000, true),
				part("care", "4420", 10000, false), part("meal", "4430", 10000, false)},
			shares: []parentbilling.CareInvoiceCostShare{{Scope: "care", AmountCents: 10000}},
			want:   []bookingLine{{accounts: parentbilling.PositionAccounts{RevenueAccount: "4430"}, amountCents: -10000}}},
		{name: "meal fully funded", total: 3200,
			parts:  []parentbilling.ExportBookingPart{part("care", "4410", 3200, false), part("meal", "4420", 960, false)},
			shares: []parentbilling.CareInvoiceCostShare{{Scope: "meal", AmountCents: 960}},
			want:   []bookingLine{{accounts: parentbilling.PositionAccounts{RevenueAccount: "4410"}, amountCents: 3200}}},
		{name: "care fully funded", total: 960,
			parts:  []parentbilling.ExportBookingPart{part("care", "4410", 3200, false), part("meal", "4420", 960, false)},
			shares: []parentbilling.CareInvoiceCostShare{{Scope: "care", AmountCents: 3200}},
			want:   []bookingLine{{accounts: parentbilling.PositionAccounts{RevenueAccount: "4420"}, amountCents: 960}}},
		{name: "both percentage", total: 2080,
			parts:  []parentbilling.ExportBookingPart{part("care", "4410", 3200, false), part("meal", "4420", 960, false)},
			shares: []parentbilling.CareInvoiceCostShare{{Scope: "both", ShareBasisPoints: &basisPoints, AmountCents: 2080}},
			want: []bookingLine{{accounts: parentbilling.PositionAccounts{RevenueAccount: "4410"}, amountCents: 1600},
				{accounts: parentbilling.PositionAccounts{RevenueAccount: "4420"}, amountCents: 480}}},
		{name: "both fixed takes care first", total: 160,
			parts:  []parentbilling.ExportBookingPart{part("care", "4410", 3200, false), part("meal", "4420", 960, false)},
			shares: []parentbilling.CareInvoiceCostShare{{Scope: "both", ShareCents: &fixed, AmountCents: 4000}},
			want:   []bookingLine{{accounts: parentbilling.PositionAccounts{RevenueAccount: "4420"}, amountCents: 160}}},
		{name: "both percentage after meal-only share", total: 1600,
			parts: []parentbilling.ExportBookingPart{part("care", "4410", 3200, false), part("meal", "4420", 960, false)},
			shares: []parentbilling.CareInvoiceCostShare{{Scope: "meal", AmountCents: 480},
				{Scope: "both", ShareBasisPoints: &basisPoints, AmountCents: 2080}},
			want: []bookingLine{{accounts: parentbilling.PositionAccounts{RevenueAccount: "4410"}, amountCents: 1600}}},
		{name: "meal rounding stays off care account", total: 3201,
			parts: []parentbilling.ExportBookingPart{part("care", "4410", 3200, false),
				part("meal", "4420", 1, false), part("meal", "4430", 1, false)},
			shares: []parentbilling.CareInvoiceCostShare{{Scope: "meal", AmountCents: 1}},
			want: []bookingLine{{accounts: parentbilling.PositionAccounts{RevenueAccount: "4410"}, amountCents: 3200},
				{accounts: parentbilling.PositionAccounts{RevenueAccount: "4420"}, amountCents: 1}}},
		{name: "care funding stays within care positions", total: 8460,
			parts: []parentbilling.ExportBookingPart{part("care", "4410", 3000, false),
				part("care", "4420", 7000, false), part("meal", "4430", 960, false)},
			shares: []parentbilling.CareInvoiceCostShare{{Scope: "care", AmountCents: 2500}},
			want: []bookingLine{{accounts: parentbilling.PositionAccounts{RevenueAccount: "4410"}, amountCents: 2250},
				{accounts: parentbilling.PositionAccounts{RevenueAccount: "4420"}, amountCents: 5250},
				{accounts: parentbilling.PositionAccounts{RevenueAccount: "4430"}, amountCents: 960}}},
		{name: "cancellation retains meal funding", total: -3200,
			parts:  []parentbilling.ExportBookingPart{part("care", "4410", 3200, false), part("meal", "4420", 960, false)},
			shares: []parentbilling.CareInvoiceCostShare{{Scope: "meal", AmountCents: 960}},
			want:   []bookingLine{{accounts: parentbilling.PositionAccounts{RevenueAccount: "4410"}, amountCents: -3200}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			invoice := parentbilling.ExportInvoice{TotalCents: c.total, Parts: c.parts, CostShares: c.shares}
			lines, _ := (bookingResolver{}).documentLines(invoice)
			assert.Equal(t, c.want, lines)
			again, _ := (bookingResolver{}).documentLines(invoice)
			assert.Equal(t, lines, again, "preview and download must not mutate their shared invoice")
		})
	}
}
