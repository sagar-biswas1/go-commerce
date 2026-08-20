package helpers

import (
	"fmt"
	"net/http"
	"strconv"
)

// PathInt reads an integer wildcard such as {id} out of the request path.
//
// It returns an error instead of writing to w on purpose: the helper has no
// opinion about how a module reports a bad request, so every resource can
// phrase its own message without this package knowing any of them.
func PathInt(r *http.Request, name string) (int, error) {
	raw := r.PathValue(name)

	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return 0, fmt.Errorf("path value %q is not a positive integer: %q", name, raw)
	}
	return value, nil
}
