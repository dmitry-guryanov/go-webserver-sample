package wsnotify

import (
	"context"
	"encoding/json"

	"github.com/sirupsen/logrus"

	"github.com/dmitry-guryanov/go-webserver-sample/internal/pkg/log"
)

type hubMsg struct {
	userID int
	msg    []byte
}

// Hub maintains the set of active clients and broadcasts messages to the
// clients.
type Hub struct {
	ctx      context.Context
	cancel   context.CancelFunc
	doneChan chan struct{}

	// Registered clients.
	clients map[*client]bool

	// Inbound messages from the clients.
	broadcast chan hubMsg

	// Register requests from the clients.
	register chan *client

	// Unregister requests from clients.
	unregister chan *client
}

// NewHub creates new hub instance
func NewHub() *Hub {
	ctx, cancel := context.WithCancel(context.Background())
	ctx = log.WithLogger(ctx, logrus.WithField("from", "wsnotify"))

	return &Hub{
		ctx:        ctx,
		cancel:     cancel,
		doneChan:   make(chan struct{}),
		broadcast:  make(chan hubMsg),
		register:   make(chan *client),
		unregister: make(chan *client),
		clients:    make(map[*client]bool),
	}
}

// Run starts processing messages. For now hub only sends messages to the
// clients.
func (h *Hub) Run() {
Loop:
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
		case hmsg := <-h.broadcast:
			for client := range h.clients {
				if hmsg.userID != client.userID {
					continue
				}

				select {
				case client.send <- hmsg.msg:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		case <-h.ctx.Done():
			break Loop
		}
	}

	close(h.doneChan)
}

// Close stop serving websockets
// TODO: terminate client connections?
func (h *Hub) Close() error {
	h.cancel()
	<-h.doneChan
	return nil
}

type message struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// Send sends message to all users. msg must be a struct, not pointer.
func (h *Hub) Send(msg interface{}, userID int) {
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		log.L(h.ctx).WithError(err).Fatal()
	}
	hmsg := hubMsg{
		userID: userID,
		msg:    msgBytes,
	}
	h.broadcast <- hmsg
}

// UserHub is a hub, which sends messages only to the user with given ID
// and to the admin user.
type UserHub struct {
	hub    *Hub
	userID int
}

// NewUserHub creates a new instance of UserHub. It's actually not a new hub, but
// a struct which points to the hub 'h'.
func (h *Hub) NewUserHub(userID int) *UserHub {
	return &UserHub{
		hub:    h,
		userID: userID,
	}
}

// Send sends given message to the user with userID of this hub and to the admin.
func (uh *UserHub) Send(msg interface{}) {
	uh.hub.Send(msg, uh.userID)
}
