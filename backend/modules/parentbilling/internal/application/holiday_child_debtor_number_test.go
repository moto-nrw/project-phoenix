package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// Counts the reads at the application's ports, independently of any
// request-level settings cache supplied by an HTTP caller.
type holidayNumberReads struct {
	HolidayStore
	parentbilling.HolidayBillingSettings
	formats, numbers int
}

func (r *holidayNumberReads) AsksDebtorNumber(context.Context, int64) (bool, error) {
	return true, nil
}

func (r *holidayNumberReads) DebtorNumberFormat(context.Context, int64) (parentbilling.DebtorNumberFormat, error) {
	r.formats++
	return parentbilling.NewDebtorNumberFormat(true, 5, 5), nil
}

func (r *holidayNumberReads) ChildDebtorNumbers(context.Context, int64) ([]parentbilling.ChildDebtorNumber, error) {
	r.numbers++
	return nil, nil
}

func TestHolidayChildNumbersReadTheFormatAndStoredNumbersOnce(t *testing.T) {
	t.Parallel()
	for _, count := range []int{1, 3} {
		reads := &holidayNumberReads{}
		h := HolidayContributions{store: reads, settings: reads}
		registration := parentbilling.HolidayRegistration{GuardianEmail: "eva.muster@example.org"}
		var inputs []parentbilling.HolidayChildDebtorNumberInput
		for i := range count {
			id := int64(71 + i)
			registration.Children = append(registration.Children, parentbilling.HolidayChild{
				ID: id, FirstName: "Kind", LastName: "Muster", BirthDate: calendar.NewDate(2018, 3, 1),
			})
			inputs = append(inputs, parentbilling.HolidayChildDebtorNumberInput{ChildID: id, DebtorNumber: "60101"})
		}
		entered, err := h.enteredChildNumbers(context.Background(), 7, registration, inputs)
		require.NoError(t, err)
		assert.Len(t, entered, count)
		assert.Equal(t, 1, reads.formats, "one format read regardless of the number of children")
		assert.Equal(t, 1, reads.numbers, "validation reuses one stored-number snapshot")
	}
}
