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
```json
{"type": "ticker"}
```
You will immediately see ticker data coming in.

## Start a subscriber client for testing
Go to the `clients` directory:
```bash
# Run a Kline subscriber client
go run klinesubscriber.go

# Run a Ticker subscriber client
go run tickersubscriber.go
```

## Notes 
1. Sometimes it takes a few seconds to start the server because the server needs to fetch the symbols list, and the kucoin REST API can a bit unstable at times.
2. Takes a bit of time for the Kline sub to stream data because the server needs to subscribe to *all* of the topics (1000+ topics)
3. Seem like you can only see the kline data coming in if a client has subscribed to the ticker topic beforehand. The workaround:
```golang
// connectors/ws.go

//...
case "kline":
	go func() {
		for {
			select {
			case <-client.payload.Ticker:
			case d := <-client.payload.Kline:
				for c := range client.pool.clients {
					if c.topic == subscriptionType {
						c.mu.Lock()
						c.conn.WriteJSON(d)
						c.mu.Unlock()
					}
				}
			}
		}
	}()
//...
```


