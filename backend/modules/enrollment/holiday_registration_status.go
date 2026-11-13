package enrollment

import (
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The status link of a holiday registration (#6676): every registration
// gets a random token at its submission, and the receipt mail carries the
// link to the parents host. Whoever holds the link reads the state of that
// one registration without an account, like the status link of the online
// registration. The page shows the state, the period, and per child the
// first name and the booked weeks; it shows no health information, no
// address, no contact, no last name and no date of birth. The link ends with
// the registration: the token lives in its row and goes with it when the
// retention deletes it (ADR 0118). The token is resolved in the
// administrative transaction like the link of a period; the read runs in
// the Träger transaction of the Träger it names.

// ErrHolidayRegistrationStatusNotFound reports a status link without a
// registration: a mistyped link, or one whose registration the retention
// already deleted.
var ErrHolidayRegistrationStatusNotFound = errors.New("holiday registration status link not found")

// HolidayRegistrationStatusLink is what a status link names: the
// registration and its Träger with the Träger's schools.
type HolidayRegistrationStatusLink struct {
	Reference HolidayRegistrationReference
	Traeger   HolidayTraeger
}

// HolidayRegistrationStatusWeek is a booked week of a child: its days, the
// time model with its hours, the host school and the placement.
type HolidayRegistrationStatusWeek struct {
	FirstDay       calendar.Date
	LastDay        calendar.Date
	TimeModelName  string
	StartsAt       string
	EndsAt         string
	HostSchoolName string
	Placement      string
}

// HolidayRegistrationStatusChild is a child of the registration by its first
// name, with its weeks in the order of the period.
type HolidayRegistrationStatusChild struct {
	FirstName string
	Weeks     []HolidayRegistrationStatusWeek
}

// HolidayRegistrationStatus is what the status link shows. FirstDay and
// LastDay span the weeks of the period and are zero without weeks;
// RegistrationClosesOn is the end of the registration window, after which
// the Träger decides.
type HolidayRegistrationStatus struct {
	TraegerName          string
	PeriodName           string
	FirstDay             calendar.Date
	LastDay              calendar.Date
	RegistrationClosesOn calendar.Date
	Binding              bool
	AllocationMode       string
	SubmittedAt          time.Time
	Status               string
	Children             []HolidayRegistrationStatusChild
}

// HolidayRegistrationStatusURL is the status link of a token on the parents
// host: the page /ferienanmeldung/status/{token}. Without a parents URL or a
// token there is no link.
func HolidayRegistrationStatusURL(parentsURL, token string) string {
	base := strings.TrimRight(strings.TrimSpace(parentsURL), "/")
	token = strings.TrimSpace(token)
	if base == "" || token == "" {
		return ""
	}
	return base + "/ferienanmeldung/status/" + url.PathEscape(token)
}
