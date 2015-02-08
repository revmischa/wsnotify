package main

import (
	"io"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message.
	pongWait = 45 * time.Second

	// Send pings to client with this time.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed.
	maxMessageSize = 512
)

type client struct {
	ws     websocket.Conn
	send   chan []byte
	pgChan string
	done   chan bool
}

func (c *client) write(mt int, message []byte) error {
	c.ws.SetWriteDeadline(time.Now().Add(writeWait))
	return c.ws.WriteMessage(mt, message)
}

func (c *client) reader() {
	c.ws.SetReadLimit(maxMessageSize)
	c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error { c.ws.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		mt, p, err := c.ws.ReadMessage()

		if err != nil && err != io.EOF {
			//TODO figure out what the EOF error type for gorilla websocket is
			//log.Error("client read error: ", err.Error())
			return
		}

		if mt == websocket.CloseMessage {
			return
		}

		if mt == websocket.TextMessage {
			//TODO: fix this so client does not need to know about DB
			db.Exec("SELECT pg_Notify($1, $2)", c.pgChan, string(p))
		}
	}
}

func (c *client) writer() {
	//defer log.Debug("exiting client writer")
	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				// the channel's been closed
				c.write(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.write(websocket.TextMessage, message); err != nil {
				log.Error("Error writing " + string(message) + " to websocket: " + err.Error())
				return
			}
		case <-time.After(20 * time.Second):
			if err := c.write(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}
