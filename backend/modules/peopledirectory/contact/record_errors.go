package contact

import "errors"

var ErrStudentRowMissing = errors.New("student row not found")
var ErrPersonRowMissing = errors.New("person row not found")
var ErrPersonnelNumberConflict = errors.New("personnel number is already assigned")
