package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"kucoin-ws-connector/connector"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/shopspring/decimal"
)

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

	if err != nil {
		log.Fatal("Unable to dial into exchange ws:", err)
	}
	defer c.Close()

	ticker := time.NewTicker(time.Second * 1)
	defer ticker.Stop()

	trade := make(chan []byte)
	rawTickerData := make(chan []byte)
	rawKlineData := make(chan []byte)

	tickerPayload := make(chan Ticker)
	klinePayload := make(chan Kline)

	go func() {
		for _, s := range symbols.Data {
			log.Println("Subscribing to kline ->", s.Symbol)
			topic := fmt.Sprintf("/market/candles:%s_1min", s.Symbol)
			id := uuid.NewString()
			s := connector.SubscriptionRequest{
				Id:             id,
				Type:           "subscribe",
				Topic:          topic,
				PrivateChannel: false,
				Response:       true,
			}

			k, _, err := connector.CreateKucoinWSClient()
			if err != nil {
				log.Println("Unable to subscribe ws client for kline topic->", topic)
			}

			sErr := connector.SubscribeToTopic(k, s)
			if sErr != nil {
				log.Println("Unable to subscribe ws client for kline topic->", topic)
			}
		}
	}()

	// Subscribe to all symbol tickers
	go func() {
		log.Println("subscribing to all tickers")
		id := uuid.NewString()
		b, _ := json.Marshal(connector.SubscriptionRequest{
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
			klinePayload <- result
		}
		close(klinePayload)
	}()

	// handle payload to clients
	go func() {
		for {
			select {
			case t := <-klinePayload:
				log.Println(t)
			case <-tickerPayload:
				// case t := <-tickerPayload:
				// 	log.Println(t)
			}
		}
	}()

	log.Fatal(http.ListenAndServe(":3000", nil))
}
