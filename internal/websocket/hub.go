package wshub

import (
	"encoding/json"
	"sync"
)

type Event struct {
	Type      string      `json:"type"`
	Channel   string      `json:"channel"`
	Data      interface{} `json:"data"`
	Timestamp string      `json:"timestamp"`
}

// Channels:
//   dashboard           -> Manajemen & Super Admin
//   admin               -> Admin Sales
//   operator            -> Operator Lapangan
//   customer:<user_id>  -> Customer tertentu

type Client struct {
	ID      string
	Channel string
	Send    chan []byte
}

type Hub struct {
	mu       sync.RWMutex
	channels map[string]map[*Client]bool
}

var Default *Hub

func InitDefault() {
	Default = &Hub{channels: map[string]map[*Client]bool{}}
}

func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.channels[c.Channel]; !ok {
		h.channels[c.Channel] = map[*Client]bool{}
	}
	h.channels[c.Channel][c] = true
}

func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if m, ok := h.channels[c.Channel]; ok {
		delete(m, c)
		close(c.Send)
		if len(m) == 0 {
			delete(h.channels, c.Channel)
		}
	}
}

func (h *Hub) Publish(channel string, evt Event) {
	payload, err := json.Marshal(evt)
	if err != nil {
		return
	}
	h.mu.RLock()
	clients := h.channels[channel]
	subs := make([]*Client, 0, len(clients))
	for c := range clients {
		subs = append(subs, c)
	}
	h.mu.RUnlock()
	for _, c := range subs {
		select {
		case c.Send <- payload:
		default:
		}
	}
}

func (h *Hub) Broadcast(channels []string, evt Event) {
	for _, ch := range channels {
		h.Publish(ch, evt)
	}
}
