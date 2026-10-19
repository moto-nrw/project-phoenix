package api

import (
	email "github.com/moto-nrw/project-phoenix/modules/delivery/mail"
)

// The two mails of the public demo (#3465), bound to the transactional Delivery outbox:
// the link back into the demo for the prospect and the lead mail for the team.

// IsDemoEnvironment reports whether appEnv names the public demo, the only
// environment that mounts the demo routes (#2736).
func IsDemoEnvironment(appEnv string) bool {
	return email.IsDemoEnvironment(appEnv)
}
