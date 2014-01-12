package main

import (
    "code.google.com/p/go.net/websocket"
	"database/sql"
	"fmt"
	_ "github.com/jgallagher/go-libpq"
	"net/http"
    "sync"
)

type client struct {
    ws websocket.Conn	
	send chan []byte
	done chan bool
}

type notifier struct {
	// Registered connections.
	connections map[*client]bool

	// Notifs
	broadcast chan []byte

	// Register requests from the connections.
	register chan *client

	// Unregister requests from connections.
	unregister chan *client

    //Last message sent for newly connecting clients
    lastMessage []byte
}

var n = notifier{
    broadcast: make(chan []byte),
    register: make(chan *client),
    unregister: make(chan *client),
    connections: make(map[*client]bool),
    lastMessage: make([]byte, 0),
}

func (c *client) writer() {
    for message := range c.send {
        _, err := c.ws.Write(message)
        if err != nil {
            break
        }
    }
    c.ws.Close()
}
func (n *notifier) run() {
	for {
		select {
		case c := <-n.register:
			n.connections[c] = true
            if (len(n.lastMessage) > 0){
                fmt.Println(string(n.lastMessage))
                c.send <- n.lastMessage
            }
		case c := <-n.unregister:
			delete(n.connections, c)
			close(c.send)
		case m := <-n.broadcast:
			for c := range n.connections {
				c.send <- m
			}
            n.lastMessage = m
            fmt.Println(string(n.lastMessage))
		}
	}
}

func (n *notifier) pglistener(db *sql.DB, wg *sync.WaitGroup) {
	notifications, err := db.Query("LISTEN mychan")
	if err != nil {
		fmt.Printf("Could not listen to mychan: %s\n", err)
		return
	}
	defer notifications.Close()

	wg.Done()
	// tell main() it's okay to spawn the pgnotifier goroutine

	var msg string
	for notifications.Next() {
        fmt.Println("got a notification")
		if err = notifications.Scan(&msg); err != nil {
			fmt.Printf("Error while scanning: %s\n", err)
			continue
		}
		n.broadcast <- []byte(msg)
	}

	fmt.Printf("Lost database connection ?!")
}

func notify(db *sql.DB) {
	for i := 0; i < 10; i++ {
		// WARNING: Postgres does not appear to support parameterized notifications
		//          like "NOTIFY mychan, $1". Be careful not to expose SQL injection!
		query := fmt.Sprintf("NOTIFY mychan, 'message-%d'", i)
		if _, err := db.Exec(query); err != nil {
			fmt.Printf("error sending NOTIFY: %s\n", err)
		}
	}
    fmt.Println("exiting notify")
}

func newClientHandler(ws *websocket.Conn) {
    c := &client{ *ws, make(chan []byte, 256), make(chan bool)}
    n.register <- c
    c.ws.Write([]byte("Hello"))
    defer func() { n.unregister <- c }()
    fmt.Print("got here")
    c.writer()
}

func startServer(db *sql.DB) {
	fmt.Println("Listening")
	http.Handle("/echo", websocket.Handler(newClientHandler))
	var wg sync.WaitGroup
	wg.Add(1)
    go n.pglistener(db, &wg)
    go n.run()
    wg.Wait()
    notify(db)
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		panic("ListenAndServe: " + err.Error())
	}
}

func main() {
	db, err := sql.Open("libpq", "") // assuming localhost, user ok, etc
	if err != nil {
		fmt.Printf("could not connect to postgres: %s\n", err)
		return
	}
	defer db.Close()
	startServer(db)
}
