package main

import (
	"encoding/json"
	"fmt"
	"kucoin-ws-connector/connector"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

const tickerTopic = "/market/ticker:all"

func getTopic(t []byte) (topic string) {
	var data struct {
		Topic string `json:"topic"`
	}
	json.Unmarshal(t, &data)
	slice := strings.Split(data.Topic, ":")
	topic = slice[0]
	return
}

func main() {
	symbols := connector.GetAllKucoinSymbols()
	client, err := connector.CreateKucoinClient()
	if err != nil {
		log.Fatal("Unable to dial into exchange ws:", err)
	}
	defer client.Conn.Close()

	kClient, err := connector.CreateKucoinClient()
	if err != nil {
		log.Fatal("Unable to dial into exchange ws:", err)
	}
	defer kClient.Conn.Close()

	ticker := time.NewTicker(5 * time.Second)

	// go connector.TopicManager(client.Conn, client.Topic)
	go connector.InitListener(client.Conn, client.Trade)
	go connector.InitListener(kClient.Conn, kClient.Trade)

	go func() {
		log.Println("subscribing to topic ->", tickerTopic)
		err := connector.ManageSubscription(client.Conn, tickerTopic, client.ID, true)
		if err != nil {
			log.Println("unable to subscribe to ticker", err)
		}
	}()

	go func() {
		for _, symbol := range symbols {
			<-time.Tick(250 * time.Millisecond) // stagger the writes to the ws server
			topic := fmt.Sprintf("/market/candles:%s_1min", symbol)
			log.Println("subscribing to topic ->", topic)
			id := symbol + client.ID
			err := connector.ManageSubscription(kClient.Conn, topic, id, true)
			if err != nil {
				log.Println("unable to subscribe to kline", err)
			}
		}
	}()

	go func() {
		for {
			select {
			case t := <-client.Trade:
				topic := getTopic(t)
				if topic == "/market/ticker" {
					client.OutboundTicker <- connector.ProcessRawTickerData(t)
				}
			case t := <-kClient.Trade:
				topic := getTopic(t)
				if topic == "/market/candles" {
					kClient.OutboundKline <- connector.ProcessRawKlineData(t)
				}
			case <-ticker.C:
				err := connector.PingServer(client.Conn, client.ID)
				if err != nil {
					log.Println("Ping error", err)
				}
				err2 := connector.PingServer(kClient.Conn, kClient.ID)
				if err2 != nil {
					log.Println("Ping error", err)
				}
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
		p := connector.WSPayload{
			Topic:  client.Topic,
			Kline:  kClient.OutboundKline,
			Ticker: client.OutboundTicker,
		}
		connector.ServeWs(p, w, r)
	})

	log.Fatal(http.ListenAndServe(":3000", nil))
}
