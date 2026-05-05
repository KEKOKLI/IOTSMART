package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	gorilla "github.com/gorilla/websocket"

	"iotsmart/backend/internal/models"
)

type Hub struct {
	logger   *log.Logger
	upgrader gorilla.Upgrader
	mu       sync.Mutex
	clients  map[*gorilla.Conn]struct{}
}

func NewHub(logger *log.Logger) *Hub {
	return &Hub{
		logger: logger,
		upgrader: gorilla.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		clients: make(map[*gorilla.Conn]struct{}),
	}
}

func (h *Hub) Handle(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Printf("upgrade websocket: %v", err)
		return
	}

	h.mu.Lock()
	h.clients[conn] = struct{}{}
	h.mu.Unlock()

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			h.remove(conn)
			return
		}
	}
}

func (h *Hub) Broadcast(reading models.TelemetryRecord) {
	payload, err := json.Marshal(map[string]any{
		"type":    "telemetry",
		"payload": reading,
	})
	if err != nil {
		h.logger.Printf("marshal websocket payload: %v", err)
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	for conn := range h.clients {
		conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		if err := conn.WriteMessage(gorilla.TextMessage, payload); err != nil {
			h.logger.Printf("broadcast websocket message: %v", err)
			conn.Close()
			delete(h.clients, conn)
		}
	}
}

func (h *Hub) remove(conn *gorilla.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	conn.Close()
	delete(h.clients, conn)
}
