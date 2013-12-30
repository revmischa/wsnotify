package main

import (
    "database/sql"
    "fmt"
    "io"
    _ "github.com/jgallagher/go-libpq"
    "code.google.com/p/go.net/websocket"
    "net/http"
)

type client struct {
    io.ReadWriteCloser
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
}

func (n *notifier) run() {
    for {
        select {
		case c := <-n.register:
			n.connections[c] = true
		case c := <-n.unregister:
			delete(n.connections, c)
			close(c.send)
		case m := <-n.broadcast:
			for c := range n.connections {
				c.send <- m
			}
		}
	}
}

func pglistener(db *sql.DB, messages chan string) {
    notifications, err := db.Query("LISTEN mychan")
    if err != nil {
        fmt.Printf("Could not listen to mychan: %s\n", err)
        close(messages)
        return
    }
    defer notifications.Close()

    // tell main() it's okay to spawn the pgnotifier goroutine

    var msg string
    for notifications.Next() {
        if err = notifications.Scan(&msg); err != nil {
            fmt.Printf("Error while scanning: %s\n", err)
            continue
        }
        messages <- msg
    }

    fmt.Printf("Lost database connection ?!")
    close(messages)
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
}

func newClientHandler(ws *websocket.Conn) {
}

func startServer() {
	fmt.Println("Listening")
	http.Handle("/echo", websocket.Handler(newClientHandler))
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
	startServer()
}
