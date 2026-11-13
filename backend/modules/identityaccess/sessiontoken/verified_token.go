package sessiontoken

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/jwtauth/v5"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess/tokenprimitives"
)

// decodedToken is the one parse of a verified token. ParseVerified stores it
// so the rate limit, the session liveness check and the authenticators
// read the same claims instead of decoding the token again.
type decodedToken struct {
	token tokenprimitives.Token
	tokenprimitives.Verified
	access AppClaims
}

type decodedTokenKey struct{}

// ParseVerified parses the token the Verifier accepted, once per request.
// Without it each reader decodes the token itself.
func ParseVerified(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := verifiedToken(r.Context())
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), decodedTokenKey{}, decode(token))))
	})
}

func decode(token tokenprimitives.Token) *decodedToken {
	verified := tokenprimitives.DecodeVerified(token, time.Now())
	return &decodedToken{token: token, Verified: verified, access: appClaimsOf(verified.Access)}
}

var errNoVerifiedToken = errors.New("no verified token")

// verifiedToken returns the token and verification error the jwtauth
// verifier, or RequireLiveSession after it, left in the context.
func verifiedToken(ctx context.Context) (tokenprimitives.Token, error) {
	if err, _ := ctx.Value(jwtauth.ErrorCtxKey).(error); err != nil {
		return nil, err
	}
	token, _ := ctx.Value(jwtauth.TokenCtxKey).(tokenprimitives.Token)
	if token == nil {
		return nil, errNoVerifiedToken
	}
	return token, nil
}

// decodedFromCtx returns the parse of the verified token in the context,
// decoding it here when ParseVerified did not run.
func decodedFromCtx(ctx context.Context) (*decodedToken, error) {
	token, err := verifiedToken(ctx)
	if err != nil {
		return nil, err
	}
	if decoded, ok := ctx.Value(decodedTokenKey{}).(*decodedToken); ok && decoded.token == token {
		return decoded, nil
	}
	return decode(token), nil
}
