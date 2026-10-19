package api

import (
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
)

var _ owners.ClassTeacherRepository = (*owners.ClassTeacherRepositoryBinding)(nil)

var _ owners.GroupTeacherRepository = (*owners.GroupTeacherRepositoryBinding)(nil)
