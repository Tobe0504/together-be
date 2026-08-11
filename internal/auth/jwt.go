package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/tobenna/together/server/internal/models"
)

var ErrInvalidToken = errors.New("invalid or expired token")

type RoomClaims struct {
	ParticipantID string                 `json:"pid"`
	RoomID        string                 `json:"rid"`
	Role          models.ParticipantRole `json:"role"`
	UserID        *string                `json:"uid,omitempty"`
	jwt.RegisteredClaims
}

type AccountClaims struct {
	UserID string `json:"uid"`
	jwt.RegisteredClaims
}

type Signer struct {
	secret []byte
}

func NewSigner(secret string) *Signer {
	return &Signer{secret: []byte(secret)}
}

func (s *Signer) MintRoomToken(participantID, roomID string, role models.ParticipantRole, userID *string, jti string, expiresAt time.Time) (string, error) {
	claims := RoomClaims{
		ParticipantID: participantID,
		RoomID:        roomID,
		Role:          role,
		UserID:        userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
}

func (s *Signer) VerifyRoomToken(tokenStr string) (*RoomClaims, error) {
	claims := &RoomClaims{}
	tok, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		return s.secret, nil
	})
	if err != nil || !tok.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

func (s *Signer) MintAccountToken(userID string, expiresAt time.Time) (string, error) {
	claims := AccountClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
}

func (s *Signer) VerifyAccountToken(tokenStr string) (*AccountClaims, error) {
	claims := &AccountClaims{}
	tok, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		return s.secret, nil
	})
	if err != nil || !tok.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
