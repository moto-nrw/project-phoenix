package cmd

// cleanupCommandScopes assigns every cleanup subcommand to retention or to
// moto Basis (#4895). None belongs to an additional module: retention keeps
// running for a school that cancelled a module (its data stays until the
// window ends, #4812), and the Basis repairs close days, sessions and
// supervisions every school has. The moto team runs them by hand, for all
// schools alike. TestEveryCleanupCommandHasAScope fails until a new
// subcommand is assigned here.
var cleanupCommandScopes = map[string]string{
	"timetable":     "retention",
	"time-tracking": "retention",
	"visits":        "retention",
	"preview":       "retention",
	"stats":         "retention",
	"tokens":        "retention",
	"invitations":   "retention",
	"rate-limits":   "retention",
	"attendance":    "basis",
	"sessions":      "basis",
	"supervisors":   "basis",
}
