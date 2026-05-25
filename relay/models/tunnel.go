package relay_models

import (
	"context"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/nambuitechx/nam-tunnel/protocol"
)

type Tunnel struct {
	id      string
	conn    *websocket.Conn
	writeMu sync.Mutex
	pending sync.Map // request id -> chan protocol.Response
}

func NewTunnel(id string, conn *websocket.Conn) *Tunnel {
	return &Tunnel{id: id, conn: conn}
}

func (t *Tunnel) ID() string {
	return t.id
}

func (t *Tunnel) Close() error {
	return t.conn.Close()
}

func (t *Tunnel) ReadMessage() ([]byte, error) {
	_, msg, err := t.conn.ReadMessage()
	return msg, err
}

func (t *Tunnel) NotifyResponse(msg []byte) error {
	resp, err := protocol.DecodeResponse(msg)
	if err != nil {
		return err
	}
	ch, ok := t.pending.Load(resp.ID)
	if !ok {
		return nil
	}
	ch.(chan protocol.Response) <- resp
	return nil
}

func (t *Tunnel) FailAll(reason string) {
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

func (t *Tunnel) RoundTrip(ctx context.Context, req protocol.Request) (protocol.Response, error) {
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
