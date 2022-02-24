package main

import (
	"encoding/json"
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

	go connector.TopicManager(c, topic)
	go connector.InitListener(c, trade)

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
			log.Fatal("unable to encode json response", err)
		}
		w.Write(jsonSymbols)
	})

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		p := connector.WSPayload{
			Topic:  topic,
			Kline:  klinePayload,
			Ticker: tickerPayload,
		}
		connector.ServeWs(p, w, r)
	})

	log.Fatal(http.ListenAndServe(":3000", nil))
}
