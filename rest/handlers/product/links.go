package product

import (
	"go-commerce/domain"
	"go-commerce/rest/response"
)

// basePath is the collection's own path, in one constant so the routes and the
// links they emit cannot drift apart.
const basePath = "/products"

// productLinks is what a client can do next with one product.
//
// Naming the write relations is the part that earns the term HATEOAS: a client
// discovers that a product can be replaced or deleted from the response, rather
// than from documentation it has to keep in sync by hand.
func productLinks(p domain.Product) response.Links {
	self := response.Path(basePath, p.ID.String())

	return response.Links{
		"self":       self,
		"update":     self,
		"replace":    self,
		"delete":     self,
		"collection": basePath,
	}
}
