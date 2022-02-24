package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"kucoin-ws-connector/connector"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type Symbol struct {
	Code string `json:"code"`
	Data []struct {
		Symbol string `json:"symbol"`
	} `json:"data"`
}

func main() {
	sr, err := http.Get("https://api.kucoin.com/api/v1/symbols")
	if err != nil {
		log.Fatal("Unable to fetch all symbols", err)
	}
	defer sr.Body.Close()
	srBody, _ := ioutil.ReadAll(sr.Body)

	var symbols Symbol
	json.Unmarshal(srBody, &symbols)

	log.Println("How many symbols", len(symbols.Data))

	c, _, err := connector.CreateKucoinWSClient()
	if err != nil {
		log.Fatal("Unable to dial into exchange ws:", err)
	}
	defer c.Close()

	ticker := time.NewTicker(time.Second * 1)
	defer ticker.Stop()

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

	// handle payload to clients
	go func() {
		for {
			select {
			case t := <-klinePayload:
				log.Println(t)
			case k := <-klineConnector:
				go connector.InitKucoinListener(k, trade)
				go func() {
					id := uuid.NewString()
					for range time.Tick(10 * time.Second) {
						err := connector.PingServer(k, id)
						if err != nil {
							log.Println("Ping error", err)
						}
					}
				}()
			case t := <-tickerPayload:
				log.Println(t)
			}
		}
	}()

	log.Fatal(http.ListenAndServe(":3000", nil))
}
