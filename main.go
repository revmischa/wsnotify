package main

import (
	"database/sql"
	"fmt"
	stdlog "log"
	stdsyslog "log/syslog"
	"os"
	"time"

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
				log.Debug(string(numClients))
				if numClients < 1 {
					log.Debug("Channel")
					listener.Unlisten(r.chanName)
					delete(publishers.m, r.chanName)
				}
			}
		}
	}
}

func pgListen() {
	for notif := range listener.Notify {
		log.Debug("message recieved")
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
	configstr := ConfigString(config)
	db, err = sql.Open("postgres", configstr)

	reportProblem := func(ev pq.ListenerEventType, err error) {
		if err != nil {
			log.Error("PGListen error " + err.Error())
		}
	}
	go publishersDaemon()
	listener = pq.NewListener(configstr, 10*time.Second, time.Minute, reportProblem)
	go pgListen()
	fmt.Println("Listening")
	runHTTPServer(config)
}
