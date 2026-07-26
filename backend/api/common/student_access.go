// Package common - student_access.go
//
// Thin re-export shims over the student-data-access policy, which now lives in
// services/usercontext (StudentAccessContext + ResolveStudentAccess). Handlers
// in different packages (students, active, …) keep calling
// common.DetermineStudentAccess so the api layer does not own the
// gdpr.student_data_scope decision.
package common

import (
	"log/slog"
	"net/http"

	configService "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
)

// StudentAccessContext re-exports the usercontext type so existing callers'
// method calls (HasFullAccessByGroupID / HasFullAccessToStudent) keep working.
type StudentAccessContext = usercontext.StudentAccessContext

// DetermineStudentAccess resolves the access context for the caller of r by
// delegating to the usercontext policy.
func DetermineStudentAccess(
	r *http.Request,
	userContextSvc usercontext.UserContextService,
	settingsSvc configService.SettingsService,
	logger *slog.Logger,
) *StudentAccessContext {
	return usercontext.ResolveStudentAccess(r.Context(), userContextSvc, settingsSvc, logger)
}
