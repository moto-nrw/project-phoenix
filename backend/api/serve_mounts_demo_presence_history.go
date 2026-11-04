package api

import (
	"net/http"

	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	studentpresenceHTTP "github.com/moto-nrw/project-phoenix/modules/studentpresence/http"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// newDemoPresenceHistoryRoute composes Student Presence's write of past care
// days for the operator of the public demo (ADR 0127). Outside APP_ENV=demo
// it is nil, and the operator routes do not mount it.
func newDemoPresenceHistoryRoute(db transactionruntime.Database, appEnv string) http.HandlerFunc {
	if !IsDemoEnvironment(appEnv) {
		return nil
	}
	history := presenceCompose.NewPresenceHistory(presenceCompose.Dependencies{DB: db.Pool()})
	return studentpresenceHTTP.DemoPresenceHistoryHandler(history)
}
