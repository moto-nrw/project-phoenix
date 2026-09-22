package identityaccess

import "errors"

// ErrRecordMissing marks a failure whose store found no row the call
// needed. The error keeps the store's text; callers test for this instead
// of the driver's no-rows error (#2736).
var ErrRecordMissing = errors.New("record missing")
