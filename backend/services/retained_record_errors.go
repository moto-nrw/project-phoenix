package services

import peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"

// The adapters here keep the retained People Directory repository error
// shape their callers still classify on; the People Directory composition
// builds it.

func WrapRetainedError(op string, err error) error {
	return peopleCompose.RetainedRecordErrors{}.WrapRecordError(op, err)
}

func retainedNotFound(op string) error {
	return peopleCompose.RetainedRecordErrors{}.MissingRecordError(op)
}
