package main

import (
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocket is a wrapper around a gorilla.WebSocket for claws.
type WebSocket struct {
	conn       *websocket.Conn
	writeChan  chan string
	pingTicker *time.Ticker
	url        string
}

// URL returns the URL of the WebSocket.
func (w *WebSocket) URL() string {
	return w.url
}

// Write writes a message to the WebSocket
func (w *WebSocket) Write(msg string) {
	w.writeChan <- msg
}

// ReadChannel retrieves a channel from which to read messages out of.
func (w *WebSocket) ReadChannel() <-chan string {
	ch := make(chan string, 16)
	go w.readPump(ch)
	return ch
}

// NOTE: only used inside ReadChannel()
func (w *WebSocket) readPump(ch chan<- string) {

	defer w.CloseWs()
	defer close(ch)

	for {
		_type, msg, err := w.conn.ReadMessage()
		if err != nil {
			state.Error(err.Error())
			return
		}

		switch _type {
		case websocket.TextMessage, websocket.BinaryMessage:
			ch <- string(msg)
		case websocket.CloseMessage:
			cl := "Closed WebSocket"
			if len(msg) > 0 {
				cl += " " + string(msg)
			}
			state.Debug(cl)
			return
		case websocket.PingMessage, websocket.PongMessage:
			if len(msg) > 0 {
				state.Debug("Ping/pong with " + string(msg))
			}
		}
	}
}

func (w *WebSocket) writePump() {

	var pingChan <-chan time.Time
	if w.pingTicker != nil {
		pingChan = w.pingTicker.C
	}

	for {

		select {

		case msg, open := <-w.writeChan:

			if !open {
				return
			}

			if E := w.conn.WriteMessage(websocket.TextMessage, []byte(msg)); E != nil {
				state.Error(E.Error())
				return
			}

		case <-pingChan:

			if E := w.conn.WriteMessage(websocket.PingMessage, []byte{}); E != nil {
				state.Error(E.Error())
				return
			}
		}
	}
}

// CloseWs closes the WebSocket connection.
func (w *WebSocket) CloseWs() error {

	if w == nil {
		return nil
	}

	// NOTE: this could be a nasty rug-pull for editor.go
	if state.Conn == w {
		state.Conn = nil
	}

	if w.pingTicker != nil {
		w.pingTicker.Stop()
		w.pingTicker = nil
	}

	if w.writeChan != nil {
		close(w.writeChan)
		w.writeChan = nil
	}

	if w.conn != nil {
		E := w.conn.Close()
		w.conn = nil
		return E
	}

	return nil
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
		url:       url,
	}

	// START PING TICKER IF ENABLED
	oSet := state.Settings.Clone()
	if oSet.PingSeconds > 0 {
		ws.pingTicker = time.NewTicker(time.Duration(oSet.PingSeconds) * time.Second)
	}

	go ws.writePump()

	return ws, nil
}
