# nam-tunnel — Implemented Features & Enhancement Guide

A minimal HTTP reverse proxy over a single WebSocket: public clients hit the **relay**, traffic is tunneled to an **agent** behind NAT/firewall, and the agent forwards real HTTP to a local **backend**.

```
[Client] --HTTP--> [relay :8001] <--WebSocket--> [agent] --HTTP--> [backend :8000]
```

---

## Repository layout

| Component | Path | Role |
|-----------|------|------|
| Relay | `relay/main.go` | Public HTTP + WebSocket registry |
| Agent | `agent/main.go` | Outbound dial; executes HTTP locally |
| Protocol | `protocol/message.go` | JSON request/response framing |
| Backend | `backend/main.go` | Demo app for local testing |
| Tests | `relay/path_test.go` | Path stripping regression tests |

---

## What is implemented

### 1. Real HTTP tunnel (not opaque bytes)

The relay forwards **method**, **path + query**, **headers**, and **body**. The agent builds a real `http.Request` to `LOCAL_URL` and returns **status**, **headers**, and **body**.

- Protocol types: `protocol.Request`, `protocol.Response` (`protocol/message.go`)
- Hop-by-hop headers stripped: `Connection`, `Upgrade`, `Host`, etc. (`CopyForwardHeaders`)

**Public URL → local path**

| Request to relay | Proxied to backend |
|------------------|-------------------|
| `GET /backend` | `GET /` |
| `GET /backend/health` | `GET /health` |
| `GET /backend/api/v1/users?page=1` | `GET /api/v1/users?page=1` |

Routing uses Go 1.22+ patterns with a catch-all:

```go
mux.HandleFunc("/{id}/{path...}", proxy)
mux.HandleFunc("/{id}", proxy)
```

`proxyPath()` reads `r.PathValue("path")` — the `{path...}` wildcard is required; without it every request was incorrectly forwarded as `/`.

---

### 2. Multiplexing (concurrent requests per tunnel)

**Problem:** One WebSocket, many HTTP clients — responses must not be swapped.

**Solution:** Each proxied request gets a unique `ID`; the relay keeps `pending[id] → chan Response` until the matching response arrives.

| Location | What it does |
|----------|----------------|
| `relay/main.go` — `newRequestID()` | 16 random bytes → hex id |
| `relay/main.go` — `Tunnel.pending` | `sync.Map` of waiters |
| `relay/main.go` — `roundTrip()` | Register channel → send WS message → wait on channel |
| `relay/main.go` — `connect()` read loop | `resp.ID` → deliver to correct channel |
| `agent/main.go` — `handleRequest()` | Copies `req.ID` into `protocol.Response` |

`writeMu` only serializes **WebSocket writes**, not the full round trip — many requests can be in flight.

---

### 3. Reconnect (same tunnel id)

When a new agent connects with an existing `id`, the old WebSocket is closed and the map entry is replaced.

```text
relay/main.go connect() lines ~56–61:
  if old, ok := tunnels[id]; ok { old.conn.Close() }
  tunnels[id] = tunnel
```

The old handler’s `defer` uses `if tunnels[id] == tunnel` before `delete` so a stale disconnect does not remove the **new** tunnel.

---

### 4. Disconnect cleanup (no hung HTTP clients)

On agent disconnect, `failAll("tunnel closed")` notifies every pending waiter with HTTP 502 semantics (`StatusBadGateway` + `Error`).

```text
relay/main.go:
  defer → tunnel.failAll("tunnel closed")   (~71)
  failAll()                                 (~94–104)
  proxy() → http.Error(..., 502) if resp.Error (~181–183)
```

---

### 5. Per-request timeout (60s on relay)

```text
relay/main.go proxy():
  ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
  tunnel.roundTrip(ctx, req)

roundTrip() select:
  case <-ctx.Done(): return ctx.Err()

proxy() on err:
  http.Error(w, err.Error(), 502)   // e.g. "context deadline exceeded"
```

After timeout, `defer pending.Delete(req.ID)` drops the waiter — a late agent response is ignored (no channel match).

The agent uses `http.Client{Timeout: 0}`; relay timeout is the primary limit today.

---

### 6. Agent configuration (env)

| Variable | Default | Purpose |
|----------|---------|---------|
| `TUNNEL_ID` | `backend` | Tunnel name in relay URL |
| `RELAY_WS` | `ws://localhost:8001/connect?id=...` | WebSocket dial target |
| `LOCAL_URL` | `http://localhost:8000` | Local backend base URL |

---

### 7. Graceful shutdown (relay & backend)

`signal.NotifyContext` + `http.Server.Shutdown` with a 5s drain window on relay and backend.

---

### 8. Tests

`relay/path_test.go` — verifies `proxyPath` for `/backend`, `/backend/`, `/backend/api/v1/users`, and query strings.

---

## How to run & verify

```bash
# Terminal 1 — local app
go run ./backend

# Terminal 2 — public relay
go run ./relay

# Terminal 3 — tunnel client (behind NAT)
go run ./agent

# Terminal 4 — through the tunnel
curl -i http://localhost:8001/backend/
curl -i http://localhost:8001/backend/health
curl -i http://localhost:8001/backend/api/v1/users   # 10s sleep on backend; tests timeout/multiplexing
```

**Multiplexing check:** Hit `/api/v1/users` twice in parallel; both should complete with correct JSON (not swapped), as long as each finishes within 60s.

---

## Known limitations (current code)

1. **No authentication** — Anyone can register any tunnel `id` (`CheckOrigin: true`, no token on `/connect` or HTTP proxy).
2. **Full body buffering** — `io.ReadAll` on relay and agent; large uploads/downloads load entire body into memory.
3. **JSON over WebSocket** — Simple to debug; not ideal for binary-heavy traffic or very large payloads.
4. **No TLS** — `ws://` and `http://` only; production needs `wss://` / `https://` termination.
5. **No agent auto-reconnect** — Agent exits on disconnect; no backoff loop in `agent/main.go`.
6. **Single agent per id** — By design; no load balancing across multiple agents for one id.
7. **Redirect handling** — Agent returns last response (`ErrUseLastResponse`); 3xx `Location` may still point at `localhost`.
8. **WebSocket ping/pong** — No keepalive; idle connections may be dropped by middleboxes without notice.
9. **502 vs 504** — Timeouts and tunnel errors both surface as 502 from `proxy`.
10. **Reconnect race** — In-flight requests on the **old** tunnel get `failAll` when old socket closes; clients must retry. No request replay buffer.

---

## Enhancement roadmap

Prioritized by impact for moving from **dev demo** → **usable tunnel**.

### P0 — Security (before any public exposure)

| Item | Guidance |
|------|----------|
| Connect token | Require `?id=X&token=Y` or `Authorization` on WebSocket upgrade; constant-time compare against server secret. |
| Tunnel ownership | One secret per tunnel id, or register tunnels in a store (DB/config). |
| Restrict `CheckOrigin` | Allowlist origins in production. |
| TLS | Terminate HTTPS/WSS at relay (Caddy, nginx, or `tls.Listen`). |

### P1 — Reliability

| Item | Guidance |
|------|----------|
| Agent reconnect loop | On `ReadMessage` / dial error: exponential backoff, redial same `RELAY_WS`. |
| WebSocket keepalive | `SetPingHandler` / periodic ping from agent or relay; set read/write deadlines. |
| Propagate client cancel | Use `r.Context()` through agent `http.NewRequestWithContext` so client abort cancels local request. |
| Agent-side timeout | `http.Client.Timeout` slightly below relay timeout (e.g. 55s) for clearer errors. |
| Status code hygiene | `504 Gateway Timeout` for `context.DeadlineExceeded`; reserve `502` for bad gateway / agent errors. |

### P2 — Performance & scale

| Item | Guidance |
|------|----------|
| Streaming bodies | Chunk over WS (frame type + stream id + eof) or cap max body size with `413`. |
| Binary framing | Replace JSON with msgpack/protobuf, or HTTP/1.1 wire bytes in frames, for lower overhead. |
| Limit pending map | Max in-flight per tunnel; `429` or `503` when overloaded. |
| Metrics | Prometheus: active tunnels, pending count, latency histogram, bytes transferred. |

### P3 — Product / DX

| Item | Guidance |
|------|----------|
| Custom domain per tunnel | `Host` routing + TLS SNI instead of path-prefix `/{id}/`. |
| Admin API | List connected tunnels, last seen, disconnect. |
| Config file | YAML/TOML for relay listen addr, tokens, timeouts. |
| Integration tests | Spin relay + agent + backend in `testing`; assert path, status, multiplexing, disconnect 502. |
| README | Link to this doc; architecture diagram; env table. |

### P4 — Advanced HTTP semantics

| Item | Guidance |
|------|----------|
| WebSocket upgrade through tunnel | Harder; needs full-duplex stream proxy, not request/response JSON. |
| SSE / chunked encoding | Stream response chunks in `protocol.Response` frames. |
| Fix redirect `Location` | Rewrite `Location` header to public relay host on the way out. |
| Trailer headers | Forward only if framing supports end-of-stream markers. |

---

## Suggested next steps (practical order)

1. **Agent reconnect with backoff** — Smallest change, biggest ops win for flaky networks.
2. **Shared secret on `/connect`** — Blocks arbitrary tunnel hijacking.
3. **Context propagation** — `NewRequestWithContext(ctx, ...)` in agent.
4. **Integration test** — Lock in path routing and multiplexing.
5. **Max body size** — Protect relay memory before streaming.

---

## Code reference index

| Feature | File | Symbols / lines |
|---------|------|-----------------|
| HTTP framing | `protocol/message.go` | `Request`, `Response`, `CopyForwardHeaders` |
| Path routing | `relay/main.go` | `proxyPath`, mux `/{id}/{path...}` |
| Multiplexing | `relay/main.go` | `pending`, `roundTrip`, `newRequestID` |
| Reconnect | `relay/main.go` | `connect` ~56–69 |
| Disconnect 502 | `relay/main.go` | `failAll`, `proxy` error branch |
| 60s timeout | `relay/main.go` | `proxy` `WithTimeout`, `roundTrip` select |
| Local HTTP exec | `agent/main.go` | `handleRequest` |
| Demo backend | `backend/main.go` | `/`, `/health`, `/api/v1/users` |

---

## Summary

**You have a working HTTP reverse proxy tunnel** with correct path stripping, request/response correlation (multiplexing), reconnect replacement, disconnect fail-fast, and relay-side timeouts. It is suitable for **local learning and controlled demos**.

**Before production:** authentication, TLS, reconnecting agent, body limits or streaming, and clearer timeout/status semantics. Use the P0–P4 tables above as a checklist.
