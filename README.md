# Kucoin Connector

This is a websocket server that connects to the [Kucoin](https://www.kucoin.com/) exchange [API](https://docs.kucoin.com). This server subscribes to the ticker and kline (candles) endpoint.

## Getting Started
**Prerequisite:**

- [Go](https://go.dev/doc/install)

Start the server. Default port is 3000.
 ```bash
 go run main.go
 ```

Websocket endpoint: 
```
ws://localhost:3000/ws
```

You can subscribe to 2 topics: `ticker` and `kline`. Send the following JSON to the server:

**Subscribe to ticker topic**
```json
{"type": "ticker"}
```

**Subscribe to kline topic**
```json
{"type": "kline"}
```

## Start a subscriber client for testing
Go to the `clients` directory:
```bash
# Run a Kline subscriber client
go run klinesubscriber.go

# Run a Ticker subscriber client
go run tickersubscriber.go
```

## Notes 
1. Sometimes it takes a few seconds to start the server because the server needs to fetch the symbols list, and the kucoin REST API can be a bit unstable at times.
2. Takes a bit of time for the Kline sub to stream data because the server needs to subscribe to *all* of the topics (1000+ topics)


