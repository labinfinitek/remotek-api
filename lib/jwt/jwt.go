package jwt

import (
	"crypto/rand"
	"log/slog"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Jwt struct {
	Key                 []byte
	TokenExpireDuration time.Duration
}

type UserClaims struct {
	UserId uint `json:"user_id"`
	jwt.RegisteredClaims
}

func NewJwt(key string, tokenExpireDuration time.Duration) *Jwt {
	return &Jwt{
		Key:                 []byte(key),
		TokenExpireDuration: tokenExpireDuration,
	}
}

func (s *Jwt) GenerateToken(userId uint) string {
	if len(s.Key) == 0 {
		slog.Error("jwt.key vuota: token non generato")
		return ""
	}
	// jti casuale: senza, due login dello stesso utente nello stesso
	// secondo davano lo stesso token, e le due righe di user_tokens (password
	// e OIDC) non si distinguevano.
	t := jwt.NewWithClaims(jwt.SigningMethodHS256,
		UserClaims{
			UserId: userId,
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.TokenExpireDuration)),
				ID:        rand.Text(),
			},
		})
	token, err := t.SignedString(s.Key)
	if err != nil {
		slog.Error("token jwt non generato", "err", err)
		return ""
	}
	return token
}

func (s *Jwt) ParseToken(tokenString string) (uint, error) {
	token, err := jwt.ParseWithClaims(tokenString, &UserClaims{}, func(_ *jwt.Token) (interface{}, error) {
		return s.Key, nil
	})
	if err != nil {
		return 0, err
	}
	if claims, ok := token.Claims.(*UserClaims); ok && token.Valid {
		return claims.UserId, nil
	}
	return 0, err
}
