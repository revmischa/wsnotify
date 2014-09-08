package main

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

func newClientHandler(w http.ResponseWriter, r *http.Request) {
	ws, err := websocket.Upgrade(w, r.Header, nil, 1024, 1024)
	if _, ok := err.(websocket.HandshakeError); ok {
		http.Error(w, "Not a websocket handshake", 400)
		return
	} else if err != nil {
		log.Error(err.Error())
		return
	}
	path := r.URL.Path
	log.Debug(path)
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
		log.Debug("Exiting client connection")
	}()
	c.reader()
}

func runHTTPServer(config *DBconfig) {
	var err error
	http.HandleFunc(config.ServerPath, newClientHandler)
	if config.SSLKeyPath != "" && config.SSLKeyPath != "" {
		err = http.ListenAndServeTLS(":"+config.Port, config.SSLCertPath, config.SSLKeyPath, nil)
	} else {
		err = http.ListenAndServe(":"+config.Port, nil)
	}
	if err != nil {
		log.Fatal("ListenAndServe: " + err.Error())
	}
}
