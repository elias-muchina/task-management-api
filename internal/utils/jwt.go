package utils

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// JWT Claims structure
type Claims struct {
	UserID uuid.UUID `json:"user_id"`
	Email  string    `json:"email"`
	jwt.RegisteredClaims
}

// Global JWT secret - must be initialized
var jwtSecret []byte

// InitJWT initializes the JWT secret (call this in main.go)
func InitJWT(secret string) {
	if secret == "" {
		panic("JWT_SECRET is not set in configuration")
	}
	jwtSecret = []byte(secret)
}

// GenerateToken creates a new JWT token for a user
func GenerateToken(userID uuid.UUID, email string) (string, error) {
	if len(jwtSecret) == 0 {
		return "", fmt.Errorf("JWT secret not initialized. Call utils.InitJWT() first")
	}

	expirationTime := time.Now().Add(24 * time.Hour) // Token expires in 24 hours

	claims := &Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "task-manager-api",
			Subject:   userID.String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret) // jwtSecret is now []byte
}

// ValidateToken validates a JWT token and returns the claims
func ValidateToken(tokenString string) (*Claims, error) {
	if len(jwtSecret) == 0 {
		return nil, fmt.Errorf("JWT secret not initialized")
	}

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil // jwtSecret is []byte
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

// ExtractUserIDFromToken extracts user ID from token without full validation
// Useful for middleware that needs just the user ID
func ExtractUserIDFromToken(tokenString string) (uuid.UUID, error) {
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, &Claims{})
	if err != nil {
		return uuid.Nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok {
		return claims.UserID, nil
	}

	return uuid.Nil, fmt.Errorf("could not extract claims")
}
