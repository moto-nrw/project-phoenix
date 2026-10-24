package careplanhttp

import "github.com/go-chi/render"

func updateStudentTxErrorRenderer(err error) render.Renderer {
	return testResource(ResourceConfig{}).updateStudentTxErrorRenderer(err)
}
