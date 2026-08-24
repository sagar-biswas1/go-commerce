package security

import (
	"testing"
	"time"

	"go-commerce/config"
	"go-commerce/domain"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func testCfg() *config.JWTConfig {
	return &config.JWTConfig{
		AccessSecret:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		RefreshSecret: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Issuer:        "go-commerce",
		Audience:      "go-commerce-api",
		AccessTTL:     15 * time.Minute,
		RefreshTTL:    time.Hour,
		Leeway:        30 * time.Second,
	}
}

func TestRoundTrip(t *testing.T) {
	i := NewJWTIssuer(testCfg())
	id := uuid.New()

	access, exp, err := i.IssueAccessToken(id, domain.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	if !exp.After(time.Now()) {
		t.Fatal("access token already expired")
	}
	got, err := i.ParseAccessToken(access)
	if err != nil {
		t.Fatal(err)
	}
	if got.UserID != id || got.Role != domain.RoleAdmin || !got.IsAdmin() {
		t.Fatalf("identity mismatch: %+v", got)
	}

	fam := uuid.New()
	refresh, _, err := i.IssueRefreshToken(id, fam)
	if err != nil {
		t.Fatal(err)
	}
	subject, err := i.ParseRefreshToken(refresh)
	if err != nil {
		t.Fatal(err)
	}
	if subject != id {
		t.Fatal("refresh subject mismatch")
	}

	// A refresh token must not verify as an access token, and vice versa.
	if _, err := i.ParseAccessToken(refresh); err == nil {
		t.Fatal("refresh token accepted as access token")
	}
	if _, err := i.ParseRefreshToken(access); err == nil {
		t.Fatal("access token accepted as refresh token")
	}

	// The family claim must name the family it was issued into.
	claims := &RefreshClaims{}
	if _, _, err := jwt.NewParser().ParseUnverified(refresh, claims); err != nil {
		t.Fatal(err)
	}
	if claims.FamilyID != fam.String() {
		t.Fatalf("family claim = %s, want %s", claims.FamilyID, fam)
	}
}

func TestRejectsForgeries(t *testing.T) {
	i := NewJWTIssuer(testCfg())
	id := uuid.New()

	// alg=none: the classic forgery a validator that trusts the header accepts.
	none := jwt.NewWithClaims(jwt.SigningMethodNone, AccessClaims{
		Kind: kindAccess, Role: domain.RoleAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: id.String(), Issuer: "go-commerce",
			Audience:  jwt.ClaimStrings{"go-commerce-api"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})
	unsigned, _ := none.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if _, err := i.ParseAccessToken(unsigned); err == nil {
		t.Fatal("alg=none token was accepted")
	}

	// A token from another issuer sharing our secret.
	other := testCfg()
	other.Issuer = "someone-else"
	foreign, _, _ := NewJWTIssuer(other).IssueAccessToken(id, domain.RoleUser)
	if _, err := i.ParseAccessToken(foreign); err == nil {
		t.Fatal("token from another issuer was accepted")
	}

	// An expired token, and the expiry-specific error on the refresh path.
	stale := testCfg()
	stale.RefreshTTL = -time.Minute
	expired, _, _ := NewJWTIssuer(stale).IssueRefreshToken(id, uuid.New())
	if _, err := i.ParseRefreshToken(expired); err != domain.ErrRefreshTokenExpired {
		t.Fatalf("expired refresh token gave %v, want ErrRefreshTokenExpired", err)
	}

	// A role the domain does not know cannot be authorized against.
	weird, _, _ := i.IssueAccessToken(id, "superuser")
	if _, err := i.ParseAccessToken(weird); err == nil {
		t.Fatal("unknown role was accepted")
	}
}
