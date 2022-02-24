package connector

// import (
// 	"log"

// 	"github.com/gorilla/websocket"
// )

// type Client struct {
// 	conn *websocket.Conn
// 	send chan []byte
// }

// func (c *Client) read() {
// 	defer c.conn.Close()
// 	for {
// 		_, message, err := c.conn.ReadMessage()
// 		if err != nil {
// 			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
// 				log.Printf("error: %v", err)
// 			}
// 			break
// 		}
// 		c.send <- message
// 	}
// }

// func serveWs() {}
