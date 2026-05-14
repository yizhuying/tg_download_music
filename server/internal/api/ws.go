package api

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// WSMessage represents a WebSocket message exchanged with the frontend.
type WSMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

// Hub manages WebSocket client connections and broadcasts.
type Hub struct {
	mu         sync.RWMutex
	clients    map[*websocket.Conn]bool
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	broadcast  chan WSMessage
	upgrader   websocket.Upgrader
	getState   func() interface{}
}

// NewHub creates a new Hub with an optional state snapshot function.
func NewHub(getState func() interface{}) *Hub {
	return &Hub{
		clients:    make(map[*websocket.Conn]bool),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
		broadcast:  make(chan WSMessage, 256),
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
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			if h.getState != nil {
				state := h.getState()
				h.broadcast <- WSMessage{Type: "download_status", Payload: state}
			}

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.Close()
			}
			h.mu.Unlock()

		case msg := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				if err := client.WriteJSON(msg); err != nil {
					client.Close()
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast sends a typed message to all connected WebSocket clients.
func (h *Hub) Broadcast(typ string, data interface{}) {
	h.broadcast <- WSMessage{Type: typ, Payload: data}
}

const (
	pingInterval = 30 * time.Second
	pongWait     = 60 * time.Second
)

// HandleWS upgrades an HTTP connection to WebSocket and pumps read messages.
func (h *Hub) HandleWS(c *gin.Context) {
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	h.register <- conn

	go func() {
		ticker := time.NewTicker(pingInterval)
		defer ticker.Stop()
		for range ticker.C {
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(pongWait)); err != nil {
				conn.Close()
				return
			}
		}
	}()

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			h.unregister <- conn
			return
		}
	}
}
