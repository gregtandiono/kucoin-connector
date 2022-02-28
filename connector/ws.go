package connector

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type WSPayload struct {
	Topic  chan string
	Kline  chan Kline
	Ticker chan Ticker
}

type TopicSubscription struct {
	Type string `json:"type"` // ticker
}
type Client struct {
	pool *Pool

	conn *websocket.Conn

	mu sync.Mutex

	payload WSPayload

	receiver chan []byte

	topic string
}

type Pool struct {
	clients map[*Client]bool

	register chan *Client

	unregister chan *Client
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func NewPool() *Pool {
	return &Pool{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (p *Pool) Run() {
	for {
		select {
		case client := <-p.register:
			p.clients[client] = true
		case client := <-p.unregister:
			delete(p.clients, client)
		}
	}
}

func (client *Client) Write() {
	for m := range client.receiver {
		// handle client disconnection
		if string(m) == "close" {
			log.Println("Unregistering client:")
			client.pool.unregister <- client
			client.conn.Close()
		}

		var t TopicSubscription
		json.Unmarshal(m, &t)

		client.topic = t.Type

		switch subscriptionType := t.Type; subscriptionType {
		case "ticker":
			go func() {
				for {
					select {
					case <-client.payload.Kline:
					case d := <-client.payload.Ticker:
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
		}
	}
}

func InitListener(c *websocket.Conn, receiver chan []byte) {
	defer func() {
		receiver <- []byte("close")
	}()
	ticker := time.NewTicker(150 * time.Millisecond) // staggering the reads so I can see the terminal logs better
	for {
		<-ticker.C
		_, message, err := c.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
		receiver <- message
	}
}

func ServeWs(pool *Pool, p WSPayload, w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		if _, ok := err.(websocket.HandshakeError); !ok {
			log.Println(err)
		}
		return
	}

	receiver := make(chan []byte)

	client := &Client{
		pool:     pool,
		conn:     ws,
		payload:  p,
		receiver: receiver,
	}

	pool.register <- client

	go InitListener(ws, receiver)
	go client.Write()
}
