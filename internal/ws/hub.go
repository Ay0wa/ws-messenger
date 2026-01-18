package ws

import (
	"sync"

	"github.com/google/uuid"
)

type Hub struct {
	mu    sync.RWMutex
	chats map[uuid.UUID]map[*Client]struct{}
}

func NewHub() *Hub {
	return &Hub{chats: make(map[uuid.UUID]map[*Client]struct{})}
}

func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	clients, ok := h.chats[c.ChatID]
	if !ok {
		clients = make(map[*Client]struct{})
		h.chats[c.ChatID] = clients
	}
	clients[c] = struct{}{}
}

func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	clients, ok := h.chats[c.ChatID]
	if !ok {
		return
	}
	delete(clients, c)
	if len(clients) == 0 {
		delete(h.chats, c.ChatID)
	}
}

func (h *Hub) Broadcast(chatID uuid.UUID, msg []byte) {
	h.mu.RLock()
	clients := h.chats[chatID]
	h.mu.RUnlock()

	for c := range clients {
		select {
		case c.Send <- msg:
		default:
		}
	}
}
