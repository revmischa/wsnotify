package main

import (
	"database/sql"
	"fmt"
	"github.com/garyburd/go-websocket/websocket"
	"github.com/lib/pq"
	"io/ioutil"
	"launchpad.net/goyaml"
	"net/http"
	"strings"
	"time"
)

type client struct {
	ws     websocket.Conn
	send   chan []byte
	pgChan string
	done   chan bool
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

	name string
}

type publisherRequest struct {
	request  string
	response chan *publisher
}

//I don't like globals
//think about a way to get rid of this
var publishers = struct {
	m           map[string]*publisher
	getOrCreate chan *publisherRequest
	remove      chan string
}{
	m:           make(map[string]*publisher),
	getOrCreate: make(chan *publisherRequest),
	remove:      make(chan string),
}

func publishersDaemon() {
	for {
		select {
		case r := <-publishers.getOrCreate:
			p, ok := publishers.m[r.request]
			if ok {
                fmt.Println("think I found an existing publisher")
				r.response <- p
			} else {
				newPub := newPublisher(r.request)
				publishers.m[r.request] = newPub
				listener.Listen(r.request)
				r.response <- newPub 
			}
		case k := <-publishers.remove:
            delete(publishers.m, k)
            listener.Unlisten(k)
		}
	}
}

var listener *pq.Listener
var db *sql.DB

func newPublisher(name string) *publisher {
	p := publisher{
		subscribers: make(map[*client]bool),
		publish:     make(chan []byte),
		subscribe:   make(chan *client),
		unsubscribe: make(chan *client),
		name:        name,
	}
	publishers.m[name] = &p
	go p.run()

	return &p
}

func (c *client) reader() {
	for {
		mt, p, err := c.ws.ReadMessage()
		if err != nil {
			fmt.Println("client read error: ", err.Error())
			return
		}
		if mt == websocket.CloseMessage {
			return
		}

		if mt == websocket.TextMessage {
			db.Exec("SELECT pg_Notify($1, $2)", c.pgChan, string(p))
		}
		fmt.Println("type", mt, "message", p)
	}
}

func (c *client) writer() {
	defer fmt.Println("exiting client writer")
	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				// the channel's been closed
				return
			}
			err := c.ws.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				fmt.Println("Error writing " + string(message) + " to websocket: " + err.Error())
				return
			}
		case <-time.After(20 * time.Second):
			c.ws.WriteMessage(websocket.PingMessage, []byte(""))
		}
	}
}

func (p *publisher) run() {
    running := true
	for running {
		select {
		case c := <-p.subscribe:
			p.subscribers[c] = true
		case c := <-p.unsubscribe:
			delete(p.subscribers, c)
			close(c.send)
			if len(p.subscribers) < 1 {
                publishers.remove <- p.name
                running = false
			}
		case m := <-p.publish:
			for c := range p.subscribers {
				select {
				case c.send <- m:
				default:
					delete(p.subscribers, c)
					close(c.send)
					go c.ws.Close()
				}
			}
		}
	}
}

func pgListen() {
	for notif := range listener.Notify {
		fmt.Println(notif.Channel)
		fmt.Println(notif.Extra)
		if publishers.m[notif.Channel] != nil {
			publishers.m[notif.Channel].publish <- []byte(notif.Extra)
		}
	}

	fmt.Printf("Lost database connection ?!")
}

func newClientHandler(w http.ResponseWriter, r *http.Request) {
	ws, err := websocket.Upgrade(w, r.Header, nil, 1024, 1024)
	if _, ok := err.(websocket.HandshakeError); ok {
		http.Error(w, "Not a websocket handshake", 400)
		return
	} else if err != nil {
		fmt.Println(err)
		return
	}
	path := r.URL.Path
	fmt.Println(path)
	pathParts := strings.Split(path, "/")
	pgChanName := pathParts[len(pathParts)-1]
	fmt.Println(pgChanName)
	c := &client{*ws, make(chan []byte, 256), pgChanName, make(chan bool)}
    pr := &publisherRequest{ request: pgChanName, response: make(chan *publisher),}
    publishers.getOrCreate <- pr
	p := <- pr.response
	p.subscribe <- c
	c.ws.WriteMessage(websocket.TextMessage, []byte("now subscribed to channel "+pgChanName))
	go c.writer()
	defer func() {
		p.unsubscribe <- c
		fmt.Println("Exiting client connection")
	}()
	c.reader()
}

type DBconfig struct {
	User     string
	DBname   string
	Port     string
	Password string
	Host     string
}

func main() {
	var err error
	configFile, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		configFile, err = ioutil.ReadFile("/etc/wsnotify/config.yaml")
		if err != nil {
			panic("could not find config file")
		}
	}
	var config DBconfig
	goyaml.Unmarshal(configFile, &config)
	configString := "user=" + config.User + " dbname=" + config.DBname + " sslmode=disable"
	if config.Host != "" {
		configString = configString + " host=" + config.Host
	}
	if config.Password != "" {
		configString = configString + " password=" + config.Password
	}
	db, err = sql.Open("postgres", configString)

	reportProblem := func(ev pq.ListenerEventType, err error) {
		if err != nil {
			fmt.Println("reportProblem error")
			fmt.Println(err.Error())
		}
	}
    go publishersDaemon()
	listener = pq.NewListener(configString, 10*time.Second, time.Minute, reportProblem)
	go pgListen()
	fmt.Println("Listening")
	http.HandleFunc("/wsnotify/", newClientHandler)
	err = http.ListenAndServe(":"+config.Port, nil)
	if err != nil {
		panic("ListenAndServe: " + err.Error())
	}
}
