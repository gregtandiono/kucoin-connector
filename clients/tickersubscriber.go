package main

import (
	"flag"
	"log"
	"net/url"

	"github.com/gorilla/websocket"
)

func main() {
	var addr = flag.String("addr", "localhost:3000", "http service address")
	u := url.URL{Scheme: "ws", Host: *addr, Path: "/ws"}
	log.Printf("connecting to %s", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	done := make(chan struct{})
	m := make(chan []byte)

	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			}
			m <- message
		}
	}()

	subscriptionRequest := []byte(`{
		"type": "ticker"
	}`)

	c.WriteMessage(websocket.TextMessage, subscriptionRequest)

	for data := range m {
		log.Printf("recv: %s", data)
	}
}
