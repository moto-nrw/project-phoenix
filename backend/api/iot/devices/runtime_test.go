package devices

import (
	"errors"
	"net/http"
	"strings"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func testRuntime() Runtime {
	return Runtime{
		ParseID: testpkg.ParseHTTPIDParam,
		Permission: func(required string) Middleware {
			return func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					claims, granted := testutil.AuthenticationContext(r.Context())
					allowed := claims.IsAdmin
					for _, permission := range granted {
						allowed = allowed || permission == required || permission == "*"
					}
					if !allowed {
						testutil.RespondError(w, r, http.StatusForbidden, errors.New("Forbidden"))
						return
					}
					next.ServeHTTP(w, r)
				})
			}
		},
		Success: testutil.RespondSuccess,
		Failure: func(w http.ResponseWriter, r *http.Request, status int, err error, message string) {
			testutil.RespondCoded(w, r, status, "", err, nil, message)
		},
		ConstraintViolation: func(err error) bool {
			return strings.Contains(err.Error(), "violates foreign key constraint") || strings.Contains(err.Error(), "violates not-null constraint")
		},
	}
}
