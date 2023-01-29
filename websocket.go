package main

import (
	"errors"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocket is a wrapper around a gorilla.WebSocket for claws.
type WebSocket struct {
	conn         *websocket.Conn
	writeChan    chan WsMsg
	pingTicker   *time.Ticker
	pingInterval time.Duration
	url          string
	sync.RWMutex // NOTE: for update private props
	// Used for reporting debug messages.
	FnDebug func(string)

	chWriEnd <-chan error
}

func (w *WebSocket) Debug(v string) {
	if (len(v) > 0) && (w.FnDebug != nil) {
		w.FnDebug(v)
	}
}

// returns the URL of the WebSocket.
func (w *WebSocket) URL() string {
	w.RLock()
	defer w.RUnlock()

	return w.url
}

// writes a message to the WebSocket
func (w *WebSocket) Write(msg WsMsg) bool {
	w.RLock()
	defer w.RUnlock()

	if w.writeChan != nil && w.conn != nil {
		w.writeChan <- msg
		return true
	}

	return false
}

func (pWs *WebSocket) SetPingInterval(secs int) {
	pWs.Lock()
	defer pWs.Unlock()

	pWs.setPingTicker(secs)
}

type WsMsg struct {
	Msg  []byte
	Type int
}

type WsReaderFunc func(*WsMsg, error)

func readPump(pConn *websocket.Conn, fnRdr WsReaderFunc) error {
	var err error

	for {
		var msg WsMsg
		msg.Type, msg.Msg, err = pConn.ReadMessage()
		if err != nil {
			// hide i/o after close error, since that's a typical
			// way of ending this read loop
			if errors.Is(err, net.ErrClosed) {
				err = nil
			}
			break
		}

		fnRdr(&msg, nil)

		if msg.Type == websocket.CloseMessage {
			break
		}
	}

	return err
}

// NOTE: closing chWrite terminates the inner goroutine
func goWritePump(pConn *websocket.Conn, chPing <-chan time.Time) (
	chWrite chan WsMsg, chExit chan error,
) {
	chWrite = make(chan WsMsg, 128)
	chExit = make(chan error)

	go func() {
		var err error
		defer func() {
			chExit <- err
		}()

		for {
			select {
			case msg, open := <-chWrite:
				if !open {
					return
				}
				if err = pConn.WriteMessage(msg.Type, msg.Msg); err != nil {
					return
				}

			case <-chPing:
				if err = pConn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			}
		}
	}()

	return
}

func (pWs *WebSocket) IsOpen() bool {
	pWs.RLock()
	defer pWs.RUnlock()
	return pWs.conn != nil
}

// closes the WebSocket connection.
func (pWs *WebSocket) WsClose() []error {
	pWs.Lock()
	defer pWs.Unlock()
	return pWs.closeAndClear()
}

// NOTE: must be mutexed by caller (currently WsClose & WsOpen)
func (pWs *WebSocket) closeAndClear() []error {
	var eRet []error

	// CLOSE OBJECTS
	{
		if pWs.pingTicker != nil {
			pWs.pingTicker.Stop()
		}

		// THIS INDIRECTLY CLOSES THE WritePump
		if pWs.writeChan != nil {
			close(pWs.writeChan)
		}

		// THIS INDIRECTLY CLOSES THE ReadPump
		if pWs.conn != nil {
			if err := pWs.conn.Close(); err != nil {
				eRet = append(eRet, err)
			}
		}
	}

	// Block and collect channel exit errors
	if pWs.chWriEnd != nil {
		if err := <-pWs.chWriEnd; err != nil {
			eRet = append(eRet, err)
		}
	}

	// CLEAR PROPS
	{
		pWs.conn = nil
		pWs.writeChan = nil
		pWs.pingInterval = 0
		pWs.url = ""
		pWs.chWriEnd = nil
	}

	return eRet
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

// opens a new WebSocket connection to `url`.
func (pWs *WebSocket) WsOpen(url string, nPingSeconds int, fnRdr WsReaderFunc) []error {
	pWs.Lock()
	defer pWs.Unlock()

	if pWs.conn != nil {
		// TODO: information message after setting ping duration
		// TODO: debug replacement
		pWs.Debug("Closing prior WebSocket connection")
		if sErr := pWs.closeAndClear(); len(sErr) > 0 {
			return sErr
		}
	}

	pWs.Debug("Starting WebSocket connection to " + url)
	conn, resp, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return []error{WebSocketResponseError{
			Err:  err,
			Resp: resp,
		}}
	}
	pWs.conn = conn
	pWs.url = url

	// READ PUMP
	go func() {
		if e := readPump(conn, fnRdr); e != nil {
			fnRdr(nil, e)
		}

		if sErr := pWs.WsClose(); len(sErr) > 0 {
			for _, e := range sErr {
				fnRdr(nil, e)
			}
		}
	}()

	// WRITE PUMP
	pWs.setPingTicker(nPingSeconds)
	pWs.writeChan, pWs.chWriEnd = goWritePump(conn, pWs.pingTicker.C)
	return nil
}

// NOTE: must be mutexed by caller
func (pWs *WebSocket) setPingTicker(secs int) {
	if pWs.pingTicker == nil {
		pWs.pingTicker = time.NewTicker(time.Hour)
	}

	dur := time.Duration(secs) * time.Second
	if dur > 0 {
		pWs.pingInterval = dur
		pWs.pingTicker.Reset(dur)
	} else {
		pWs.pingInterval = 0
		pWs.pingTicker.Stop()
	}
}
