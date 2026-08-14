package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	// wsWriteWait is the deadline for writing a message to the peer.
	wsWriteWait = 10 * time.Second
	// wsReadWait is the deadline for reading from the peer; the frontend
	// heartbeat refreshes it every 10 seconds.
	wsReadWait = 60 * time.Second
	// wsMaxMessageSize limits application-level messages from the peer.
	wsMaxMessageSize = 512
	// wsSendBuffer is the per-client outgoing buffer before the client is
	// considered too slow and dropped.
	wsSendBuffer = 64
)

// WSMessage represents a WebSocket message exchanged with the frontend.
type WSMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

// wsClient couples a connection with a dedicated send channel so that all
// writes to the connection happen from a single goroutine. The channel
// carries pre-marshaled frames so each message is serialized only once no
// matter how many clients are connected.
type wsClient struct {
	conn *websocket.Conn
	send chan []byte
}

// Hub manages WebSocket client connections and broadcasts.
// The clients map is owned exclusively by the Run goroutine.
type Hub struct {
	clients    map[*wsClient]bool
	register   chan *wsClient
	unregister chan *wsClient
	broadcast  chan []byte
	upgrader   websocket.Upgrader
	getState   func() interface{}
}

// NewHub creates a new Hub with an optional state snapshot function that is
// pushed to a client right after it connects.
func NewHub(getState func() interface{}) *Hub {
	return &Hub{
		clients:    make(map[*wsClient]bool),
		register:   make(chan *wsClient),
		unregister: make(chan *wsClient),
		broadcast:  make(chan []byte, 256),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		getState: getState,
	}
}

// Run processes registration, unregistration, and broadcast messages.
func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			h.clients[c] = true
			if h.getState != nil {
				if data, err := json.Marshal(WSMessage{Type: "download_status", Payload: h.getState()}); err == nil {
					select {
					case c.send <- data:
					default:
					}
				}
			}

		case c := <-h.unregister:
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(c.send)
			}

		case msg := <-h.broadcast:
			for c := range h.clients {
				select {
				case c.send <- msg:
				default:
					// Client is too slow, drop it instead of blocking the hub.
					delete(h.clients, c)
					close(c.send)
				}
			}
		}
	}
}

// Broadcast marshals a typed message once and sends the frame to all
// connected WebSocket clients. It never blocks: when the hub is overwhelmed,
// messages are dropped.
func (h *Hub) Broadcast(typ string, data interface{}) {
	payload, err := json.Marshal(WSMessage{Type: typ, Payload: data})
	if err != nil {
		return
	}
	select {
	case h.broadcast <- payload:
	default:
	}
}

// HandleWS upgrades an HTTP connection to WebSocket and starts its pumps.
func (h *Hub) HandleWS(c *gin.Context) {
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	client := &wsClient{conn: conn, send: make(chan []byte, wsSendBuffer)}
	h.register <- client

	go h.writePump(client)
	h.readPump(client)
}

var wsPongFrame = []byte(`{"type":"pong"}`)

// readPump reads messages from the connection, refreshing the read deadline
// on activity and answering the application-level heartbeat.
func (h *Hub) readPump(c *wsClient) {
	defer func() {
		h.unregister <- c
		_ = c.conn.Close()
	}()
	c.conn.SetReadLimit(wsMaxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(wsReadWait))
	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		_ = c.conn.SetReadDeadline(time.Now().Add(wsReadWait))
		if string(msg) == `{"type":"ping"}` {
			select {
			case c.send <- wsPongFrame:
			default:
			}
		}
	}
}

// writePump is the only writer of the connection.
func (h *Hub) writePump(c *wsClient) {
	defer c.conn.Close()
	for msg := range c.send {
		_ = c.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
	_ = c.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
	_ = c.conn.WriteMessage(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
}
