package sse

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

type Broker struct {
	clients    map[string]map[chan string]bool
	mu         sync.RWMutex
	register   chan *ClientChan
	unregister chan *ClientChan
}

type ClientChan struct {
	RoomCode string
	Channel  chan string
}

type Event struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

func NewBroker() *Broker {
	b := &Broker{
		clients:    make(map[string]map[chan string]bool),
		register:   make(chan *ClientChan),
		unregister: make(chan *ClientChan),
	}
	go b.run()
	return b
}

func (b *Broker) run() {
	for {
		select {
		case client := <-b.register:
			b.mu.Lock()
			if b.clients[client.RoomCode] == nil {
				b.clients[client.RoomCode] = make(map[chan string]bool)
			}
			b.clients[client.RoomCode][client.Channel] = true
			b.mu.Unlock()

		case client := <-b.unregister:
			b.mu.Lock()
			if clients, ok := b.clients[client.RoomCode]; ok {
				delete(clients, client.Channel)
				close(client.Channel)
			}
			b.mu.Unlock()
		}
	}
}

func (b *Broker) Subscribe(roomCode string) (<-chan string, func()) {
	ch := make(chan string, 100)
	client := &ClientChan{RoomCode: roomCode, Channel: ch}

	b.register <- client

	cleanup := func() {
		b.unregister <- client
	}

	return ch, cleanup
}

func (b *Broker) Broadcast(roomCode string, eventType string, payload interface{}) {
	event := Event{Type: eventType, Payload: payload}
	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	if clients, ok := b.clients[roomCode]; ok {
		for ch := range clients {
			ch <- string(data)
		}
	}
}

func (b *Broker) BroadcastToHost(roomCode string, eventType string, payload interface{}) {
	b.Broadcast(roomCode, eventType, payload)
}

func (b *Broker) ServeHTTP(w http.ResponseWriter, r *http.Request, roomCode string) {
	log.Printf("SSE: Starting for room %s", roomCode)

	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Printf("SSE: Flusher not supported")
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}
	log.Printf("SSE: Flusher OK")

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch, cleanup := b.Subscribe(roomCode)
	defer cleanup()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	ctx := r.Context()

	log.Printf("SSE: Entering loop for room %s", roomCode)
	for {
		select {
		case <-ctx.Done():
			log.Printf("SSE: Context done for room %s", roomCode)
			return
		case data, ok := <-ch:
			if !ok {
				log.Printf("SSE: Channel closed for room %s", roomCode)
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-ticker.C:
			fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}
