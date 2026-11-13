package careplanhttp_test

import (
	"github.com/go-chi/render"
	careplanhttp "github.com/moto-ogs/project-phoenix/modules/careplan/http"
)

func updateStudentTxErrorRenderer(err error) render.Renderer {
	return testResource(careplanhttp.ResourceConfig{}).UpdateStudentTxErrorRendererForTest(err)
}
