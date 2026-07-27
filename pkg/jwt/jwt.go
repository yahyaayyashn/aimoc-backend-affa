package jwtpkg

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	UserID   uuid.UUID `json:"uid"`
	RoleCode string    `json:"role"`
	Email    string    `json:"email"`
	jwt.RegisteredClaims
}

type Service struct {
	accessSecret  []byte
	refreshSecret []byte
	accessTTL     time.Duration
	refreshTTL    time.Duration
}

func New(accessSecret, refreshSecret string, accessTTLMin, refreshTTLDay int) *Service {
	return &Service{
		accessSecret:  []byte(accessSecret),
		refreshSecret: []byte(refreshSecret),
		accessTTL:     time.Duration(accessTTLMin) * time.Minute,
		refreshTTL:    time.Duration(refreshTTLDay) * 24 * time.Hour,
	}
}

func (s *Service) GenerateAccess(userID uuid.UUID, role, email string) (string, error) {
	c := Claims{
		UserID:   userID,
		RoleCode: role,
		Email:    email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.accessTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "aimoc",
			Subject:   userID.String(),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	return t.SignedString(s.accessSecret)
}

func (s *Service) GenerateRefresh(userID uuid.UUID) (string, time.Time, error) {
	exp := time.Now().Add(s.refreshTTL)
	c := jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(exp),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		Issuer:    "aimoc",
		Subject:   userID.String(),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	str, err := t.SignedString(s.refreshSecret)
	return str, exp, err
}

func (s *Service) ParseAccess(tokenStr string) (*Claims, error) {
	c := &Claims{}
	t, err := jwt.ParseWithClaims(tokenStr, c, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}
		return s.accessSecret, nil
	})
	if err != nil || !t.Valid {
		return nil, errors.New("token tidak valid")
	}
	return c, nil
}

func (s *Service) ParseRefresh(tokenStr string) (*jwt.RegisteredClaims, error) {
	c := &jwt.RegisteredClaims{}
	t, err := jwt.ParseWithClaims(tokenStr, c, func(t *jwt.Token) (interface{}, error) {
		return s.refreshSecret, nil
	})
	if err != nil || !t.Valid {
		return nil, errors.New("refresh token tidak valid")
	}
	return c, nil
}
