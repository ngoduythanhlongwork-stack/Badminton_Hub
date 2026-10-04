package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

type Principal struct {
	AccountID string
}

type Authenticator interface {
	Authenticate(context.Context, string) (Principal, error)
}

type AuthenticateFunc func(context.Context, string) (Principal, error)

func (f AuthenticateFunc) Authenticate(ctx context.Context, token string) (Principal, error) {
	return f(ctx, token)
}

type principalKey struct{}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalKey{}).(Principal)
	return principal, ok && principal.AccountID != ""
}

func RequireAuth(authenticator Authenticator, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authenticator == nil {
			WriteError(w, r, http.StatusServiceUnavailable, "authentication_unavailable", "Authentication is temporarily unavailable.")
			return
		}
		scheme, token, ok := strings.Cut(strings.TrimSpace(r.Header.Get("Authorization")), " ")
		if !ok || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" || strings.Contains(token, " ") {
			WriteError(w, r, http.StatusUnauthorized, "authentication_required", "A valid access token is required.")
			return
		}
		principal, err := authenticator.Authenticate(r.Context(), token)
		if err != nil || principal.AccountID == "" {
			WriteError(w, r, http.StatusUnauthorized, "invalid_access_token", "The access token is invalid or expired.")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, principal)))
	})
}

var ErrUnauthenticated = errors.New("unauthenticated")
