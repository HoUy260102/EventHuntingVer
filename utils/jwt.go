package utils

import (
	"EventHunting/configs"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type JwtCustomClaim struct {
	Email string
	Type  string
	Roles []string
	jwt.RegisteredClaims
}

func GenerateToken(userID, email string, roles []string, duration int, typeToken string) (string, *JwtCustomClaim, error) {
	var (
		secretKey string = configs.GetJWTSecret()
		issuer    string = configs.GetJWTIssuer()
	)
	if duration <= 0 {
		return "", nil, fmt.Errorf("duration không hợp lệ %d", duration)
	}
	tokenId, _ := uuid.NewRandom()

	claims := &JwtCustomClaim{
		Email: email,
		Type:  typeToken,
		Roles: roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        tokenId.String(),
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(duration) * time.Second)),
			Issuer:    issuer,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tok, err := token.SignedString([]byte(secretKey))
	if err != nil {
		return "", nil, err
	}
	return tok, claims, nil
}

func ExtractCustomClaims(tokenStr string) (*JwtCustomClaim, error) {
	var (
		secretKey string = configs.GetJWTSecret()
	)
	token, err := jwt.ParseWithClaims(tokenStr, &JwtCustomClaim{}, func(token *jwt.Token) (interface{}, error) {
		_, oke := token.Method.(*jwt.SigningMethodHMAC)
		if !oke {
			return nil, fmt.Errorf("JWT token đang xác thực có signing method không đúng")
		}
		return []byte(secretKey), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*JwtCustomClaim); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("Token không hợp lệ")
}

func ValidateToken(tokenString string) (*jwt.Token, error) {
	var (
		secretKey string = configs.GetJWTSecret()
	)
	token, err := jwt.Parse(tokenString, func(t_ *jwt.Token) (interface{}, error) {
		if _, ok := t_.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method %v", t_.Header["alg"])
		}
		return []byte(secretKey), nil
	})
	return token, err
}
