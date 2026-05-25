package relay_handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nambuitechx/nam-tunnel/protocol"
	relay_models "github.com/nambuitechx/nam-tunnel/relay/models"
)

type TunnelHandler struct {
	tunnels  map[string]*relay_models.Tunnel
	mu       sync.RWMutex
	upgrader websocket.Upgrader
}

func NewTunnelHandler() *TunnelHandler {
	return &TunnelHandler{
		tunnels: make(map[string]*relay_models.Tunnel),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

func newRequestID() string {
	var b [16]byte
	rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func (h *TunnelHandler) Connect(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	ws, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	tunnel := relay_models.NewTunnel(id, ws)

	h.mu.Lock()
	if old, ok := h.tunnels[id]; ok {
		old.Close()
	}
	h.tunnels[id] = tunnel
	h.mu.Unlock()

	log.Println("agent connected:", id)

	defer func() {
		h.mu.Lock()
		if h.tunnels[id] == tunnel {
			delete(h.tunnels, id)
		}
		h.mu.Unlock()
		tunnel.FailAll("tunnel closed")
		ws.Close()
		log.Println("agent disconnected:", id)
	}()

	for {
		msg, err := tunnel.ReadMessage()
		if err != nil {
			return
		}
		if err := tunnel.NotifyResponse(msg); err != nil {
			log.Println("invalid response from agent:", err)
		}
	}
}

// ProxyPath returns the path sent to the local server (e.g. /api/v1/users?q=1).
// Requires a mux pattern with {path...}, e.g. /{id}/{path...}.
func ProxyPath(r *http.Request) string {
	sub := r.PathValue("path")
	path := "/"
	if sub != "" {
		path = "/" + sub
	}
	if q := r.URL.RawQuery; q != "" {
		path += "?" + q
	}
	return path
}

func (h *TunnelHandler) Proxy(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	h.mu.RLock()
	tunnel := h.tunnels[id]
	h.mu.RUnlock()

	if tunnel == nil {
		http.Error(w, "tunnel offline", http.StatusNotFound)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}

	req := protocol.Request{
		ID:     newRequestID(),
		Method: r.Method,
		Path:   ProxyPath(r),
		Header: make(http.Header),
		Body:   body,
	}
	protocol.CopyForwardHeaders(req.Header, r.Header)

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	resp, err := tunnel.RoundTrip(ctx, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	if resp.Error != "" {
		http.Error(w, resp.Error, http.StatusBadGateway)
		return
	}

	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.Status)
	w.Write(resp.Body)
}
