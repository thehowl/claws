package main

import (
	"bytes"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/gorilla/websocket"
	"github.com/jroimartin/gocui"
)

// State is the central function managing the information of claws.
type State struct {
	// important to running the application as a whole
	ActionIndex       int
	Mode              UIMode
	ConnectionStarted time.Time
	wsConn            WebSocket

	Writer     io.Writer
	writerLock sync.RWMutex

	// important for drawing
	FirstDrawDone     bool
	ShouldQuit        bool
	HideHelp          bool
	KeepAutoscrolling bool

	// functions
	ExecuteFunc func(func(*gocui.Gui) error)

	Settings Settings
}

// PushAction adds an action to LastActions
func (s *State) PushAction(act string) error {
	return s.Settings.PushAction(act)
}

// BrowseActions changes the ActionIndex and returns the value at the specified index.
// move is the number of elements to move (negatives go into more recent history,
// 0 returns the current element, positives go into older history)
func (s *State) BrowseActions(move int) string {
	oSet := s.Settings.Clone()

	nActions := len(oSet.LastActions)
	s.ActionIndex += move
	if s.ActionIndex >= nActions {
		s.ActionIndex = nActions - 1
	} else if s.ActionIndex < -1 {
		s.ActionIndex = -1
	}

	// -1 always indicates the "next" element, thus empty
	if s.ActionIndex == -1 {
		return ""
	}

	return oSet.LastActions[s.ActionIndex]
}

// StartConnection begins a WebSocket connection to url.
func (s *State) StartConnection(url string) {
	var err error
	defer func() {
		if err != nil {
			s.PrintError(err)
		}
	}()

	url = strings.TrimSpace(url)
	if len(url) > 0 {
		s.Settings.LastWebsocketURL = url
		s.Settings.Update("LastWebsocketURL")
	}

	if len(s.Settings.LastWebsocketURL) == 0 {
		return
	}

	// TODO: channel into editor message pump?
	s.wsConn.FnDebug = func(v string) {
		s.PrintDebug(v)
	}

	fnWsReadmsg := func(msg *WsMsg, err error) {
		if err != nil {
			s.PrintError(err)
		}
		if msg != nil {
			s.PrintFromPeer(*msg)
		}
	}

	oSet := s.Settings.Clone()
	sErrs := s.wsConn.WsOpen(
		s.Settings.LastWebsocketURL,
		oSet.PingSeconds,
		fnWsReadmsg,
	)
	for _, err := range sErrs {
		s.PrintError(err)
	}

	s.ConnectionStarted = time.Now()
}

func (s *State) SetPingInterval(nSecs int) {
	s.Settings.PingSeconds = nSecs
	s.Settings.Update("PingSeconds")

	s.wsConn.SetPingInterval(nSecs)
}

func (s *State) WsSendMsg(msg string) bool {
	return s.wsConn.Write(WsMsg{
		Type: websocket.TextMessage,
		Msg:  []byte(msg),
	})
}

type WsInfo struct {
	IsOpen   bool
	Url      string
	Settings SettingsBase
}

func (s *State) GetWsInfo() WsInfo {
	return WsInfo{
		IsOpen:   s.wsConn.IsOpen(),
		Url:      s.wsConn.URL(),
		Settings: s.Settings.Clone(),
	}
}

func (s *State) WsClose() []error {
	return s.wsConn.WsClose()
}

var (
	printDebug  = color.New(color.FgCyan).Fprint
	printError  = color.New(color.FgRed).Fprint
	printUser   = color.New(color.FgGreen).Fprint
	printServer = color.New(color.FgWhite).Fprint
)

// PrintDebug prints debug information to the Writer, using light blue.
func (s *State) PrintDebug(x string) {
	s.printToOut(x, false, printDebug)
}

// PrintError prints an error to the Writer, using red.
func (s *State) PrintError(x error) {
	if x != nil {
		s.printToOut(x.Error(), false, printError)
	}
}

// prints user-provided messages to the Writer, using green.
func (s *State) PrintFromUser(x string) {
	oSet := s.Settings.Clone()

	res, err := s.pipe([]byte(x), "out", oSet.Pipe.Out)
	if err != nil {
		s.PrintError(err)
		if len(bytes.TrimSpace(res)) == 0 {
			return
		}
	}

	s.printToOut(string(res), false, printUser)
}

// prints server-returned messages to the Writer, using white.
func (s *State) PrintFromPeer(msg WsMsg) {
	switch msg.Type {
	case websocket.PingMessage:
		s.PrintDebug("<PING MSG>")
		return
	case websocket.PongMessage:
		s.PrintDebug("<PONG MSG>")
		return
	case websocket.CloseMessage:
		s.PrintDebug("<CLOSE MSG>")
		return
	}

	// TODO: cmdline flags for HTTP headers to send with websocket connect
	// TODO: persistent pipes?
	oSet := s.Settings.Clone()
	res, err := s.pipe(msg.Msg, "in", oSet.Pipe.In)
	if err != nil {
		s.PrintError(err)
		if len(bytes.TrimSpace(res)) == 0 {
			return
		}
	}

	var szText string
	switch msg.Type {
	case websocket.BinaryMessage:
		szText = strings.TrimSuffix(hex.Dump(res), "\n")

	case websocket.TextMessage:
		if oSet.JSONFormatting {
			res = attemptJSONFormatting(res)
		}
		szText = strings.TrimSuffix(string(res), "\n")
	}

	s.printToOut(szText, true, printServer)
}

var (
	sessionStarted = time.Now()
)

func (s *State) pipe(data []byte, t string, command []string) ([]byte, error) {
	if len(command) < 1 {
		return data, nil
	}
	// prepare the command: create it, set up env variables
	c := exec.Command(command[0], command[1:]...)
	c.Env = append(
		os.Environ(),
		"CLAWS_PIPE_TYPE="+t,
		"CLAWS_SESSION="+strconv.FormatInt(sessionStarted.UnixNano()/1000, 10),
		"CLAWS_CONNECTION="+strconv.FormatInt(s.ConnectionStarted.UnixNano()/1000, 10),
		"CLAWS_WS_URL="+s.wsConn.URL(),
	)
	// set up stdin
	stdin := bytes.NewReader(data)
	c.Stdin = stdin

	// run the command
	return c.Output()
}

func (s *State) printToOut(
	str string,
	bIndent bool,
	f func(io.Writer, ...interface{}) (int, error),
) {
	// NOTE: mutexed to sequentialize whole writes between
	//       UI goroutine, read pump, & write pump
	s.writerLock.Lock()
	defer s.writerLock.Unlock()

	var szTs string
	oSet := s.Settings.Clone()
	if len(oSet.Timestamp) > 0 {
		szTs = time.Now().Format(oSet.Timestamp)
	}

	s.ExecuteFunc(func(*gocui.Gui) error {
		if s.Writer == nil {
			return nil
		}

		// Timestamp, not indented.
		bHasTs := len(szTs) > 0
		if bHasTs {
			if _, e := f(s.Writer, szTs); e != nil {
				return e
			}
		}

		if bIndent {
			if bHasTs {
				if _, e := f(s.Writer, "\n"); e != nil {
					return e
				}
			}

			const indentPrefix = "  "
			_, e := f(s.Writer, indentPrefix+strings.ReplaceAll(str, "\n", "\n"+indentPrefix)+"\n")
			return e
		}

		_, e := f(s.Writer, str+"\n")
		return e
	})
}
