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
	K chan connector.Kline
	T chan connector.Ticker
}

func payloadHandler(ws *websocket.Conn, k chan connector.Kline, t chan connector.Ticker) {
	for {
		select {
		case d := <-t:
			ws.WriteJSON(d)
		case <-k:
		}
	}
}

func serveWs(p WSPayload, w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		if _, ok := err.(websocket.HandshakeError); !ok {
			log.Println(err)
		}
		return
	}
	go payloadHandler(ws, p.K, p.T)
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
	klinePayload := make(chan connector.Kline)
	klineConnector := make(chan *websocket.Conn)

	go func() {
		for _, s := range symbols.Data {
			log.Println("Subscribing to kline ->", s.Symbol)
			topic := fmt.Sprintf("/market/candles:%s_1min", s.Symbol)
			id := uuid.NewString()

			k, _, err := connector.CreateKucoinWSClient()
			if err != nil {
				log.Println("Unable to subscribe ws client for kline topic->", topic)
			}
			defer k.Close()

			sErr := connector.SubscribeToTopic(k, topic, id)
			if sErr != nil {
				log.Println("Unable to subscribe ws client for kline topic->", topic)
			}

			klineConnector <- k
		}
	}()

	// Subscribe to all symbol tickers
	go func() {
		log.Println("subscribing to all tickers")
		id := uuid.NewString()
		topic := "/market/ticker:all"
		err := connector.SubscribeToTopic(c, topic, id)
		if err != nil {
			log.Println("Unable to subscribe to ticker", err)
			return
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

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		p := WSPayload{
			K: klinePayload,
			T: tickerPayload,
		}
		serveWs(p, w, r)
	})

	log.Fatal(http.ListenAndServe(":3000", nil))
}
