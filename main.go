package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

type TokenResponse struct {
	Code string `json:"code"`
	Data struct {
		InstanceServers []struct {
			Endpoint     string `json:"endpoint"`
			Protocol     string `json:"protocol"`
			Encrypt      bool   `json:"encrypt"`
			PingInterval int    `json:"pingInterval"`
			PingTimeout  int    `json:"pingTimeout"`
		} `json:"instanceServers"`
		Token string `json:"token"`
	} `json:"data"`
}

type Ping struct {
	Id   string `json:"id"`
	Type string `json:"type"`
}

type SubscriptionRequest struct {
	Id             int    `json:"id"`
	Type           string `json:"type"`
	Topic          string `json:"topic"`
	PrivateChannel bool   `json:"privateChannel"`
	Response       bool   `json:"response"`
}

func main() {
	r, err := http.Post("https://api.kucoin.com/api/v1/bullet-public", "application/json", nil)
	if err != nil {
		log.Fatal("Unable to retrieve public token", err)
	}
	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)

	if err != nil {
		log.Fatal("Public token error", err)
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		log.Fatal("Unable to parse response body", err)
	}

	connectId := "loremipsumdolorsitamet78436"

	u := fmt.Sprintf("wss://ws-api.kucoin.com/endpoint?token=%s&[connectId=%s]", tokenResp.Data.Token, connectId)
	c, _, err := websocket.DefaultDialer.Dial(u, nil)

	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	messageType := make(chan int, 1)

	// Message Reader & Handler
	go func() {
		for {
			mt, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read err:", err)
			}
			messageType <- mt
			// log.Println(string(message))
		}
	}()

	// Ping
	// go func() {
	// 	m := <-messageType

	// 	b, err := json.Marshal(Ping{Id: connectId, Type: "ping"})
	// 	if err != nil {
	// 		log.Println("err", err)
	// 		return
	// 	}
	// 	writeErr := c.WriteMessage(m, b)
	// 	if writeErr != nil {
	// 		log.Println("Unable to ping ws server")
	// 		return
	// 	}
	// }()

	// Subscribe
	go func() {
		log.Println("subscribing")
		m := <-messageType
		b, err := json.Marshal(SubscriptionRequest{
			Id:             1545910660739,
			Type:           "subscribe",
			Topic:          "/market/ticker:BTC-USDT",
			PrivateChannel: false,
			Response:       true,
		})
		if err != nil {
			log.Println("err", err)
			return
		}

		writeErr := c.WriteMessage(m, b)
		if writeErr != nil {
			log.Println("Unable to subscribe to ticker")
			return
		}
	}()

	pingInterval := tokenResp.Data.InstanceServers[0].PingInterval
	log.Println("ping interval", time.Duration(pingInterval))

	ticker := time.NewTicker(time.Second * 10)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ticker.C:
				b, err := json.Marshal(Ping{Id: connectId, Type: "ping"})
				if err != nil {
					log.Println("err", err)
					return
				}
				writeErr := c.WriteMessage(1, b)
				if writeErr != nil {
					log.Println("Unable to ping ws server")
					return
				}
			case <-messageType:
				// log.Println("message type inferred")
			}
		}
	}()

	log.Fatal(http.ListenAndServe(":3000", nil))
}
