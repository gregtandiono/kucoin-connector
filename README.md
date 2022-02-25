 # Kucoin Connector

 This is a websocket server that connects to the Kucoin exchange API. This server subscribes to ticker and kline (candles) endpoint

## Getting Started

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

