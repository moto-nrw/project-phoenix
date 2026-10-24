package presenceprojectionhttp_test

import (
	"github.com/moto-nrw/project-phoenix/api"
	subject "github.com/moto-nrw/project-phoenix/modules/presenceprojection/http"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

func init() {
	subject.ComposeStatusDays = func(repo *subject.StatusDayRepositoryStub) studentpresence.StatusDays {
		return api.NewStudentHTTPTestStatusDays(repo)
	}
}
