package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/shopspring/decimal"
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

type Symbol struct {
	Code string `json:"code"`
	Data []struct {
		Symbol string `json:"symbol"`
	} `json:"data"`
}

type TickerRaw struct {
	Type    string `json:"type"`
	Topic   string `json:"topic"`
	Subject string `json:"subject"`
	Data    struct {
		BestAsk string `json:"bestAsk"`
		BestBid string `json:"bestBid"`
		Time    int    `json:"time"`
	} `json:"data"`
}

type KlineRaw struct {
	Type    string `json:"type"`
	Topic   string `json:"topic"`
	Subject string `json:"subject"`
	Data    struct {
		Symbol  string   `json:"symbol"`
		Candles []string `json:"candles"`
		Time    int      `json:"time"`
	} `json:"data"`
}

type KucoinInstrument struct {
	Exchange      string `json:"exchange"`
	Symbol        string `json:"symbol"`
	BaseCurrency  string `json:"base_currency"`
	QuoteCurrency string `json:"quote_currency"`
}

type TickerData struct {
	AskPrice decimal.Decimal `json:"ask_price"`
	BidPrice decimal.Decimal `json:"bid_price"`
	Time     int             `json:"raw"`
}

type Ticker struct {
	Type       string           `json:"type"`
	Instrument KucoinInstrument `json:"instrument"`
	Data       TickerData       `json:"data"`
}

type KlineData struct {
	IntervalSeconds int             `json:"interval_seconds"`
	StartTime       int             `json:"start_time"`
	EndTime         int             `json:"end_time"`
	Open            decimal.Decimal `json:"open"`
	High            decimal.Decimal `json:"high"`
	Low             decimal.Decimal `json:"low"`
	Close           decimal.Decimal `json:"close"`
	VolumeBase      decimal.Decimal `json:"volume_base"`
	VolumeQuote     decimal.Decimal `json:"volume_quote"`
}

type Kline struct {
	Type       string           `json:"type"`
	Instrument KucoinInstrument `json:"instrument"`
	Data       KlineData        `json:"data"`
}

type SubscriptionRequest struct {
	Id             string `json:"id"`
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
	body, _ := ioutil.ReadAll(r.Body)

	var tokenResp TokenResponse
	json.Unmarshal(body, &tokenResp)

	sr, err := http.Get("https://api.kucoin.com/api/v1/symbols")
	if err != nil {
		log.Fatal("Unable to fetch all symbols", err)
	}
	defer sr.Body.Close()
	srBody, _ := ioutil.ReadAll(sr.Body)

	var symbols Symbol
	json.Unmarshal(srBody, &symbols)

	connectId := "loremipsumdolorsitamet78436"

	u := fmt.Sprintf("wss://ws-api.kucoin.com/endpoint?token=%s&[connectId=%s]", tokenResp.Data.Token, connectId)
	c, _, err := websocket.DefaultDialer.Dial(u, nil)

	if err != nil {
		log.Fatal("Unable to dial into exchange ws:", err)
	}
	defer c.Close()

	// Subscribe to all symbol tickers
	go func() {
		log.Println("subscribing to all tickers")
		id := uuid.NewString()
		b, _ := json.Marshal(SubscriptionRequest{
			Id:             id,
			Type:           "subscribe",
			Topic:          "/market/ticker:all",
			PrivateChannel: false,
			Response:       true,
		})
		err := c.WriteMessage(websocket.TextMessage, b)
		if err != nil {
			log.Println("Unable to subscribe to ticker", err)
			return
		}
	}()

	// Subscribe to klines
	go func() {
		log.Println("Subscribing to kline ->")

		topic := fmt.Sprintf("/market/candles:%s_1min", "BTC-USDT")
		id := uuid.NewString()

		b, _ := json.Marshal(SubscriptionRequest{
			Id:             id,
			Type:           "subscribe",
			Topic:          topic,
			PrivateChannel: false,
			Response:       true,
		})
		err := c.WriteMessage(websocket.TextMessage, b)
		if err != nil {
			log.Println("Unable to subscribe to klineticker", err)
			return
		}
		// for _, s := range symbols.Data {
		// 	log.Println("Subscribing to kline ->", s.Symbol)

		// 	topic := fmt.Sprintf("/market/candles:%s_1min", s.Symbol)
		// 	id := uuid.NewString()

		// 	b, _ := json.Marshal(SubscriptionRequest{
		// 		Id:             id,
		// 		Type:           "subscribe",
		// 		Topic:          topic,
		// 		PrivateChannel: false,
		// 		Response:       true,
		// 	})
		// 	err := c.WriteMessage(websocket.TextMessage, b)
		// 	if err != nil {
		// 		log.Println("Unable to subscribe to klineticker", err)
		// 		return
		// 	}
		// }
	}()

	// pingInterval := tokenResp.Data.InstanceServers[0].PingInterval
	// log.Println("ping interval", time.Duration(pingInterval))

	ticker := time.NewTicker(time.Second * 1)
	defer ticker.Stop()

	trade := make(chan []byte)
	rawTickerData := make(chan []byte)
	rawKlineData := make(chan []byte)

	tickerPayload := make(chan Ticker)
	klinePayload := make(chan Kline)

	// Message Reader & Handler
	go func() {
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read err:", err)
				break
			}

			// <-ticker.C
			trade <- message
		}
		close(trade)
	}()

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
			var raw TickerRaw
			json.Unmarshal(d, &raw)

			askPrice, _ := decimal.NewFromString(raw.Data.BestAsk)
			bidPrice, _ := decimal.NewFromString(raw.Data.BestBid)

			result := Ticker{
				Type: "ticker",
				Instrument: KucoinInstrument{
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
			tickerPayload <- result
		}
		close(tickerPayload)
	}()

	// transform kline
	go func() {
		for d := range rawKlineData {
			var raw KlineRaw
			json.Unmarshal(d, &raw)

			startTime, _ := strconv.Atoi(raw.Data.Candles[0])
			endTime := startTime + 3600

			openPrice, _ := decimal.NewFromString(raw.Data.Candles[1])
			closePrice, _ := decimal.NewFromString(raw.Data.Candles[2])
			highPrice, _ := decimal.NewFromString(raw.Data.Candles[3])
			lowPrice, _ := decimal.NewFromString(raw.Data.Candles[4])
			volumeBase, _ := decimal.NewFromString(raw.Data.Candles[5])
			volumeQuote, _ := decimal.NewFromString(raw.Data.Candles[6])

			result := Kline{
				Type: "ohlc",
				Instrument: KucoinInstrument{
					Exchange:      "kucoin",
					Symbol:        raw.Subject,
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
			klinePayload <- result
		}
		close(klinePayload)
	}()

	// handle payload to clients
	go func() {
		for {
			select {
			case t := <-tickerPayload:
				log.Println(t)
			case t := <-klinePayload:
				log.Println(t)
			}
		}
	}()

	// ping
	go func() {
		for range ticker.C {
			b, err := json.Marshal(Ping{Id: connectId, Type: "ping"})
			if err != nil {
				log.Println("err", err)
				return
			}
			writeErr := c.WriteMessage(websocket.TextMessage, b)
			if writeErr != nil {
				log.Println("Unable to ping ws server")
				return
			}
		}
	}()

	log.Fatal(http.ListenAndServe(":3000", nil))
}
