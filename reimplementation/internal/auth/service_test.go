package auth

import (
	"testing"
	"time"

	"github.com/research/specter-reimplementation/internal/domain"
)

func TestAccessTokenRoundTrip(t *testing.T) {
	service := New(nil, "01234567890123456789012345678901", time.Minute, time.Hour)
	tokens, err := service.access(domain.Account{ID: 7, Username: "researcher", Enabled: true, Roles: []string{"viewer"}}, "refresh")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := service.Parse(tokens.JWTToken)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != "7" || claims.Username != "researcher" || len(claims.Roles) != 1 || claims.Roles[0] != "viewer" {
		t.Fatalf("unexpected claims: %#v", claims)
	}
}

func TestRejectsWrongIssuer(t *testing.T) {
	service := New(nil, "01234567890123456789012345678901", time.Minute, time.Hour)
	if _, err := service.Parse("not-a-token"); err == nil {
		t.Fatal("invalid token was accepted")
	}
}
