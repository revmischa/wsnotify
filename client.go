package main

import (
	"time"

	"github.com/garyburd/go-websocket/websocket"
)

type client struct {
	ws     websocket.Conn
	send   chan []byte
	pgChan string
	done   chan bool
}

func (c *client) reader() {
	for {
		mt, p, err := c.ws.ReadMessage()
		if err != nil {
			log.Error("client read error: ", err.Error())
			return
		}
		if mt == websocket.CloseMessage {
			return
		}

		if mt == websocket.TextMessage {
			db.Exec("SELECT pg_Notify($1, $2)", c.pgChan, string(p))
		}
		//fmt.Println("type", mt, "message", p)
	}
}

func (c *client) writer() {
	defer log.Notice("exiting client writer")
	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				// the channel's been closed
				return
			}
			err := c.ws.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				log.Error("Error writing " + string(message) + " to websocket: " + err.Error())
				return
			}
		case <-time.After(20 * time.Second):
			c.ws.WriteMessage(websocket.PingMessage, []byte(""))
		}
	}
}
