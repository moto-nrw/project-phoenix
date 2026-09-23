package presence

import (
	"context"
)

type attendancePrincipalTestKey struct{}

func testAttendancePrincipal(ctx context.Context) RequestPrincipal {
	principal, _ := ctx.Value(attendancePrincipalTestKey{}).(RequestPrincipal)
	return principal
}
