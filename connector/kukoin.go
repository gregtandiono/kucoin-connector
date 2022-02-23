package connector

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type SubscriptionRequest struct {
	Id             string `json:"id"`
	Type           string `json:"type"`
	Topic          string `json:"topic"`
	PrivateChannel bool   `json:"privateChannel"`
	Response       bool   `json:"response"`
}

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

func getToken() (token string) {
	r, err := http.Post("https://api.kucoin.com/api/v1/bullet-public", "application/json", nil)
	if err != nil {
		log.Fatal("Unable to retrieve public token", err)
	}
	defer r.Body.Close()
	body, _ := ioutil.ReadAll(r.Body)

	var tokenResp TokenResponse
	json.Unmarshal(body, &tokenResp)
	token = tokenResp.Data.Token

	return
}

func CreateKucoinWSClient() (c *websocket.Conn, connectId string, err error) {
	token := getToken()
	connectId = uuid.NewString()

	u := fmt.Sprintf("wss://ws-api.kucoin.com/endpoint?token=%s&[connectId=%s]", token, connectId)
	c, _, err = websocket.DefaultDialer.Dial(u, nil)

	return
}

func InitKucoinListener(c *websocket.Conn, t chan []byte) {
	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			log.Printf("error: %v", err)
			break
		}
		t <- message
	}
}

func SubscribeToTopic(c *websocket.Conn, s SubscriptionRequest) (err error) {
	b, _ := json.Marshal(s)
	err = c.WriteMessage(websocket.TextMessage, b)

	return
}

func PayloadHandler(k chan struct{}, t chan struct{}) {
	for {
		select {
		case d := <-k:
			log.Println(d)
		case d := <-t:
			log.Println(d)
		}
	}
}
