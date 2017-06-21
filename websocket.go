package main

import (
	"net/http"

	"github.com/gorilla/websocket"
)

// WebSocket is a wrapper around a gorilla.WebSocket for claws.
type WebSocket struct {
	conn      *websocket.Conn
	writeChan chan string
}

// ReadChannel retrieves a channel from which to read messages out of.
func (w *WebSocket) ReadChannel() <-chan string {
	ch := make(chan string, 16)
	go w.readChannel(ch)
	return ch
}

// Write writes a message to the WebSocket
func (w *WebSocket) Write(msg string) {
	w.writeChan <- msg
}

func (w *WebSocket) readChannel(c chan<- string) {
	for {
		_type, msg, err := w.conn.ReadMessage()
		if err != nil {
			state.Error(err.Error())
			w.close(c)
			return
		}

		switch _type {
		case websocket.TextMessage, websocket.BinaryMessage:
			c <- string(msg)
		case websocket.CloseMessage:
			cl := "Closed WebSocket"
			if len(msg) > 0 {
				cl += " " + string(msg)
			}
			state.Debug(cl)
			w.close(c)
			return
		case websocket.PingMessage, websocket.PongMessage:
			if len(msg) > 0 {
				state.Debug("Ping/pong with " + string(msg))
			}
		}
	}
}

func (w *WebSocket) writePump() {
	for msg := range w.writeChan {
		err := w.conn.WriteMessage(websocket.TextMessage, []byte(msg))
		if err != nil {
			state.Error(err.Error())
			w.Close()
			return
		}
	}
}

// Close closes the WebSocket connection.
func (w *WebSocket) Close() error {
	close(w.writeChan)
	return w.conn.Close()
}

// close finalises the WebSocket connection.
func (w *WebSocket) close(c chan<- string) error {
	close(c)
	return w.Close()
}

// WebSocketResponseError is the error returned when there is an error in
// CreateWebSocket.
type WebSocketResponseError struct {
	Err  error
	Resp *http.Response
}

func (w WebSocketResponseError) Error() string {
	return w.Err.Error()
}

// CreateWebSocket initialises a new WebSocket connection.
func CreateWebSocket(url string) (*WebSocket, error) {
	state.Debug("Starting WebSocket connection to " + url)

	conn, resp, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, WebSocketResponseError{
			Err:  err,
			Resp: resp,
		}
	}

	ws := &WebSocket{
		conn:      conn,
		writeChan: make(chan string, 128),
	}

	go ws.writePump()

	return ws, nil
}
