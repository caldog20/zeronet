package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const DaysTokenValid = 30

var jwtSigningKey = []byte("SECRETKEY")

var (
	ErrTokenInvalid = errors.New("jwt token is invalid")
	ErrTokenExpired = errors.New("jwt token is expired")
	ErrParsingToken = errors.New("error parsing jwt token")
)

type Claims struct {
	jwt.RegisteredClaims
}

func getExpireDate() *jwt.NumericDate {
	// return jwt.NewNumericDate(time.Now().Add(DaysTokenValid * (24 * time.Hour)))
	return jwt.NewNumericDate(time.Now().Add(time.Second * 5))
}

func GenerateJwtWithClaims() (string, error) {
	claims := &Claims{
		jwt.RegisteredClaims{
			ExpiresAt: getExpireDate(),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "zeronet-controller",
			// Subject:   "zeronet grpc api",
			// ID:        id,
			// Audience: []string{"zeronet"},
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	ts, err := token.SignedString(jwtSigningKey)
	if err != nil {
		return "", err
	}

	return ts, nil
}

func keyFunc(token *jwt.Token) (interface{}, error) {
	if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
		return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
	}
	return jwtSigningKey, nil
}

func ParseJwtWithClaims(token string) (*Claims, error) {
	claims := &Claims{}

	t, err := jwt.ParseWithClaims(token, claims, keyFunc)
	if err != nil {
		return nil, err
	}

	if !t.Valid {
		return nil, ErrTokenInvalid
	}

	return claims, nil
}

func IsJwtExpired(token string) bool {
	t, err := jwt.ParseWithClaims(token, &Claims{}, keyFunc)
	if err != nil || !t.Valid {
		return true
	}
	return false
}
