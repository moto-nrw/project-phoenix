package application

import "errors"

var errStudentRepoMissing = errors.New("student repository not configured")

func isRepositoryNotFound(err error) bool {
	var marker interface{ RepositoryNotFound() }
	return errors.As(err, &marker)
}
