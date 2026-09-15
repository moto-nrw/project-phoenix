package ports

import "errors"

var ErrGroupNotOpen = errors.New("group is missing or already ended")
