package main

import (
    "github.com/garyburd/go-websocket/websocket"
	"fmt"
	"github.com/lib/pq"
	"net/http"
    "launchpad.net/goyaml"
    "io/ioutil"
    "strings"
    "time"
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
}

//I don't like globals
//think about a way to get rid of this
var publishers = struct {
    m map[string]*publisher
}{
    m: make(map[string]*publisher),
}

var listener *pq.Listener

func newPublisher(name string) *publisher {
    p := publisher {
        subscribers: make(map [*client]bool),
        publish: make(chan []byte),
        subscribe: make(chan *client),
        unsubscribe: make(chan *client),
        lastMessage: make([]byte, 0),
        name: name,
    }
    publishers.m[name] = &p
    go p.run()

    return &p 
}

func (c *client) reader(){
    //takes messages, probably mostly for requesting to subscriptions
    for {
        mt, p, err := c.ws.ReadMessage() 
        if err != nil {
            fmt.Println(err.Error())
            return
        }
        fmt.Println(mt, p)
    }
}

func (c *client) writer() {
    for {
        select {
        case message := <- c.send : 
            err := c.ws.WriteMessage(websocket.TextMessage, message)
            if err != nil {
                fmt.Println("Error writing to websocket: " + err.Error())
                break
            }
        case <-time.After(20 * time.Second):
            c.ws.WriteMessage(websocket.PingMessage, []byte(""))
        }
    }
    fmt.Println("exiting client writer")
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
                select {
                case c.send <- m:
                default:
                    delete(p.subscribers, c)
                    close(c.send)
                    go c.ws.Close()
                }
			}
            p.lastMessage = m
            fmt.Println(string(p.lastMessage))
		}
	}
}

func pgListen() {
    for notif := range listener.Notify {
        fmt.Println(notif.Channel)
        fmt.Println(notif.Extra)
		publishers.m[notif.Channel].publish <- []byte(notif.Extra)
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
    pgChanName := pathParts[len(pathParts) -1]
    fmt.Println(pgChanName)
    c := &client{ *ws, make(chan []byte, 256), make(chan bool)}
    p := publishers.m[pgChanName]
    
    go c.reader()
    if (p == nil) {
        p = newPublisher(pgChanName)
    }

    p.subscribe <- c

    listener.Listen(pgChanName)
    c.ws.WriteMessage(websocket.TextMessage, []byte("now subscribed to channel " + pgChanName))
    defer func() { 
        listener.Unlisten(pgChanName)
        p.unsubscribe <- c 
        fmt.Println("Exiting client connection")
    }()
    c.writer()
}


type DBconfig struct {
    User string
    DBname string
    Port string
}

func main() {
    configFile, _ := ioutil.ReadFile("config.yaml") 
    var config DBconfig;
    goyaml.Unmarshal(configFile, &config)
    configString := "user=" + config.User + " dbname=" + config.DBname  + " sslmode=disable"

    reportProblem := func(ev pq.ListenerEventType, err error) {
        if err != nil {
            fmt.Println(err.Error())
        }
    }

    listener = pq.NewListener(configString, 10 * time.Second, time.Minute, reportProblem)
    go pgListen()
	fmt.Println("Listening")
	http.HandleFunc("/wsnotify/", newClientHandler)
    err := http.ListenAndServe(":" + config.Port, nil)
	if err != nil {
		panic("ListenAndServe: " + err.Error())
	}
}
