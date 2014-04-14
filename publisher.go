package main

import "fmt"

type publisher struct {
	// Registered connections.
	subscribers map[*client]bool

	// Notifs
	publish chan []byte

	// Register requests from the connections.
	subscribe chan *subscribeRequest

	// Unregister requests from connections.
	unsubscribe chan *subscribeRequest

	name string
}

func newPublisher(name string) *publisher {
	p := publisher{
		subscribers: make(map[*client]bool),
		publish:     make(chan []byte),
		subscribe:   make(chan *subscribeRequest),
		unsubscribe: make(chan *subscribeRequest),
		name:        name,
	}
	go p.run()

	return &p
}

func (p *publisher) run() {
	running := true
	for running {
		select {
		case r := <-p.subscribe:
			p.subscribers[r.c] = true
			r.done <- len(p.subscribers)
		case r := <-p.unsubscribe:
			fmt.Println("entering unsubscribe for" + p.name)
			delete(p.subscribers, r.c)
			close(r.c.send)
			r.done <- len(p.subscribers)
			if len(p.subscribers) < 1 {
				fmt.Println("Setting running to false")
				running = false
			}
		case m := <-p.publish:
			for c := range p.subscribers {
				select {
				case c.send <- m:
				default:
					delete(p.subscribers, c)
					close(c.send)
				}
			}
		}
	}
}
