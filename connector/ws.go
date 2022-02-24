package connector

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type WSPayload struct {
	Topic  chan string
	Kline  chan Kline
	Ticker chan Ticker
}

type TopicSubscription struct {
	Type   string `json:"type"`   // ticker
	Symbol string `json:"symbol"` // symbol
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func InitListener(c *websocket.Conn, receiver chan []byte) {
	defer func() {
		c.Close()
	}()
	for {
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

func ServeWs(p WSPayload, w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		if _, ok := err.(websocket.HandshakeError); !ok {
			log.Println(err)
		}
		return
	}

	receiver := make(chan []byte)
	go InitListener(ws, receiver)

	for m := range receiver {
		var t TopicSubscription
		json.Unmarshal(m, &t)

		switch subscriptionType := t.Type; subscriptionType {
		case "ticker":
			topic := "/market/ticker:all"
			p.Topic <- topic
			go func() {
				for d := range p.Ticker {
					ws.WriteJSON(d)
				}
			}()
		case "kline":
			topic := fmt.Sprintf("/market/candles:%s_1min", t.Symbol)
			p.Topic <- topic
			if err != nil {
				ws.WriteMessage(websocket.TextMessage, []byte("Unable to subscribe to topic"))
			}
			go func() {
				for d := range p.Kline {
					ws.WriteJSON(d)
				}
			}()
		}
	}
}

func TopicManager(c *websocket.Conn, topic chan string) {
	for topic := range topic {
		log.Println("subscribing to topic ->", topic)
		id := uuid.NewString()
		err := ManageSubscription(c, topic, id, true)
		if err != nil {
			log.Println("Unable to subscribe to ticker", err)
			return
		}
	}
}
