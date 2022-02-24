package connector

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/shopspring/decimal"
)

type Ping struct {
	Id   string `json:"id"`
	Type string `json:"type"`
}
type KucoinSymbol struct {
	Code string `json:"code"`
	Data []struct {
		Symbol string `json:"symbol"`
	} `json:"data"`
}

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

type KucoinTickerRaw struct {
	Type    string `json:"type"`
	Topic   string `json:"topic"`
	Subject string `json:"subject"`
	Data    struct {
		BestAsk string `json:"bestAsk"`
		BestBid string `json:"bestBid"`
		Time    int    `json:"time"`
	} `json:"data"`
}

type KucoinKlineRaw struct {
	Type    string `json:"type"`
	Topic   string `json:"topic"`
	Subject string `json:"subject"`
	Data    struct {
		Symbol  string   `json:"symbol"`
		Candles []string `json:"candles"`
		Time    int      `json:"time"`
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

func GetAllKucoinSymbols() (symbols KucoinSymbol) {
	r, err := http.Get("https://api.kucoin.com/api/v1/symbols")
	if err != nil {
		log.Fatal("Unable to fetch all symbols", err)
	}
	defer r.Body.Close()
	body, _ := ioutil.ReadAll(r.Body)

	var symbolsBody KucoinSymbol
	json.Unmarshal(body, &symbolsBody)
	symbols = symbolsBody
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

func PingServer(c *websocket.Conn, id string) (err error) {
	b, _ := json.Marshal(Ping{
		Id:   id,
		Type: "ping",
	})
	err = c.WriteMessage(websocket.TextMessage, b)
	c.PingHandler()
	return
}

func SubscribeToTopic(c *websocket.Conn, topic string, id string) (err error) {
	s := SubscriptionRequest{
		Id:             id,
		Type:           "subscribe",
		Topic:          topic,
		PrivateChannel: false,
		Response:       true,
	}
	b, _ := json.Marshal(s)
	err = c.WriteMessage(websocket.TextMessage, b)

	return
}

func ProcessRawTickerData(d []byte) (result Ticker) {
	var raw KucoinTickerRaw
	json.Unmarshal(d, &raw)

	askPrice, _ := decimal.NewFromString(raw.Data.BestAsk)
	bidPrice, _ := decimal.NewFromString(raw.Data.BestBid)

	result = Ticker{
		Type: "ticker",
		Instrument: Instrument{
			Exchange:      "kucoin",
			Symbol:        raw.Subject,
			BaseCurrency:  "USD",
			QuoteCurrency: "USD",
		},
		Data: TickerData{
			AskPrice: askPrice,
			BidPrice: bidPrice,
			Time:     raw.Data.Time,
		},
	}
	return
}

func ProcessRawKlineData(d []byte) (result Kline) {
	var raw KucoinKlineRaw
	json.Unmarshal(d, &raw)

	startTime, _ := strconv.Atoi(raw.Data.Candles[0])
	endTime := startTime + 3600

	openPrice, _ := decimal.NewFromString(raw.Data.Candles[1])
	closePrice, _ := decimal.NewFromString(raw.Data.Candles[2])
	highPrice, _ := decimal.NewFromString(raw.Data.Candles[3])
	lowPrice, _ := decimal.NewFromString(raw.Data.Candles[4])
	volumeBase, _ := decimal.NewFromString(raw.Data.Candles[5])
	volumeQuote, _ := decimal.NewFromString(raw.Data.Candles[6])

	result = Kline{
		Type: "ohlc",
		Instrument: Instrument{
			Exchange:      "kucoin",
			Symbol:        raw.Data.Symbol,
			BaseCurrency:  "USD",
			QuoteCurrency: "USD",
		},
		Data: KlineData{
			IntervalSeconds: 3600,
			StartTime:       startTime,
			EndTime:         endTime,
			Open:            openPrice,
			High:            highPrice,
			Low:             lowPrice,
			Close:           closePrice,
			VolumeBase:      volumeBase,
			VolumeQuote:     volumeQuote,
		},
	}
	return
}