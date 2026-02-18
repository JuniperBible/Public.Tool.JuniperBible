package api

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/FocuswithJustin/JuniperBible/internal/logging"
	"github.com/gorilla/websocket"
)

var (
	// GlobalHub is the shared WebSocket hub for broadcasting progress updates.
	GlobalHub *Hub

	// GlobalWebSocketRateLimiter is the shared rate limiter for WebSocket messages.
	GlobalWebSocketRateLimiter *WebSocketRateLimiter

	// WebSocket upgrader with CORS configuration
	// DEPRECATED: Use secureUpgrader via SecureWebSocketHandler instead.
	// This upgrader allows all origins and is only for backward compatibility.
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins for API usage - INSECURE
		},
	}
)

// ProgressMessage represents a progress update sent via WebSocket.
type ProgressMessage struct {
	Type      string                 `json:"type"`      // "progress", "complete", "error"
	Operation string                 `json:"operation"` // "convert", "ingest", "export", etc.
	Stage     string                 `json:"stage"`     // Current stage of operation
	Progress  int                    `json:"progress"`  // 0-100
	Message   string                 `json:"message"`   // Human-readable status
	Timestamp string                 `json:"timestamp"` // ISO 8601 timestamp
	Data      map[string]interface{} `json:"data,omitempty"`
}

// Client represents a WebSocket client connection.
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

// Hub maintains active WebSocket connections and broadcasts messages.
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

// NewHub creates a new WebSocket hub.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run starts the hub's main loop to handle client registration and broadcasting.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.handleRegister(client)
		case client := <-h.unregister:
			h.handleUnregister(client)
		case message := <-h.broadcast:
			h.handleBroadcast(message)
		}
	}
}

// handleRegister handles client registration.
func (h *Hub) handleRegister(client *Client) {
	h.mu.Lock()
	h.clients[client] = true
	h.mu.Unlock()
	logging.WebSocketEvent("client_connected", len(h.clients))
}

// handleUnregister handles client unregistration.
func (h *Hub) handleUnregister(client *Client) {
	h.mu.Lock()
	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		close(client.send)
	}
	h.mu.Unlock()
	logging.WebSocketEvent("client_disconnected", len(h.clients))
}

// handleBroadcast sends message to all clients.
func (h *Hub) handleBroadcast(message []byte) {
	h.mu.RLock()
	for client := range h.clients {
		select {
		case client.send <- message:
		default:
			close(client.send)
			delete(h.clients, client)
		}
	}
	h.mu.RUnlock()
}

// Broadcast sends a progress message to all connected clients.
func (h *Hub) Broadcast(msg ProgressMessage) {
	// Set timestamp if not already set
	if msg.Timestamp == "" {
		msg.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	data, err := json.Marshal(msg)
	if err != nil {
		logging.Error("failed to marshal progress message", "error", err)
		return
	}

	select {
	case h.broadcast <- data:
	default:
		logging.Warn("broadcast channel full, dropping message")
	}
}

// BroadcastProgress sends a progress update to all connected clients.
func BroadcastProgress(operation, stage, message string, progress int) {
	if GlobalHub == nil {
		return
	}

	GlobalHub.Broadcast(ProgressMessage{
		Type:      "progress",
		Operation: operation,
		Stage:     stage,
		Progress:  progress,
		Message:   message,
	})
}

// BroadcastComplete sends a completion message to all connected clients.
func BroadcastComplete(operation, message string, data map[string]interface{}) {
	if GlobalHub == nil {
		return
	}

	GlobalHub.Broadcast(ProgressMessage{
		Type:      "complete",
		Operation: operation,
		Progress:  100,
		Message:   message,
		Data:      data,
	})
}

// BroadcastError sends an error message to all connected clients.
func BroadcastError(operation, message string) {
	if GlobalHub == nil {
		return
	}

	GlobalHub.Broadcast(ProgressMessage{
		Type:      "error",
		Operation: operation,
		Message:   message,
	})
}

// readPump reads messages from the WebSocket connection.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logging.Error("websocket unexpected close", "error", err)
			}
			break
		}
	}
}

// writePump writes messages to the WebSocket connection.
func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !c.handleSendMessage(message, ok) {
				return
			}
		case <-ticker.C:
			if !c.sendPing() {
				return
			}
		}
	}
}

// handleSendMessage writes a message and any queued messages
func (c *Client) handleSendMessage(message []byte, ok bool) bool {
	c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if !ok {
		c.conn.WriteMessage(websocket.CloseMessage, []byte{})
		return false
	}
	w, err := c.conn.NextWriter(websocket.TextMessage)
	if err != nil {
		return false
	}
	w.Write(message)
	c.flushQueuedMessages(w)
	return w.Close() == nil
}

// flushQueuedMessages writes any additional queued messages
func (c *Client) flushQueuedMessages(w io.WriteCloser) {
	n := len(c.send)
	for i := 0; i < n; i++ {
		w.Write([]byte{'\n'})
		w.Write(<-c.send)
	}
}

// sendPing sends a ping message
func (c *Client) sendPing() bool {
	c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return c.conn.WriteMessage(websocket.PingMessage, nil) == nil
}

// handleWebSocket upgrades HTTP connections to WebSocket and registers clients.
// DEPRECATED: This function does not implement security measures.
// Use SecureWebSocketHandler instead for production deployments.
// This function lacks:
//   - Origin validation (allows all origins)
//   - Authentication checks
//   - Message rate limiting
//   - Message size limits
func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if GlobalHub == nil {
		http.Error(w, "WebSocket hub not initialized", http.StatusInternalServerError)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logging.Error("websocket upgrade failed", "error", err)
		return
	}

	client := &Client{
		hub:  GlobalHub,
		conn: conn,
		send: make(chan []byte, 256),
	}

	client.hub.register <- client

	// Start goroutines for reading and writing
	go client.writePump()
	go client.readPump()
}
