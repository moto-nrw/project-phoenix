package api

import (
	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	schoolCalendarHTTP "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/http"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/traegercalendar"
)

// mountTraegerClosingDayRoutes serves the closing days of a Träger (#4702) at
// /traeger/closing-days: reading behind "Übersicht sehen", planning behind
// "Einstellungen verwalten".
func mountTraegerClosingDayRoutes(root chi.Router, schools traegercalendar.Schools) {
	resource := schoolCalendarHTTP.NewTraegerClosingDayResource(schoolCalendarCompose.NewTraegerClosingDays(schools),
		common.RequiresPermission(identityaccess.TraegerRightOverviewRead), common.RequiresPermission(identityaccess.TraegerRightSettingsManage))
	root.Mount("/traeger/closing-days", resource.TraegerRouter(ProtectedTraegerRoutes))
}
