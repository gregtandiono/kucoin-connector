package connector

import "github.com/shopspring/decimal"

type Instrument struct {
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
	Type       string     `json:"type"`
	Instrument Instrument `json:"instrument"`
	Data       TickerData `json:"data"`
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
	Type       string     `json:"type"`
	Instrument Instrument `json:"instrument"`
	Data       KlineData  `json:"data"`
}
