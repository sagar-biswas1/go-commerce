package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go-commerce/domain"
	"go-commerce/rest/middlewares"

	"github.com/google/uuid"
)

// fakeService satisfies the Service port declared in port.go -- no issuer, no
// hasher, no database.
type fakeService struct {
	user *domain.User
	pair *domain.TokenPair
	err  error

	gotSession   *domain.SessionContext
	gotEmail     string
	gotPassword  string
	gotInput     *domain.RegisterInput
	loggedOut    string
	logoutAllFor uuid.UUID
	revoked      uuid.UUID
}

func (f *fakeService) Register(_ context.Context, input *domain.RegisterInput) (*domain.User, error) {
	f.gotInput = input
	if f.err != nil {
		return nil, f.err
	}
	return f.user, nil
}

func (f *fakeService) Login(_ context.Context, email, password string, session *domain.SessionContext) (*domain.User, *domain.TokenPair, error) {
	f.gotEmail, f.gotPassword, f.gotSession = email, password, session
	if f.err != nil {
		return nil, nil, f.err
	}
	return f.user, f.pair, nil
}

func (f *fakeService) Refresh(_ context.Context, token string, session *domain.SessionContext) (*domain.User, *domain.TokenPair, error) {
	f.loggedOut, f.gotSession = token, session
	if f.err != nil {
		return nil, nil, f.err
	}
	return f.user, f.pair, nil
}

func (f *fakeService) Logout(_ context.Context, token string) error {
	f.loggedOut = token
	return f.err
}

func (f *fakeService) LogoutEverywhere(_ context.Context, userID uuid.UUID) (int64, error) {
	f.logoutAllFor = userID
	return 3, f.err
}

func (f *fakeService) Me(context.Context, uuid.UUID) (*domain.User, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.user, nil
}

func (f *fakeService) Sessions(_ context.Context, _ uuid.UUID, page domain.Page) (*domain.PageResult[*domain.RefreshToken], error) {
	if f.err != nil {
		return nil, f.err
	}
	return &domain.PageResult[*domain.RefreshToken]{
		Items: []*domain.RefreshToken{{
			ID:        uuid.MustParse("99999999-8888-7777-6666-555555555555"),
			UserID:    f.user.ID,
			TokenHash: "a-digest-that-must-never-ship",
			ExpiresAt: time.Now().Add(time.Hour),
		}},
		Total: 1,
		Page:  page,
	}, nil
}

func (f *fakeService) RevokeSession(_ context.Context, _, sessionID uuid.UUID) error {
	f.revoked = sessionID
	return f.err
}

func newTestServer(t *testing.T, secureCookies bool) (*http.ServeMux, *fakeService) {
	t.Helper()

	svc := &fakeService{
		user: &domain.User{
			ID:        uuid.MustParse("11111111-2222-3333-4444-555555555555"),
			Email:     "sagar@example.com",
			Password:  "$2a$10$averyrealbcryptdigestthatmustnevershipoutwards",
			FirstName: "Sagar",
			Role:      domain.RoleUser,
			Status:    domain.StatusActive,
		},
		pair: &domain.TokenPair{
			AccessToken:      "an-access-token",
			AccessExpiresAt:  time.Now().Add(15 * time.Minute),
			RefreshToken:     "a-refresh-token",
			RefreshExpiresAt: time.Now().Add(24 * time.Hour),
		},
	}

	mux := http.NewServeMux()
	NewHandler(svc, nil, secureCookies).RegisterRoutes(mux)
	return mux, svc
}

func do(t *testing.T, mux *http.ServeMux, actor domain.Identity, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req = req.WithContext(middlewares.WithIdentity(req.Context(), actor))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding %q: %v", rec.Body.String(), err)
	}
	return got
}

func cookie(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// There is no role field on the request type, so a body naming one is rejected
// outright rather than quietly ignored.
func TestRegisterRefusesARoleInTheBody(t *testing.T) {
	mux, svc := newTestServer(t, true)

	rec := do(t, mux, domain.Identity{}, http.MethodPost, "/auth/register",
		`{"firstName":"New","lastName":"Person","email":"new@example.com","password":"GoodPass1","role":"admin"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for an unknown field: %s", rec.Code, rec.Body)
	}
	if svc.gotInput != nil {
		t.Error("a body naming a role reached the service")
	}
}

// Registration does not sign anybody in: no tokens are minted, so a client has
// to log in with the credentials it just chose and prove they work.
func TestRegisterMintsNoTokens(t *testing.T) {
	mux, _ := newTestServer(t, true)

	rec := do(t, mux, domain.Identity{}, http.MethodPost, "/auth/register",
		`{"firstName":"New","lastName":"Person","email":"new@example.com","password":"GoodPass1"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body)
	}
	if cookie(rec, RefreshTokenCookieName) != nil {
		t.Error("registration set a refresh cookie")
	}
	if strings.Contains(rec.Body.String(), "accessToken") {
		t.Error("registration returned an access token")
	}
}

// The refresh token travels only in the HttpOnly cookie. Putting it in the body
// as well would undo exactly what the cookie is for.
func TestLoginKeepsTheRefreshTokenOutOfTheBody(t *testing.T) {
	mux, svc := newTestServer(t, true)

	rec := do(t, mux, domain.Identity{}, http.MethodPost, "/auth/login",
		`{"email":"sagar@example.com","password":"GoodPass1"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), svc.pair.RefreshToken) {
		t.Fatalf("the refresh token is in the body: %s", rec.Body)
	}

	data := decode(t, rec)["data"].(map[string]any)
	if data["accessToken"] != svc.pair.AccessToken {
		t.Errorf("accessToken = %v", data["accessToken"])
	}
	// Seconds remaining, so a client does not have to trust its own clock
	// against ours.
	if _, ok := data["expiresIn"].(float64); !ok {
		t.Errorf("expiresIn = %v, want a number", data["expiresIn"])
	}
}

// HttpOnly is what keeps the token out of reach of JavaScript; the path scoping
// is what keeps the browser from sending it to the rest of the API at all.
func TestLoginSetsAHardenedRefreshCookie(t *testing.T) {
	mux, svc := newTestServer(t, true)

	rec := do(t, mux, domain.Identity{}, http.MethodPost, "/auth/login",
		`{"email":"sagar@example.com","password":"GoodPass1"}`)

	c := cookie(rec, RefreshTokenCookieName)
	if c == nil {
		t.Fatal("no refresh cookie was set")
	}
	if c.Value != svc.pair.RefreshToken {
		t.Errorf("cookie value = %q", c.Value)
	}
	if !c.HttpOnly {
		t.Error("the refresh cookie is readable by JavaScript")
	}
	if !c.Secure {
		t.Error("the refresh cookie is not marked Secure")
	}
	if c.SameSite != http.SameSiteStrictMode {
		t.Errorf("SameSite = %v, want Strict", c.SameSite)
	}
	// "/auth", not "/": a cookie sent on every request is a cookie exposed by
	// every request.
	if c.Path != "/auth" {
		t.Errorf("path = %q, want the cookie scoped to the routes that need it", c.Path)
	}
}

// A cookie marked Secure is not sent over plain HTTP at all, so it cannot be
// hardcoded either way without breaking development or shipping a token in the
// clear.
func TestRefreshCookieDropsSecureInDevelopment(t *testing.T) {
	mux, _ := newTestServer(t, false)

	rec := do(t, mux, domain.Identity{}, http.MethodPost, "/auth/login",
		`{"email":"sagar@example.com","password":"GoodPass1"}`)

	if c := cookie(rec, RefreshTokenCookieName); c == nil || c.Secure {
		t.Errorf("cookie = %+v, want Secure off when the deployment is not HTTPS", c)
	}
}

// A browser refreshing sends no body at all, and the cookie carries the token.
func TestRefreshReadsTheTokenFromTheCookie(t *testing.T) {
	mux, svc := newTestServer(t, true)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: RefreshTokenCookieName, Value: "the-old-token"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body)
	}
	if svc.loggedOut != "the-old-token" {
		t.Errorf("service saw %q, want the cookie's token", svc.loggedOut)
	}
	if c := cookie(rec, RefreshTokenCookieName); c == nil || c.Value != svc.pair.RefreshToken {
		t.Errorf("cookie = %+v, want it replaced with the successor", c)
	}
}

// Leaving a rejected token in the jar means every later request retries with a
// credential that cannot work.
func TestRefreshClearsTheCookieOnFailure(t *testing.T) {
	mux, svc := newTestServer(t, true)
	svc.err = domain.ErrRefreshTokenReused

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: RefreshTokenCookieName, Value: "replayed"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401: %s", rec.Code, rec.Body)
	}
	c := cookie(rec, RefreshTokenCookieName)
	if c == nil || c.MaxAge >= 0 {
		t.Errorf("cookie = %+v, want it cleared", c)
	}
}

func TestRefreshWithNoTokenAtAllIs401(t *testing.T) {
	mux, _ := newTestServer(t, true)

	rec := do(t, mux, domain.Identity{}, http.MethodPost, "/auth/refresh", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401: %s", rec.Code, rec.Body)
	}
}

// Logging out of a session that is already gone is the outcome the caller
// wanted, and saying otherwise would tell an unauthenticated caller which
// tokens exist.
func TestLogoutAlwaysSucceedsAndClearsTheCookie(t *testing.T) {
	mux, _ := newTestServer(t, true)

	rec := do(t, mux, domain.Identity{}, http.MethodPost, "/auth/logout", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body)
	}
	c := cookie(rec, RefreshTokenCookieName)
	if c == nil || c.MaxAge >= 0 {
		t.Errorf("cookie = %+v, want it cleared", c)
	}
}

// LogoutEverywhere acts on sessions other than the presenting one, so it takes
// the user id from the access token and never from the request.
func TestLogoutEverywhereUsesTheAuthenticatedIdentity(t *testing.T) {
	mux, svc := newTestServer(t, true)
	actor := domain.Identity{UserID: svc.user.ID, Role: domain.RoleUser}

	rec := do(t, mux, actor, http.MethodPost, "/auth/logout-all", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body)
	}
	if svc.logoutAllFor != actor.UserID {
		t.Errorf("revoked for %v, want the token's own user", svc.logoutAllFor)
	}
}

// The digest is what a stolen database would otherwise hand an attacker as a
// set of working sessions, so it must not leave the process.
func TestSessionsListingNeverShowsTheTokenDigest(t *testing.T) {
	mux, svc := newTestServer(t, true)
	actor := domain.Identity{UserID: svc.user.ID, Role: domain.RoleUser}

	rec := do(t, mux, actor, http.MethodGet, "/auth/sessions", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "a-digest-that-must-never-ship") {
		t.Fatalf("the token digest was serialized: %s", rec.Body)
	}

	body := decode(t, rec)
	if _, ok := body["meta"]; !ok {
		t.Error("no meta member; a client cannot page without knowing the total")
	}
	item := body["data"].([]any)[0].(map[string]any)
	links, ok := item["links"].(map[string]any)
	if !ok || links["revoke"] == nil {
		t.Errorf("item links = %v, want a revoke relation", item["links"])
	}
}

func TestRevokeSessionRepliesWith204(t *testing.T) {
	mux, svc := newTestServer(t, true)
	actor := domain.Identity{UserID: svc.user.ID, Role: domain.RoleUser}
	sessionID := uuid.New()

	rec := do(t, mux, actor, http.MethodDelete, "/auth/sessions/"+sessionID.String(), "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204: %s", rec.Code, rec.Body)
	}
	if svc.revoked != sessionID {
		t.Errorf("revoked %v, want %v", svc.revoked, sessionID)
	}
}

// Me reads the record rather than echoing the token's claims, so a role changed
// since the token was minted shows up.
func TestMeReturnsTheStoredRecord(t *testing.T) {
	mux, svc := newTestServer(t, true)
	actor := domain.Identity{UserID: svc.user.ID, Role: domain.RoleUser}

	rec := do(t, mux, actor, http.MethodGet, "/auth/me", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "$2a$") {
		t.Fatalf("the password digest was serialized: %s", rec.Body)
	}

	data := decode(t, rec)["data"].(map[string]any)
	if data["email"] != svc.user.Email {
		t.Errorf("email = %v", data["email"])
	}
}

func TestBadCredentialsBecome401(t *testing.T) {
	mux, svc := newTestServer(t, true)
	svc.err = domain.ErrInvalidCredentials

	rec := do(t, mux, domain.Identity{}, http.MethodPost, "/auth/login",
		`{"email":"sagar@example.com","password":"wrong"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401: %s", rec.Code, rec.Body)
	}
	if _, ok := decode(t, rec)["error"]; !ok {
		t.Error("a failure reply must use the error envelope")
	}
}

func TestSuspendedAccountBecomes403(t *testing.T) {
	mux, svc := newTestServer(t, true)
	svc.err = domain.ErrUserNotActive

	rec := do(t, mux, domain.Identity{}, http.MethodPost, "/auth/login",
		`{"email":"sagar@example.com","password":"GoodPass1"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403: %s", rec.Code, rec.Body)
	}
}

// Both values are attacker-controlled, so they are recorded and never trusted.
func TestLoginRecordsWhereTheSessionCameFrom(t *testing.T) {
	mux, svc := newTestServer(t, true)

	req := httptest.NewRequest(http.MethodPost, "/auth/login",
		strings.NewReader(`{"email":"sagar@example.com","password":"GoodPass1"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "curl/8.0")
	req.RemoteAddr = "203.0.113.7:54321"

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	if svc.gotSession == nil || svc.gotSession.UserAgent != "curl/8.0" {
		t.Errorf("session = %+v, want the user agent recorded", svc.gotSession)
	}
	if svc.gotSession.IPAddress != "203.0.113.7" {
		t.Errorf("ip = %q, want the peer address", svc.gotSession.IPAddress)
	}
}
