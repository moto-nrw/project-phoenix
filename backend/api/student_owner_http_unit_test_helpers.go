//go:build testhooks

package api

import (
	care "github.com/moto-nrw/project-phoenix/modules/careplan/http"
	people "github.com/moto-nrw/project-phoenix/modules/peopledirectory/inbound/students"
	projection "github.com/moto-nrw/project-phoenix/modules/presenceprojection/http"
	review "github.com/moto-nrw/project-phoenix/modules/requestreview/http"
	presence "github.com/moto-nrw/project-phoenix/modules/studentpresence/http"
	delete "github.com/moto-nrw/project-phoenix/workflows/studentdeletion/http"
)

func NewCareHTTPUnitResource(cfg care.ResourceConfig) *care.Resource {
	_, subject, _, _, _, _ := NewStudentHTTPResources(people.ResourceConfig{}, cfg, projection.ResourceConfig{}, presence.ResourceConfig{}, delete.ResourceConfig{}, review.ResourceConfig{})
	return subject
}
func NewProjectionHTTPUnitResource(cfg projection.ResourceConfig) *projection.Resource {
	_, _, subject, _, _, _ := NewStudentHTTPResources(people.ResourceConfig{}, care.ResourceConfig{}, cfg, presence.ResourceConfig{}, delete.ResourceConfig{}, review.ResourceConfig{})
	return subject
}
func NewReviewHTTPUnitResource(cfg review.ResourceConfig) *review.Resource {
	_, _, _, _, _, subject := NewStudentHTTPResources(people.ResourceConfig{}, care.ResourceConfig{}, projection.ResourceConfig{}, presence.ResourceConfig{}, delete.ResourceConfig{}, cfg)
	return subject
}
