package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nambuitechx/nam-tunnel/protocol"
)

type Tunnel struct {
	id      string
	conn    *websocket.Conn
	writeMu sync.Mutex
	pending sync.Map // request id -> chan protocol.Response
}

var (
	tunnels = map[string]*Tunnel{}
	mu      sync.RWMutex
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func newRequestID() string {
	var b [16]byte
	rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func connect(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	tunnel := &Tunnel{id: id, conn: ws}

	mu.Lock()
	if old, ok := tunnels[id]; ok {
		old.conn.Close()
	}
	tunnels[id] = tunnel
	mu.Unlock()

	log.Println("agent connected:", id)

	defer func() {
		mu.Lock()
		if tunnels[id] == tunnel {
			delete(tunnels, id)
		}
		mu.Unlock()
		tunnel.failAll("tunnel closed")
		ws.Close()
		log.Println("agent disconnected:", id)
	}()

	for {
		_, msg, err := ws.ReadMessage()
		if err != nil {
			return
		}
		resp, err := protocol.DecodeResponse(msg)
		if err != nil {
			log.Println("invalid response from agent:", err)
			continue
		}
		ch, ok := tunnel.pending.Load(resp.ID)
		if !ok {
			continue
		}
		ch.(chan protocol.Response) <- resp
	}
}

func (t *Tunnel) failAll(reason string) {
	t.pending.Range(func(key, value any) bool {
		ch := value.(chan protocol.Response)
		select {
		case ch <- protocol.Response{ID: key.(string), Status: http.StatusBadGateway, Error: reason}:
		default:
		}
		t.pending.Delete(key)
		return true
	})
}

func (t *Tunnel) roundTrip(ctx context.Context, req protocol.Request) (protocol.Response, error) {
	ch := make(chan protocol.Response, 1)
	t.pending.Store(req.ID, ch)
	defer t.pending.Delete(req.ID)

	data, err := protocol.Encode(req)
	if err != nil {
		return protocol.Response{}, err
	}

	t.writeMu.Lock()
	err = t.conn.WriteMessage(websocket.TextMessage, data)
	t.writeMu.Unlock()
	if err != nil {
		return protocol.Response{}, err
	}

	select {
	case resp := <-ch:
		return resp, nil
	case <-ctx.Done():
		return protocol.Response{}, ctx.Err()
	}
}

// proxyPath returns the path sent to the local server (e.g. /api/v1/users?q=1).
// Requires a mux pattern with {path...}, e.g. /{id}/{path...}.
func proxyPath(r *http.Request) string {
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

func proxy(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	mu.RLock()
	tunnel := tunnels[id]
	mu.RUnlock()

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
		Path:   proxyPath(r),
		Header: make(http.Header),
		Body:   body,
	}
	protocol.CopyForwardHeaders(req.Header, r.Header)

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	resp, err := tunnel.roundTrip(ctx, req)
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

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/connect", connect)
	// {path...} captures everything after the tunnel id (api/v1/users, etc.)
	mux.HandleFunc("/{id}/{path...}", proxy)
	mux.HandleFunc("/{id}", proxy)

	srv := &http.Server{Addr: ":8001", Handler: mux}

	go func() {
		log.Println("relay server is listening on :8001")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	<-ctx.Done()
	log.Println("relay server is shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatal(err)
	}
	log.Println("relay server is gracefully shutted down")
}
