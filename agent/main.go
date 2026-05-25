package main

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/nambuitechx/nam-tunnel/protocol"
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func handleRequest(localBase *url.URL, client *http.Client, raw []byte) protocol.Response {
	req, err := protocol.DecodeRequest(raw)
	if err != nil {
		return protocol.Response{Error: "invalid request: " + err.Error()}
	}

	target, err := url.Parse(req.Path)
	if err != nil {
		return protocol.Response{ID: req.ID, Error: "invalid path: " + err.Error()}
	}

	// req.Path is "/foo?bar=1" — join with local base (scheme + host + optional base path).
	localURL := *localBase
	localURL.Path = strings.TrimSuffix(localBase.Path, "/") + target.Path
	localURL.RawQuery = target.RawQuery

	httpReq, err := http.NewRequest(req.Method, localURL.String(), bytes.NewReader(req.Body))
	if err != nil {
		return protocol.Response{ID: req.ID, Error: err.Error()}
	}
	protocol.CopyForwardHeaders(httpReq.Header, req.Header)

	resp, err := client.Do(httpReq)
	if err != nil {
		return protocol.Response{ID: req.ID, Error: err.Error()}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return protocol.Response{ID: req.ID, Error: err.Error()}
	}

	out := protocol.Response{
		ID:     req.ID,
		Status: resp.StatusCode,
		Header: resp.Header.Clone(),
		Body:   body,
	}
	// Content-Length on the wire may not match after relay; let the relay set it.
	out.Header.Del("Content-Length")
	return out
}

func main() {
	tunnelID := env("TUNNEL_ID", "backend")
	relayWS := env("RELAY_WS", "ws://localhost:8001/connect?id="+tunnelID)
	localURL := env("LOCAL_URL", "http://localhost:8000")

	base, err := url.Parse(localURL)
	if err != nil {
		log.Fatal(err)
	}

	client := &http.Client{
		Timeout: 0, // per-request timeout is enforced by relay
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	ws, _, err := websocket.DefaultDialer.Dial(relayWS, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer ws.Close()

	log.Println("connected to relay as", tunnelID, "→", localURL)

	for {
		_, msg, err := ws.ReadMessage()
		if err != nil {
			log.Println("relay disconnected:", err)
			return
		}

		out := handleRequest(base, client, msg)
		data, err := protocol.Encode(out)
		if err != nil {
			log.Println("encode response:", err)
			continue
		}
		if err := ws.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Println("write to relay:", err)
			return
		}
	}
}
