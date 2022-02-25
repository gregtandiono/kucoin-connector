 # Kucoin Connector

 This is a websocket server that connects to the Kucoin exchange API. This server subscribes to ticker and kline (candles) endpoint

## Getting Started

Start the server. Default port is 3000.
 ```bash
 go run main.go
 ```

Websocket endpoint: 
```
ws://localhost:3000/ws
```

You can subscribe to 2 topics: `ticker` and `kline`. Send the following JSON to the server:
```json
{"type": "ticker"}
```

You will immediately see ticker data coming in.

Connecting as a client example:
```golang
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
		"type": "ticker",
	}`)

	c.WriteMessage(websocket.TextMessage, subscriptionRequest)

	for data := range m {
		log.Printf("recv: %s", data)
	}
}
```

## Notes 
1. Sometimes it takes a few seconds to start the server because the server needs to fetch the symbols list, and the kucoin REST API is a bit unstable at times.
2. Seem like you can only see the kline data coming in if a client has subscribed to the ticker topic beforehand. The workaround:
```golang
		case "kline":
			go func() {
				for {
					select {
					case <-client.payload.Ticker:
					case d := <-client.payload.Kline:
						for c := range client.pool.clients {
							if c.topic == subscriptionType {
								c.mu.Lock()
								c.conn.WriteJSON(d)
								c.mu.Unlock()
							}
						}
					}
				}
			}()
```


