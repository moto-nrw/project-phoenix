package studentpresence

import (
	"errors"
)

var ErrGroupNotOpen = errors.New("group is missing or already ended")
