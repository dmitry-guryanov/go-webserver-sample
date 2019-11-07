package wsnotify

import (
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"

	"github.com/dmitry-guryanov/go-webserver-sample/internal/pkg/log"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type client struct {
	hub    *Hub
	conn   *websocket.Conn
	send   chan []byte
	userID int
}

func (c *client) readPump() {
	defer func() {
		c.hub.unregister <- c
		if err := c.conn.Close(); err != nil {
			log.L(c.hub.ctx).WithError(err).Error("error closing websocket connection")
		}
	}()
	c.conn.SetReadLimit(maxMessageSize)
	if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		log.L(c.hub.ctx).WithError(err).Error("conn.SetReadDeadline returned error")
	}
	c.conn.SetPongHandler(func(string) error {
		if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
			log.L(c.hub.ctx).WithError(err).Error("conn.SetReadDeadline returned error")
		}
		return nil
	})
	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				log.L(c.hub.ctx).WithError(err).Error("websocket close unexpectedly")
			}
			break
		}

		/* FIXME: handle message */
	}
}

func (c *client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		if err := c.conn.Close(); err != nil {
			log.L(c.hub.ctx).WithError(err).Error("error closing websocket connection")
			return
		}
	}()

	for {
		select {
		case message, ok := <-c.send:
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				log.L(c.hub.ctx).WithError(err).Error("conn.SetWriteDeadline returned error")
			}

			if !ok {
				// The hub closed the channel.
				log.L(c.hub.ctx).Info("Server closed WebSocket connection")
				if err := c.conn.WriteMessage(websocket.CloseMessage, []byte{}); err != nil {
					log.L(c.hub.ctx).WithError(err).Error("error writing close message to the websocket")
				}
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				log.L(c.hub.ctx).WithError(err).Error("conn.NextWriter returned error")
				return
			}

			if _, err := w.Write(message); err != nil {
				log.L(c.hub.ctx).WithError(err).Error("error writing to websocket")
				return
			}

			if err := w.Close(); err != nil {
				log.L(c.hub.ctx).WithError(err).Error("Error closing WebSocket writer")
				return
			}
		case <-ticker.C:
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				log.L(c.hub.ctx).WithError(err).Error("SetWriteDeadline returned error")
				return
			}
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.L(c.hub.ctx).WithError(err).Error("conn.WriteMessage returned error")
				return
			}
		}
	}
}

// ServeWs handles websocket requests from the peer.
func (hub *Hub) ServeWs(w http.ResponseWriter, r *http.Request, userID int) error {
	header := http.Header{}
	header.Add("Sec-Websocket-Protocol", "chat")

	conn, err := upgrader.Upgrade(w, r, header)
	if err != nil {
		return errors.Wrapf(err, "upgrader.Upgrade")
	}
	c := &client{
		hub:    hub,
		conn:   conn,
		userID: userID,
		send:   make(chan []byte, 256),
	}
	c.hub.register <- c

	go c.writePump()
	go c.readPump()

	return nil
}
