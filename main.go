package main

import (
	"database/sql"
	"fmt"
	stdlog "log"
	stdsyslog "log/syslog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/lib/pq"
	"github.com/op/go-logging"
)

var listener *pq.Listener
var log = logging.MustGetLogger("wsnotify")
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

// function handles the channel connections
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
			p, ok := publishers.m[r.chanName]
			if ok {
				p.unsubscribe <- r
				numClients := <-r.done
				//log.Debug(string(numClients))
				if numClients < 1 {
					listener.Unlisten(r.chanName)
					delete(publishers.m, r.chanName)
				}
			}
		}
	}
}

// function to get notification from PG 
func pgListen() {
	for notif := range listener.Notify {
		//log.Debug("message recieved")
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

	log.Error("Lost database connection ?!")
}

// this function subscribs to channel , the channel send by application like pandapops
func newClientHandler(w http.ResponseWriter, r *http.Request) {
	ws, err := websocket.Upgrade(w, r, nil, 1024, 1024)
	if _, ok := err.(websocket.HandshakeError); ok {
		http.Error(w, "Not a websocket handshake", 400)
		return
	} else if err != nil {
		log.Error(err.Error())
		return
	}
	path := r.URL.Path
	//log.Debug(path)
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
		//log.Debug("Exiting client connection")
	}()
	c.reader()
}

func main() {

	syslog, err := stdsyslog.New(stdsyslog.LOG_LOCAL0|stdsyslog.LOG_DEBUG, "wsnotify")
	if err != nil {
		log.Fatal(err)
	}

	syslogBackend := logging.SyslogBackend{syslog}
	stdOutBackend := logging.NewLogBackend(os.Stderr, "", stdlog.LstdFlags|stdlog.Lshortfile)
	if err != nil {
		log.Fatal(err)
	}

	logging.SetBackend(&syslogBackend, stdOutBackend)
	config, err := GetConfig()

	if err != nil {
		log.Fatal(err)
	}

	// connect to PG
	configstr := ConfigString(config)
	db, err = sql.Open("postgres", configstr)

	reportProblem := func(ev pq.ListenerEventType, err error) {
		if err != nil {
			log.Error("PGListen error " + err.Error())
		}
	}

	// create publisher channel
	go publishersDaemon()

	listener = pq.NewListener(configstr, 10*time.Second, time.Minute, reportProblem)

	// start listening to PG
	go pgListen()

	fmt.Println("Listening")   // bogus

	http.HandleFunc("/wsnotify/", newClientHandler)

	err = http.ListenAndServe(":"+config.Port, nil)
	if err != nil {
		log.Fatal("ListenAndServe: " + err.Error())
	}
}
