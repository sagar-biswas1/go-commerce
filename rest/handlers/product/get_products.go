package product

import (
	"net/http"

	"go-commerce/domain"
	"go-commerce/rest/response"
)

// GetProducts lists products, one page at a time.
//
// Every listing is paged -- there is no "all products" reply. A collection that
// returns everything works fine on a hundred rows and takes the service down on
// a million, and the endpoint that has to change is the one every client already
// depends on.
func (h *Handler) GetProducts(w http.ResponseWriter, r *http.Request) {
	page := response.PageFromRequest(r)

	filter := domain.ProductFilter{
		Search:   r.URL.Query().Get("q"),
		MinPrice: response.FloatParam(r, "minPrice"),
		MaxPrice: response.FloatParam(r, "maxPrice"),
		Sort:     response.SortFromRequest(r),
	}

	result, err := h.service.List(r.Context(), page, filter)
	if err != nil {
		response.Fail(w, r, err)
		return
	}

	links := response.PageLinks(r, result, page)
	links["create"] = basePath

	response.Collection(w, http.StatusOK,
		response.Collect(result.Items, productLinks),
		links,
		response.PageMeta(result, page, result.Total),
	)
}
