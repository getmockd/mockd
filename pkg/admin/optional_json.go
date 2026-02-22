package admin

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// decodeOptionalJSONBody decodes JSON when a body is present.
// Empty bodies are treated as "not provided" and are not errors.
func decodeOptionalJSONBody(r *http.Request, dst any) error {
	if r == nil || r.Body == nil || r.Body == http.NoBody {
		return nil
	}
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	return nil
}
