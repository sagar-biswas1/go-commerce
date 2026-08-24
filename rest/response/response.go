// Package response is the one place this API decides what a reply looks like.
//
// Every body -- success or failure, item or collection -- is a JSON object with
// the same three members, so a client writes one parser and not one per
// endpoint:
//
//	{"data": ..., "links": {...}, "meta": {...}}
//	{"error": {"code": ..., "message": ..., "fields": {...}}}
//
// Links are what make the API navigable: a response says where to go next
// rather than expecting the client to know how to build the URL.
package response

import (
	"encoding/json"
	"log"
	"net/http"
)

// Links is a set of named relations, keyed by IANA link relation names --
// "self", "next", "prev", "first", "last", and the resource-specific ones a
// handler adds.
type Links map[string]string

// Meta carries what a client needs to page through a collection without
// guessing how big it is.
type Meta struct {
	Page       int `json:"page"`
	PageSize   int `json:"pageSize"`
	Total      int `json:"total"`
	TotalPages int `json:"totalPages"`
}

// envelope is the success shape. Links and meta are omitted when empty, so a
// plain item response does not carry two empty objects.
type envelope struct {
	Data  any   `json:"data"`
	Links Links `json:"links,omitempty"`
	Meta  *Meta `json:"meta,omitempty"`
}

// errorEnvelope is the failure shape.
type errorEnvelope struct {
	Error ErrorBody `json:"error"`
}

// ErrorBody names the failure twice: a stable code for a client to branch on,
// and a message for a human to read. Fields carries per-field validation
// messages, so a form can mark every bad input at once instead of one per
// round trip.
type ErrorBody struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

// JSON writes a body and status. It is the only function here that touches the
// ResponseWriter, so the Content-Type is set in exactly one place.
func JSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	// A JSON body is never something to guess the type of; saying so stops a
	// browser from ever treating an error message as markup.
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)

	if body == nil {
		return
	}

	if err := json.NewEncoder(w).Encode(body); err != nil {
		// The status line is already on the wire, so there is no way to turn
		// this into an error response -- the client sees a truncated body.
		log.Printf("[response] encoding body: %v", err)
	}
}

// Item replies with a single resource and its links.
func Item(w http.ResponseWriter, status int, data any, links Links) {
	JSON(w, status, envelope{Data: data, Links: links})
}

// NoContent replies with an empty 204, for a delete that leaves nothing to say.
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// Collection replies with a page of resources, its navigation links, and the
// counts a client needs to page through it.
func Collection(w http.ResponseWriter, status int, data any, links Links, meta Meta) {
	JSON(w, status, envelope{Data: data, Links: links, Meta: &meta})
}
