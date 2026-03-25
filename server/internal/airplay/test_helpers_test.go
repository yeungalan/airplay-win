package airplay

import (
	"bytes"
	"net/http"
	"net/http/httptest"
)

// Test helpers

func newPostRequest(path string, body []byte) *http.Request {
	req := httptest.NewRequest("POST", path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/octet-stream")
	return req
}

func recordRequest(mux *http.ServeMux, req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	return w
}
