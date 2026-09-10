package iot

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/modules/devicescan"
)

// Runtime supplies the authentication and response envelope of device-info
// routes. The composition package mounts them under device-only authentication.
type Runtime struct {
	Authenticated func(context.Context) bool
	Success       func(http.ResponseWriter, *http.Request, int, any, string)
	Failure       func(http.ResponseWriter, *http.Request, int, error, string)
}

type Resource struct {
	configuration devicescan.ConfigurationQuery
	school        devicescan.SchoolNameQuery
	runtime       Runtime
}

func NewResource(configuration devicescan.ConfigurationQuery, school devicescan.SchoolNameQuery, runtime Runtime) *Resource {
	return &Resource{configuration: configuration, school: school, runtime: runtime}
}

func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Get("/config", rs.getDeviceConfig)
	r.Get("/school-name", rs.getSchoolName)
	return r
}

func (rs *Resource) requireDevice(w http.ResponseWriter, r *http.Request) bool {
	if rs.runtime.Authenticated(r.Context()) {
		return true
	}
	rs.runtime.Failure(w, r, http.StatusUnauthorized, errors.New(devicescan.MessageDeviceAPIKeyRequired), "")
	return false
}

type schoolNameResponse struct {
	Name string `json:"name"`
}

func (rs *Resource) getSchoolName(w http.ResponseWriter, r *http.Request) {
	if !rs.requireDevice(w, r) {
		return
	}
	name, err := rs.school.DeviceSchoolName(r.Context())
	if err != nil {
		rs.runtime.Failure(w, r, http.StatusInternalServerError, err, "")
		return
	}
	rs.runtime.Success(w, r, http.StatusOK, schoolNameResponse{Name: name}, "School name retrieved")
}

func (rs *Resource) getDeviceConfig(w http.ResponseWriter, r *http.Request) {
	if !rs.requireDevice(w, r) {
		return
	}
	response, err := rs.configuration.DeviceConfiguration(r.Context())
	if err != nil {
		rs.runtime.Failure(w, r, http.StatusInternalServerError, err, "failed to resolve device configuration")
		return
	}
	rs.runtime.Success(w, r, http.StatusOK, response, "Device configuration retrieved")
}
