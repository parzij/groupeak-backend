package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"groupeak/internal/services" // JWTClaims
)

type contextKey string

const (
	UserIDKey       contextKey = "userID"
	UserPositionKey contextKey = "userPosition"
)

func AuthMiddleware(jwtSecret []byte) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")

			if authHeader == "" {
				http.Error(w, "authorization header missing", http.StatusUnauthorized)
				return
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				http.Error(w, "invalid authorization header", http.StatusUnauthorized)
				return
			}

			tokenStr := parts[1]
			claims := &services.JWTClaims{}

			token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				}
				return jwtSecret, nil
			})

			if err != nil || token == nil || !token.Valid {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, UserPositionKey, claims.Position)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
