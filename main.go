package main

import (
	"encoding/json"
	"fmt"
	"kucoin-ws-connector/connector"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type WSPayload struct {
	topic  chan string
	kline  chan connector.Kline
	ticker chan connector.Ticker
}

type TopicSubscription struct {
	Type   string `json:"type"`   // ticker
	Symbol string `json:"symbol"` // symbol
}

func serveWs(p WSPayload, w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		if _, ok := err.(websocket.HandshakeError); !ok {
			log.Println(err)
		}
		return
	}

	receiver := make(chan []byte)

	go func() {
		_, message, err := ws.ReadMessage()
		if err != nil {
			log.Printf("Unable to read message %s", err)
		}
		receiver <- message
	}()

	for m := range receiver {
		var t TopicSubscription
		json.Unmarshal(m, &t)

		switch subscriptionType := t.Type; subscriptionType {
		case "ticker":
			topic := "/market/ticker:all"
			p.topic <- topic
			go func() {
				for d := range p.ticker {
					ws.WriteJSON(d)
				}
			}()
		case "kline":
			topic := fmt.Sprintf("/market/candles:%s_1min", t.Symbol)
			p.topic <- topic
			if err != nil {
				ws.WriteMessage(websocket.TextMessage, []byte("Unable to subscribe to topic"))
			}
			go func() {
				for d := range p.kline {
					ws.WriteJSON(d)
				}
			}()
		}
	}
}

func main() {
	symbols := connector.GetAllKucoinSymbols()
	c, _, err := connector.CreateKucoinWSClient()
	if err != nil {
		log.Fatal("Unable to dial into exchange ws:", err)
	}
	defer c.Close()

	trade := make(chan []byte)
	rawTickerData := make(chan []byte)
	rawKlineData := make(chan []byte)
	tickerPayload := make(chan connector.Ticker)
	topic := make(chan string)
	klinePayload := make(chan connector.Kline)

	go func() {
		for topic := range topic {
			log.Println("subscribing to topic ->", topic)
			id := uuid.NewString()
			err := connector.ManageSubscription(c, topic, id, true)
			if err != nil {
				log.Println("Unable to subscribe to ticker", err)
				return
			}
		}
	}()

	// Message Reader & Handler
	go connector.InitKucoinListener(c, trade)

	// filter
	go func() {
		for t := range trade {
			var data struct {
				Topic string `json:"topic"`
			}
			json.Unmarshal(t, &data)
			slice := strings.Split(data.Topic, ":")
			topic := slice[0]

			if topic == "/market/ticker" {
				rawTickerData <- t
			}

			if topic == "/market/candles" {
				rawKlineData <- t
			}
		}
	}()

	// transform ticker
	go func() {
		for d := range rawTickerData {
			result := connector.ProcessRawTickerData(d)
			tickerPayload <- result
		}
		close(tickerPayload)
	}()

	// transform kline
	go func() {
		for d := range rawKlineData {
			result := connector.ProcessRawKlineData(d)
			klinePayload <- result
		}
		close(klinePayload)
	}()

	go func() {
		id := uuid.NewString()
		for range time.Tick(10 * time.Second) {
			err := connector.PingServer(c, id)
			if err != nil {
				log.Println("Ping error", err)
			}
		}
	}()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		resp := make(map[string]string)

		resp["message"] = "OK"
		jsonResp, err := json.Marshal(resp)
		if err != nil {
			log.Fatal("Unable to encode json response", err)
		}
		w.Write(jsonResp)
	})

	http.HandleFunc("/symbols", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		jsonSymbols, err := json.Marshal(symbols)
		if err != nil {
			log.Fatal("unable to encode json responsel", err)
		}
		w.Write(jsonSymbols)
	})

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		p := WSPayload{
			topic:  topic,
			kline:  klinePayload,
			ticker: tickerPayload,
		}
		serveWs(p, w, r)
	})

	log.Fatal(http.ListenAndServe(":3000", nil))
}
