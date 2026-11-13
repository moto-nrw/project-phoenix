package api

import (
	owners "github.com/moto-ogs/project-phoenix/roots/owners"
)

var _ owners.ClassTeacherRepository = (*owners.ClassTeacherRepositoryBinding)(nil)

var _ owners.GroupTeacherRepository = (*owners.GroupTeacherRepositoryBinding)(nil)
