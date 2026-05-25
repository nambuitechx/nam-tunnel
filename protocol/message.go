package protocol

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Request is sent relay → agent over the WebSocket.
type Request struct {
	ID     string              `json:"id"`
	Method string              `json:"method"`
	Path   string              `json:"path"` // path + optional ?query (no host)
	Header http.Header         `json:"header"`
	Body   []byte              `json:"body,omitempty"`
}

// Response is sent agent → relay over the WebSocket.
type Response struct {
	ID     string      `json:"id"`
	Status int         `json:"status"`
	Header http.Header `json:"header"`
	Body   []byte      `json:"body,omitempty"`
	Error  string      `json:"error,omitempty"` // agent-side failure (no HTTP response)
}

func Encode(v any) ([]byte, error) {
	return json.Marshal(v)
}

func DecodeRequest(data []byte) (Request, error) {
	var req Request
	err := json.Unmarshal(data, &req)
	return req, err
}

func DecodeResponse(data []byte) (Response, error) {
	var resp Response
	err := json.Unmarshal(data, &resp)
	return resp, err
}

// Hop-by-hop and WebSocket-specific headers must not be forwarded.
var hopHeaders = map[string]bool{
	"connection":          true,
	"keep-alive":          true,
	"proxy-authenticate":  true,
	"proxy-authorization": true,
	"te":                  true,
	"trailers":            true,
	"transfer-encoding":   true,
	"upgrade":             true,
}

func CopyForwardHeaders(dst, src http.Header) {
	for k, vv := range src {
		lower := strings.ToLower(k)
		if hopHeaders[lower] || lower == "host" {
			continue
		}
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}
