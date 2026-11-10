package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

type placeMailOutbox struct{ mails []enrollment.TraegerMail }

func (o *placeMailOutbox) EnqueueTraegerMail(_ context.Context, mail enrollment.TraegerMail) error {
	o.mails = append(o.mails, mail)
	return nil
}

// A booking moves up only when its mail can name the Träger and the host
// schools (#6437): without the Träger directory the move up fails before
// any mail is enqueued, so no composition leaves the parents an anonymous
// mail again.
func TestHolidayMoveUpNeedsTheTraegerDirectory(t *testing.T) {
	t.Parallel()
	outbox := &placeMailOutbox{}
	placement := holidayPlacement{mail: outbox}
	err := placement.announce(context.Background(), 1, enrollment.HolidayPeriod{ID: 2},
		[]enrollment.HolidayPromotion{{RegistrationID: 3, ChildID: 4, WeekID: 5, BookingID: 6}}, 7)
	assert.ErrorIs(t, err, errHolidayPlaceMailRequired)
	assert.Empty(t, outbox.mails)
}
