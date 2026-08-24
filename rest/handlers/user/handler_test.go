package user

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-commerce/domain"
	"go-commerce/rest/middlewares"

	"github.com/google/uuid"
)

// fakeService satisfies the Service port declared in port.go. Nothing here
// imports the user aggregate: the handler is tested against the shape it asked
// for, which is what that port is for.
type fakeService struct {
	user *domain.User
	err  error

	gotPatch  *domain.UserPatch
	gotInput  *domain.UserCreateInput
	gotActor  domain.Identity
	gotFilter *domain.UserFilter
	gotPage   domain.Page

	changedTo string
	deleted   bool
}

func (f *fakeService) List(_ context.Context, page domain.Page, filter *domain.UserFilter) (*domain.PageResult[*domain.User], error) {
	f.gotPage, f.gotFilter = page, filter
	if f.err != nil {
		return nil, f.err
	}
	return &domain.PageResult[*domain.User]{Items: []*domain.User{f.user}, Total: 1, Page: page}, nil
}

func (f *fakeService) Get(context.Context, uuid.UUID) (*domain.User, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.user, nil
}

func (f *fakeService) Create(_ context.Context, input *domain.UserCreateInput) (*domain.User, error) {
	f.gotInput = input
	if f.err != nil {
		return nil, f.err
	}
	return f.user, nil
}

func (f *fakeService) Update(_ context.Context, actor domain.Identity, _ uuid.UUID, patch *domain.UserPatch) (*domain.User, error) {
	f.gotActor, f.gotPatch = actor, patch
	if f.err != nil {
		return nil, f.err
	}
	return f.user, nil
}

func (f *fakeService) ChangePassword(_ context.Context, actor domain.Identity, _ uuid.UUID, _, next string) error {
	f.gotActor, f.changedTo = actor, next
	return f.err
}

func (f *fakeService) Delete(context.Context, uuid.UUID) error {
	f.deleted = true
	return f.err
}

func newTestServer(t *testing.T) (*http.ServeMux, *fakeService) {
	t.Helper()

	svc := &fakeService{user: &domain.User{
		ID:        uuid.MustParse("11111111-2222-3333-4444-555555555555"),
		Email:     "sagar@example.com",
		Password:  "$2a$10$averyrealbcryptdigestthatmustnevershipoutwards",
		FirstName: "Sagar",
		LastName:  "Biswas",
		Role:      domain.RoleUser,
		Status:    domain.StatusActive,
	}}

	mux := http.NewServeMux()
	NewHandler(svc, nil).RegisterRoutes(mux)
	return mux, svc
}

// do sends a request as actor. The nil middleware pipeline means RequireAuth
// does not run, so the identity is put in the context directly -- these tests
// are about the handlers, not about the gate in front of them.
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

func admin() domain.Identity {
	return domain.Identity{UserID: uuid.New(), Role: domain.RoleAdmin}
}

// The `json:"-"` tag on Password is the only thing between a bcrypt digest and
// every handler that marshals a user. This is the test that keeps it there.
func TestAUserReplyNeverCarriesThePasswordDigest(t *testing.T) {
	mux, svc := newTestServer(t)

	rec := do(t, mux, admin(), http.MethodGet, "/users/"+svc.user.ID.String(), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "$2a$") {
		t.Fatalf("the password digest was serialized: %s", rec.Body)
	}

	// Checked against the entity rather than the whole body: the links carry a
	// changePassword relation, which is a URL and not a credential.
	data, ok := decode(t, rec)["data"].(map[string]any)
	if !ok {
		t.Fatalf("data = %v, want the user object", decode(t, rec)["data"])
	}
	for field := range data {
		if strings.Contains(strings.ToLower(field), "password") {
			t.Errorf("the entity carries a %q field: %v", field, data)
		}
	}
}

// Reading someone else's record is an admin's privilege, and the rule is about
// who is asking with nothing in the body to weigh -- so the handler decides it.
func TestGetUserRefusesReadingSomebodyElse(t *testing.T) {
	mux, svc := newTestServer(t)
	stranger := domain.Identity{UserID: uuid.New(), Role: domain.RoleUser}

	rec := do(t, mux, stranger, http.MethodGet, "/users/"+svc.user.ID.String(), "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403: %s", rec.Code, rec.Body)
	}
}

func TestGetUserAllowsReadingYourself(t *testing.T) {
	mux, svc := newTestServer(t)
	owner := domain.Identity{UserID: svc.user.ID, Role: domain.RoleUser}

	rec := do(t, mux, owner, http.MethodGet, "/users/"+svc.user.ID.String(), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body)
	}
}

// An unauthenticated context yields a zero Identity, whose nil UUID would
// otherwise compare equal to a nil owner id.
func TestGetUserRefusesAZeroIdentity(t *testing.T) {
	mux, _ := newTestServer(t)

	rec := do(t, mux, domain.Identity{}, http.MethodGet, "/users/"+uuid.Nil.String(), "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403: %s", rec.Code, rec.Body)
	}
}

func TestCreateUserRepliesWith201AndLocation(t *testing.T) {
	mux, svc := newTestServer(t)

	rec := do(t, mux, admin(), http.MethodPost, "/users",
		`{"email":"new@example.com","password":"GoodPass1","firstName":"New","lastName":"Person","role":"staff","status":"active"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("Location"); got != "/users/"+svc.user.ID.String() {
		t.Errorf("Location = %q", got)
	}
	// This route is behind an admin gate, so a role may be stated here -- unlike
	// on public registration.
	if svc.gotInput.Role != domain.RoleStaff {
		t.Errorf("role = %q, want it passed through", svc.gotInput.Role)
	}
}

// Privileged fields are accepted by the transport and refused by the service.
// Rejecting them here instead would put an authorization rule in the layer that
// cannot see what is being changed.
func TestPatchPassesPrivilegedFieldsToTheServiceToJudge(t *testing.T) {
	mux, svc := newTestServer(t)
	actor := domain.Identity{UserID: svc.user.ID, Role: domain.RoleUser}

	rec := do(t, mux, actor, http.MethodPatch, "/users/"+svc.user.ID.String(),
		`{"role":"admin"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	if svc.gotPatch.Role == nil || *svc.gotPatch.Role != domain.RoleAdmin {
		t.Errorf("role = %v, want it forwarded for the service to refuse", svc.gotPatch.Role)
	}
	// The service cannot judge the request without knowing who is asking.
	if svc.gotActor.UserID != actor.UserID {
		t.Error("the actor was not forwarded to the service")
	}
}

func TestPatchDistinguishesOmittedFromEmpty(t *testing.T) {
	mux, svc := newTestServer(t)
	id := svc.user.ID.String()

	do(t, mux, admin(), http.MethodPatch, "/users/"+id, `{"firstName":"Renamed"}`)
	if svc.gotPatch.Email != nil {
		t.Errorf("email = %v, want nil for a field the body never mentioned", *svc.gotPatch.Email)
	}

	do(t, mux, admin(), http.MethodPatch, "/users/"+id, `{"phoneNumber":""}`)
	if svc.gotPatch.PhoneNumber == nil || *svc.gotPatch.PhoneNumber != "" {
		t.Errorf("phoneNumber = %v, want an explicit clear to reach the service", svc.gotPatch.PhoneNumber)
	}
}

func TestDeleteRefusesClosingSomebodyElsesAccount(t *testing.T) {
	mux, svc := newTestServer(t)
	stranger := domain.Identity{UserID: uuid.New(), Role: domain.RoleUser}

	rec := do(t, mux, stranger, http.MethodDelete, "/users/"+svc.user.ID.String(), "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403: %s", rec.Code, rec.Body)
	}
	if svc.deleted {
		t.Error("the delete reached the service anyway")
	}
}

func TestDeleteRepliesWith204(t *testing.T) {
	mux, svc := newTestServer(t)

	rec := do(t, mux, admin(), http.MethodDelete, "/users/"+svc.user.ID.String(), "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204: %s", rec.Code, rec.Body)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want nothing", rec.Body.String())
	}
}

// The client's own refresh token is among the ones just ended, so the reply
// points at where it has to go next.
func TestChangePasswordSaysTheSessionsAreGone(t *testing.T) {
	mux, svc := newTestServer(t)
	owner := domain.Identity{UserID: svc.user.ID, Role: domain.RoleUser}

	rec := do(t, mux, owner, http.MethodPut, "/users/"+svc.user.ID.String()+"/password",
		`{"currentPassword":"OldPass1","newPassword":"NewPass1"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body)
	}
	if svc.changedTo != "NewPass1" {
		t.Errorf("new password = %q", svc.changedTo)
	}

	links, ok := decode(t, rec)["links"].(map[string]any)
	if !ok || links["login"] == nil {
		t.Errorf("links = %v, want somewhere to sign in again", links)
	}
}

func TestListPassesPagingAndFiltersThrough(t *testing.T) {
	mux, svc := newTestServer(t)

	rec := do(t, mux, admin(), http.MethodGet,
		"/users?page=3&pageSize=5&q=sagar&role=admin&status=active&sort=-createdAt", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body)
	}

	if svc.gotPage.Number != 3 || svc.gotPage.Size != 5 {
		t.Errorf("page = %+v, want page 3 of size 5", svc.gotPage)
	}
	if svc.gotFilter.Search != "sagar" || svc.gotFilter.Role != domain.RoleAdmin {
		t.Errorf("filter = %+v", svc.gotFilter)
	}
	if svc.gotFilter.Sort.Field != "createdAt" || !svc.gotFilter.Sort.Descending {
		t.Errorf("sort = %+v, want a descending createdAt sort", svc.gotFilter.Sort)
	}

	body := decode(t, rec)
	if _, ok := body["meta"]; !ok {
		t.Error("no meta member; a client cannot page without knowing the total")
	}
	if strings.Contains(body["data"].([]any)[0].(map[string]any)["email"].(string), "@") == false {
		t.Error("the listed item does not look like a user")
	}
}

func TestNotFoundBecomes404(t *testing.T) {
	mux, svc := newTestServer(t)
	svc.err = domain.ErrUserNotFound

	rec := do(t, mux, admin(), http.MethodGet, "/users/"+svc.user.ID.String(), "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %s", rec.Code, rec.Body)
	}
}

func TestEmailTakenBecomes409(t *testing.T) {
	mux, svc := newTestServer(t)
	svc.err = domain.ErrEmailTaken

	rec := do(t, mux, admin(), http.MethodPost, "/users",
		`{"email":"taken@example.com","password":"GoodPass1","firstName":"New","lastName":"Person"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409: %s", rec.Code, rec.Body)
	}
}
