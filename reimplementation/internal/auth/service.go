package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/research/specter-reimplementation/internal/domain"
	"github.com/research/specter-reimplementation/internal/store"
)

type Service struct {
	store      *store.Store
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

type Claims struct {
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
	jwt.RegisteredClaims
}

type Tokens struct {
	Username     string    `json:"username"`
	Enabled      bool      `json:"enabled"`
	Roles        []string  `json:"roles"`
	JWTToken     string    `json:"jwtToken"`
	RefreshToken string    `json:"refreshToken,omitempty"`
	ExpiresAt    time.Time `json:"expiresAt"`
}

func New(store *store.Store, secret string, accessTTL, refreshTTL time.Duration) *Service {
	return &Service{store: store, secret: []byte(secret), accessTTL: accessTTL, refreshTTL: refreshTTL}
}

func (s *Service) Login(ctx context.Context, username, password, ip string) (Tokens, error) {
	account, err := s.store.Authenticate(ctx, username, password)
	if err != nil {
		return Tokens{}, err
	}
	return s.issue(ctx, account, ip, "")
}

func (s *Service) Refresh(ctx context.Context, current, ip string) (Tokens, error) {
	replacement, err := randomToken()
	if err != nil {
		return Tokens{}, err
	}
	expires := time.Now().UTC().Add(s.refreshTTL)
	account, err := s.store.RotateRefreshToken(ctx, current, replacement, ip, expires)
	if err != nil {
		return Tokens{}, err
	}
	return s.access(account, replacement)
}

func (s *Service) issue(ctx context.Context, account domain.Account, ip, refresh string) (Tokens, error) {
	var err error
	if refresh == "" {
		refresh, err = randomToken()
		if err != nil {
			return Tokens{}, err
		}
	}
	if err := s.store.SaveRefreshToken(ctx, account.ID, refresh, ip, time.Now().UTC().Add(s.refreshTTL)); err != nil {
		return Tokens{}, err
	}
	return s.access(account, refresh)
}

func (s *Service) access(account domain.Account, refresh string) (Tokens, error) {
	now := time.Now().UTC()
	expires := now.Add(s.accessTTL)
	claims := Claims{
		Username: account.Username,
		Roles:    account.Roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(account.ID, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expires),
			Issuer:    "specter-reimplementation",
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
	if err != nil {
		return Tokens{}, err
	}
	return Tokens{
		Username: account.Username, Enabled: account.Enabled, Roles: account.Roles,
		JWTToken: signed, RefreshToken: refresh, ExpiresAt: expires,
	}, nil
}

func (s *Service) Parse(raw string) (Claims, error) {
	token, err := jwt.ParseWithClaims(raw, &Claims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method %s", token.Method.Alg())
		}
		return s.secret, nil
	}, jwt.WithIssuer("specter-reimplementation"), jwt.WithExpirationRequired())
	if err != nil || !token.Valid {
		return Claims{}, errors.New("invalid token")
	}
	claims, ok := token.Claims.(*Claims)
	if !ok {
		return Claims{}, errors.New("invalid claims")
	}
	return *claims, nil
}

func randomToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
