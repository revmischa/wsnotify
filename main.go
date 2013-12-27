package main

import (
    "database/sql"
    "fmt"
    "io"
    _ "github.com/jgallagher/go-libpq"
    "code.google.com/p/go.net/websocket"
    "net/http"
)

type Client struct {
    io.ReadWriteCloser 
    done chan bool 
}

// figure out a better way to keep track of clients
var globalClients = make(map[string]*Client)


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

func notifier(db *sql.DB) {
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
    msg := []byte("foo")
    ws.Write(msg)
}

func startServer(){
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
    go startServer()
    /*messages := make(chan string)
    var wg sync.WaitGroup
    wg.Add(1)
    go pglistener(db, messages, &wg)

    // wait until LISTEN was issued, then spawn notifier goroutine
    wg.Wait()
    go notifier(db)

    for i := 0; i < 10; i++ {
        fmt.Printf("received notification %s\n", <-messages)
    }*/
}
