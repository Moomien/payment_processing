package httputil

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecodeJSON_Validation(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		body string
	}{
		{name: "unknown field", body: `{"known":"ok","unknown":true}`},
		{name: "multiple objects", body: `{"known":"ok"} {"known":"second"}`},
		{name: "too large", body: `{"known":"` + strings.Repeat("a", maxJSONBodyBytes) + `"}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			var dst struct {
				Known string `json:"known"`
			}
			require.Error(t, DecodeJSON(rec, req, &dst))
		})
	}
}
