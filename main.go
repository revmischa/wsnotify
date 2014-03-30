package main

import (
	"database/sql"
	"fmt"
	"github.com/garyburd/go-websocket/websocket"
	"github.com/lib/pq"
	"net/http"
	"strings"
	"time"
)

var listener *pq.Listener
var db *sql.DB

var publishers = struct {
	m           map[string]*publisher
	subscribe   chan *subscribeRequest
	get         chan *publisherRequest
	unsubscribe chan *subscribeRequest
}{
	m:           make(map[string]*publisher),
	subscribe:   make(chan *subscribeRequest),
	get:         make(chan *publisherRequest),
	unsubscribe: make(chan *subscribeRequest),
}

type publisherRequest struct {
	chanName string
	response chan *publisher
}

type subscribeRequest struct {
	chanName string
	c        *client
	done     chan int
}

func publishersDaemon() {
	for {
		select {
		case r := <-publishers.get:
			p := publishers.m[r.chanName]
			r.response <- p
		case r := <-publishers.subscribe:
			p, ok := publishers.m[r.chanName]
			if ok {
				p.subscribe <- r
			} else {
				newPub := newPublisher(r.chanName)
				publishers.m[r.chanName] = newPub
				listener.Listen(r.chanName)
				newPub.subscribe <- r
			}
		case r := <-publishers.unsubscribe:
			listener.Unlisten(r.chanName)
			p, ok := publishers.m[r.chanName]
			if ok {
				p.unsubscribe <- r
				numClients := <-r.done
				fmt.Println(numClients)
				if numClients < 1 {
					listener.Unlisten(r.chanName)
					delete(publishers.m, r.chanName)
				}
			}
		}
	}
}

func pgListen() {
	for notif := range listener.Notify {
		fmt.Println(notif.Channel)
		fmt.Println(notif.Extra)
		pr := &publisherRequest{
			chanName: notif.Channel,
			response: make(chan *publisher),
		}
		publishers.get <- pr
		pub := <-pr.response
		if pr != nil {
			pub.publish <- []byte(notif.Extra)
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
	sr := &subscribeRequest{chanName: pgChanName, c: c, done: make(chan int)}
	publishers.subscribe <- sr
	<-sr.done
	c.ws.WriteMessage(websocket.TextMessage, []byte("now subscribed to channel "+pgChanName))
	go c.writer()
	defer func() {
		sr := &subscribeRequest{chanName: pgChanName, c: c, done: make(chan int)}
		publishers.unsubscribe <- sr
		c.ws.Close()
		fmt.Println("Exiting client connection")
	}()
	c.reader()
}

func main() {
	config, err := GetConfig()
	if err != nil {
		panic(err)
	}
	configstr := ConfigString(config)
	db, err = sql.Open("postgres", configstr)

	reportProblem := func(ev pq.ListenerEventType, err error) {
		if err != nil {
			fmt.Println("PGListen error")
			fmt.Println(err.Error())
		}
	}
	go publishersDaemon()
	listener = pq.NewListener(configstr, 10*time.Second, time.Minute, reportProblem)
	go pgListen()
	fmt.Println("Listening")
	http.HandleFunc("/wsnotify/", newClientHandler)
	err = http.ListenAndServe(":"+config.Port, nil)
	if err != nil {
		panic("ListenAndServe: " + err.Error())
	}
}
