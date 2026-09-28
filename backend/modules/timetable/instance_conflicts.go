package timetable

import "errors"

// ErrDuplicateTemplateInstance: a create or a planned edit would give a
// template a second block on the same date and start time. The lifecycle
// wraps the storage conflict with it and keeps the storage error in the
// chain.
var ErrDuplicateTemplateInstance = errors.New("instance already exists for this template/date/start_time")
