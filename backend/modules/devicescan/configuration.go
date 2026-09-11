package devicescan

import "context"

type ConfigurationQuery interface {
	DeviceConfiguration(context.Context) (Configuration, error)
}

// SchoolNameQuery reads only the name of the authenticated kiosk's school.
type SchoolNameQuery interface {
	DeviceSchoolName(context.Context) (string, error)
}

// ConfigurationCheckout holds checkout button visibility settings.
type ConfigurationCheckout struct {
	RaumwechselEnabled bool    `json:"raumwechsel_enabled"`
	SchulhofEnabled    bool    `json:"schulhof_enabled"`
	WCEnabled          bool    `json:"wc_enabled"`
	DailyCheckoutTime  *string `json:"daily_checkout_time"` // "HH:MM" or null (always available)
}

// ConfigurationFeedback holds feedback settings.
type ConfigurationFeedback struct {
	Enabled bool `json:"enabled"`
}

// Configuration is the payload for GET /api/iot/config.
// PresenceMode tells the kiosk whether the tenant runs the detailed flow
// (room selection, visit tracking) or the binary flow (attendance only —
// simpler single-tap door kiosk). Old kiosk builds that don't read the field
// default to detailed behavior, so the contract is backwards-compatible.
type Configuration struct {
	Checkout     ConfigurationCheckout `json:"checkout"`
	Feedback     ConfigurationFeedback `json:"feedback"`
	PresenceMode string                `json:"presence_mode"`
}
