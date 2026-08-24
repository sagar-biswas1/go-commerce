package auth

import (
	"net/http"

	"go-commerce/domain"
	"go-commerce/rest/helpers"
	"go-commerce/rest/middlewares"
	"go-commerce/rest/response"
)

// Sessions lists the caller's own active sessions -- what a "signed in on these
// devices" screen shows.
//
// The user id comes from the access token and never from the path or a query
// parameter, so there is no id for a caller to change in order to read somebody
// else's sessions.
func (h *Handler) Sessions(w http.ResponseWriter, r *http.Request) {
	actor := middlewares.MustIdentity(r.Context())
	page := response.PageFromRequest(r)

	result, err := h.service.Sessions(r.Context(), actor.UserID, page)
	if err != nil {
		response.Fail(w, r, err)
		return
	}

	links := response.PageLinks(r, result, page)
	links["revokeAll"] = basePath + "/logout-all"

	response.Collection(w, http.StatusOK,
		response.Collect(result.Items, sessionResourceLinks),
		links,
		response.PageMeta(result, page, result.Total),
	)
}

func sessionResourceLinks(t domain.RefreshToken) response.Links {
	self := response.Path(basePath, "sessions", t.ID.String())
	return response.Links{
		"self":   self,
		"revoke": self,
	}
}

// RevokeSession ends one named session, so a user who sees a device they do not
// recognise can sign just that one out.
func (h *Handler) RevokeSession(w http.ResponseWriter, r *http.Request) {
	sessionID, err := helpers.PathUUID(r, "id")
	if err != nil {
		response.Fail(w, r, err)
		return
	}

	actor := middlewares.MustIdentity(r.Context())
	if err := h.service.RevokeSession(r.Context(), actor.UserID, sessionID); err != nil {
		response.Fail(w, r, err)
		return
	}

	response.NoContent(w)
}
