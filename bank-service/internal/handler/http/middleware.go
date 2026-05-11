package http

import (
	"GoBank_PJ/bank-service/internal/config"
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func AuthMiddleware(cfg *config.Config, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			sendErrorResponse(w, http.StatusUnauthorized, "Authorization header required", nil)
			return
		}
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			sendErrorResponse(w, http.StatusUnauthorized, "Bearer token required", nil)
			return
		}
		claims := &jwt.RegisteredClaims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(cfg.JwtSecret), nil
		})
		if err != nil || !token.Valid {
			sendErrorResponse(w, http.StatusUnauthorized, "Invalid or expired token", err)
			return
		}
		if claims.ExpiresAt != nil && claims.ExpiresAt.Before(time.Now()) {
			sendErrorResponse(w, http.StatusUnauthorized, "Token expired", nil)
			return
		}
		userID, err := uuid.Parse(claims.Subject)
		if err != nil {
			sendErrorResponse(w, http.StatusUnauthorized, "Invalid user ID in token", err)
			return
		}
		ctx := context.WithValue(r.Context(), userIDContextKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
