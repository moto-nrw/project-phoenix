package enrollment

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Holiday periods of a Träger (spec #4611 user stories 22 to 25, #4659): the
// Träger-Büro plans holiday care for several schools at once. A period has
// bookable weeks, each hosted by one school of the Träger, and each week has
// time models with their own price in cent. The period belongs to the
// Träger, not to a school: it is read and written in the Träger transaction
// (ADR 0086 section 5, ADR 0105). The per-school online registration stays
// the registration path; the public holiday registration (#4661) builds on
// these periods.

// Holiday period states. A draft is visible only in the Träger-Portal; a
// published period is offered for registration.
const (
	HolidayPeriodDraft     = "draft"
	HolidayPeriodPublished = "published"
)

var (
	// ErrHolidayPeriodNotFound reports a period that does not exist for the
	// Träger of the session.
	ErrHolidayPeriodNotFound = errors.New("holiday period not found")
	// ErrHolidayHostSchoolOutsideTraeger reports a host school that is not
	// an undeleted school of the Träger.
	ErrHolidayHostSchoolOutsideTraeger = errors.New("the host school is not a school of the Träger")
	// ErrHolidayPeriodIncomplete reports a publication of a period without
	// a week or with a week without a time model.
	ErrHolidayPeriodIncomplete = errors.New("a holiday period needs weeks with time models before it is published")
)

// HolidayHostSchoolRefusal names the week, by its position in the written
// period, whose host school is not an undeleted school of the Träger. It
// unwraps to ErrHolidayHostSchoolOutsideTraeger.
type HolidayHostSchoolRefusal struct {
	Week int
	Err  error
}

func (r *HolidayHostSchoolRefusal) Error() string {
	return fmt.Sprintf("week %d: %v", r.Week+1, r.Err)
}

func (r *HolidayHostSchoolRefusal) Unwrap() []error {
	return []error{r.Err, ErrHolidayHostSchoolOutsideTraeger}
}

// HolidayTimeModel is a bookable care time of one week, such as "bis
// 13 Uhr", with its price in cent. Times are wall-clock "HH:MM".
type HolidayTimeModel struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	StartsAt   string `json:"starts_at"`
	EndsAt     string `json:"ends_at"`
	PriceCents int64  `json:"price_cents"`
}

// HolidayWeek is a bookable week of a period with its host school.
type HolidayWeek struct {
	ID           int64              `json:"id"`
	StartsOn     calendar.Date      `json:"starts_on"`
	EndsOn       calendar.Date      `json:"ends_on"`
	HostSchoolID int64              `json:"host_school_id"`
	TimeModels   []HolidayTimeModel `json:"time_models"`
}

// HolidayPeriod is a holiday period of a Träger with its weeks.
type HolidayPeriod struct {
	ID                   int64         `json:"id"`
	Name                 string        `json:"name"`
	RegistrationOpensOn  calendar.Date `json:"registration_opens_on"`
	RegistrationClosesOn calendar.Date `json:"registration_closes_on"`
	// Binding states that a registration is binding once submitted.
	Binding bool `json:"binding"`
	// Notes are the hints for parents, such as lunch or a packed lunch.
	Notes       string     `json:"notes"`
	Status      string     `json:"status"`
	PublishedAt *time.Time `json:"published_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	// PublicToken is the key of the period's one public registration link
	// for all its schools (#4661, ADR 0110). Only the Träger-Portal reads it.
	PublicToken string `json:"public_token"`
	// RegistrationCount is the number of registrations received, a summed
	// figure without personal data.
	RegistrationCount int           `json:"registration_count"`
	Weeks             []HolidayWeek `json:"weeks"`
}

// HolidayTimeModelInput is a time model as the Träger-Büro writes it. A zero
// ID adds a time model; an ID of the week keeps and changes it.
type HolidayTimeModelInput struct {
	ID         int64
	Name       string
	StartsAt   string
	EndsAt     string
	PriceCents int64
}

// HolidayWeekInput is a week as the Träger-Büro writes it. A zero ID adds a
// week; an ID of the period keeps and changes it. Weeks and time models left
// out of an update are removed.
type HolidayWeekInput struct {
	ID           int64
	StartsOn     calendar.Date
	EndsOn       calendar.Date
	HostSchoolID int64
	TimeModels   []HolidayTimeModelInput
}

// HolidayPeriodInput is the whole period as the Träger-Büro writes it.
type HolidayPeriodInput struct {
	Name                 string
	RegistrationOpensOn  calendar.Date
	RegistrationClosesOn calendar.Date
	Binding              bool
	Notes                string
	Weeks                []HolidayWeekInput
}

// HolidayPeriods lets the Träger-Büro plan holiday periods. Every operation
// names the Träger of the session and runs in its Träger transaction, which
// the caller opens.
type HolidayPeriods interface {
	ListHolidayPeriods(ctx context.Context, organizationID int64) ([]HolidayPeriod, error)
	HolidayPeriod(ctx context.Context, organizationID, periodID int64) (HolidayPeriod, error)
	CreateHolidayPeriod(ctx context.Context, organizationID int64, input HolidayPeriodInput) (HolidayPeriod, error)
	UpdateHolidayPeriod(ctx context.Context, organizationID, periodID int64, input HolidayPeriodInput) (HolidayPeriod, error)
	PublishHolidayPeriod(ctx context.Context, organizationID, periodID int64) (HolidayPeriod, error)
}
