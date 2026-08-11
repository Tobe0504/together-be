package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/tobenna/together/server/internal/db"
	"github.com/tobenna/together/server/internal/models"
)

type ctxKey int

const (
	roomClaimsKey ctxKey = iota
	accountClaimsKey
)

func RequireRoomAuth(signer *Signer, store *db.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := bearerToken(r)
			if tokenStr == "" {
				http.Error(w, `{"error":"missing token"}`, http.StatusUnauthorized)
				return
			}
			claims, err := signer.VerifyRoomToken(tokenStr)
			if err != nil {
				http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}
			valid, err := store.IsSessionValid(r.Context(), claims.ID)
			if err != nil || !valid {
				http.Error(w, `{"error":"session revoked"}`, http.StatusUnauthorized)
				return
			}
			participant, err := store.GetParticipantByID(r.Context(), claims.ParticipantID)
			if err != nil {
				http.Error(w, `{"error":"session revoked"}`, http.StatusUnauthorized)
				return
			}
			if participant.Status == models.ParticipantKicked {
				http.Error(w, `{"error":"session revoked"}`, http.StatusUnauthorized)
				return
			}
			claims.Role = participant.Role
			ctx := context.WithValue(r.Context(), roomClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireRole(allowed ...models.ParticipantRole) func(http.Handler) http.Handler {
	set := make(map[models.ParticipantRole]bool, len(allowed))
	for _, r := range allowed {
		set[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := RoomClaimsFromContext(r.Context())
			if claims == nil || !set[claims.Role] {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RequireAccountAuth(signer *Signer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := bearerToken(r)
			if tokenStr == "" {
				http.Error(w, `{"error":"missing token"}`, http.StatusUnauthorized)
				return
			}
			claims, err := signer.VerifyAccountToken(tokenStr)
			if err != nil {
				http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), accountClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func OptionalAccountAuth(signer *Signer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if tokenStr := bearerToken(r); tokenStr != "" {
				if claims, err := signer.VerifyAccountToken(tokenStr); err == nil {
					r = r.WithContext(context.WithValue(r.Context(), accountClaimsKey, claims))
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RoomClaimsFromContext(ctx context.Context) *RoomClaims {
	c, _ := ctx.Value(roomClaimsKey).(*RoomClaims)
	return c
}

func AccountClaimsFromContext(ctx context.Context) *AccountClaims {
	c, _ := ctx.Value(accountClaimsKey).(*AccountClaims)
	return c
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}

	return r.URL.Query().Get("token")
}
