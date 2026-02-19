package auth

import (
	"context"
	"net/http"
)

type contextKey string

const userContextKey contextKey = "auth.user"

func ContextWithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

func UserFromContext(ctx context.Context) (*User, bool) {
	user, ok := ctx.Value(userContextKey).(*User)
	return user, ok
}

func RequireAuth(service *Service, onUnauthorized func(http.ResponseWriter, *http.Request)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(SessionCookieName)
			if err != nil {
				onUnauthorized(w, r)
				return
			}

			user, err := service.ValidateSession(cookie.Value)
			if err != nil {
				onUnauthorized(w, r)
				return
			}

			next.ServeHTTP(w, r.WithContext(ContextWithUser(r.Context(), user)))
		})
	}
}

func RequireRole(required Role, onForbidden func(http.ResponseWriter, *http.Request)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := UserFromContext(r.Context())
			if !ok || !HasRequiredRole(user.Role, required) {
				onForbidden(w, r)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
