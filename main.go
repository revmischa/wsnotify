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

type publisher struct {
	// Registered connections.
	subscribers map[*client]bool

	// Notifs
	publish chan []byte

	// Register requests from the connections.
	subscribe chan *client

	// Unregister requests from connections.
	unsubscribe chan *client

    //Last message sent for newly connecting clients
    lastMessage []byte

    name string
    db *sql.DB
}

func newPublisher(db *sql.DB, name string) *publisher {
    p := publisher {
        subscribers: make(map [*client]bool),
        publish: make(chan []byte),
        subscribe: make(chan *client),
        unsubscribe: make(chan *client),
        lastMessage: make([]byte, 0),
        name: name,
        db: db,
    }
    go p.run()
    go p.pgListen()

    return &p 
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
func (p *publisher) run() {
	for {
		select {
		case c := <-p.subscribe:
			p.subscribers[c] = true
            if (len(p.lastMessage) > 0){
                fmt.Println(string(p.lastMessage))
                c.send <- p.lastMessage
            }
		case c := <-p.unsubscribe:
			delete(p.subscribers, c)
			close(c.send)
		case m := <-p.publish:
			for c := range p.subscribers {
				c.send <- m
			}
            p.lastMessage = m
            fmt.Println(string(p.lastMessage))
		}
	}
}

func (p *publisher) pgListen() {
	notifications, err := p.db.Query("LISTEN " + p.name)
	if err != nil {
		fmt.Printf("Could not listen to mychan: %s\n", err)
		return
	}
	defer notifications.Close()

	//wg.Done()

	// tell main() it's okay to spawn the pgnotifier goroutine

	var msg string
	for notifications.Next() {
        fmt.Println("got a notification")
		if err = notifications.Scan(&msg); err != nil {
			fmt.Printf("Error while scanning: %s\n", err)
			continue
		}
		p.publish <- []byte(msg)
	}

	fmt.Printf("Lost database connection ?!")
}

func notify(db *sql.DB) {
	for i := 0; i < 1000; i++ {
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
	//wg.Add(1)
    go n.pglistener(db, &wg)
    go n.run()
    //wg.Wait()
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
