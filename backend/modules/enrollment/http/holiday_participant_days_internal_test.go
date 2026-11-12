package enrollmenthttp

import (
	"testing"

	"github.com/stretchr/testify/assert"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func TestHolidayParticipantExportIncludesBookedDates(t *testing.T) {
	t.Parallel()
	list := capability.HolidayParticipantList{
		Week: capability.HostedHolidayWeek{BookingUnit: capability.HolidayBookingUnitDay},
		Participants: []capability.HolidayParticipant{
			{FirstName: "Ben", Days: []calendar.Date{calendar.NewDate(2027, 1, 4), calendar.NewDate(2027, 1, 6)}},
		},
	}
	doc := holidayParticipantDocument(list, holidaySchoolNameMap{})
	assert.Equal(t, "04.01.2027, 06.01.2027", doc.Rows[0].Values["days"])
	found := false
	for _, column := range doc.Columns {
		if column.ID == "days" {
			found = true
			assert.Equal(t, "Tage", column.Label)
		}
	}
	assert.True(t, found, "dates need an export column in both CSV and Excel")
}
