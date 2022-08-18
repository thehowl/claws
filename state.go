package main

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/jroimartin/gocui"
)

var state = &State{
	ActionIndex: -1,
	HideHelp:    len(os.Args) > 1,
}

// State is the central function managing the information of claws.
type State struct {
	// important to running the application as a whole
	ActionIndex int
	Mode        int
	Writer      io.Writer
	Conn        *WebSocket

	// important for drawing
	FirstDrawDone     bool
	ShouldQuit        bool
	HideHelp          bool
	KeepAutoscrolling bool

	// functions
	ExecuteFunc func(func(*gocui.Gui) error)

	Settings Settings
}

func clearBuffer(*gocui.Gui, *gocui.View) error {
	v := state.Writer.(*gocui.View)
	v.Clear()
	v.SetCursor(0, 0)
	v.SetOrigin(0, 0)
	return nil
}

// adds an action to LastActions
func (s *State) PushAction(act string) error {
	return s.Settings.PushAction(act)
}

// changes the ActionIndex and returns the value at the specified index.
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
func (s *State) StartConnection(url string) error {
	if s.Conn != nil {
		return errors.New("state: conn is not nil")
	}
	ws, err := CreateWebSocket(url)
	if err != nil {
		return err
	}
	s.Conn = ws
	connectionStarted = time.Now()
	go s.wsReader()
	return nil
}

func (s *State) wsReader() {
	ch := s.Conn.ReadChannel()
	for msg := range ch {
		s.Server(msg)
	}
}

var (
	printDebug  = color.New(color.FgCyan).Fprint
	printError  = color.New(color.FgRed).Fprint
	printUser   = color.New(color.FgGreen).Fprint
	printServer = color.New(color.FgWhite).Fprint
)

// Debug prints debug information to the Writer, using light blue.
func (s *State) Debug(x string) {
	s.printToOut(printDebug, x)
}

// Error prints an error to the Writer, using red.
func (s *State) Error(x string) {
	s.printToOut(printError, x)
}

// User prints user-provided messages to the Writer, using green.
func (s *State) User(x string) {

	oSet := s.Settings.Clone()

	res, err := s.pipe(x, "out", oSet.Pipe.Out)
	if err != nil {
		s.Error(err.Error())
		if res == "" || res == "\n" {
			return
		}
	}

	s.printToOut(printUser, res)
}

// Server prints server-returned messages to the Writer, using white.
func (s *State) Server(x string) {

	oSet := s.Settings.Clone()

	res, err := s.pipe(x, "in", oSet.Pipe.In)
	if err != nil {
		s.Error(err.Error())
		if res == "" || res == "\n" {
			return
		}
	}

	if oSet.JSONFormatting {
		res = attemptJSONFormatting(res)
	}

	s.printToOut(printServer, res)
}

var (
	sessionStarted    = time.Now()
	connectionStarted time.Time
)

func (s *State) pipe(data, t string, command []string) (string, error) {
	if len(command) < 1 {
		return data, nil
	}
	// prepare the command: create it, set up env variables
	c := exec.Command(command[0], command[1:]...)
	c.Env = append(
		os.Environ(),
		"CLAWS_PIPE_TYPE="+t,
		"CLAWS_SESSION="+strconv.FormatInt(sessionStarted.UnixNano()/1000, 10),
		"CLAWS_CONNECTION="+strconv.FormatInt(connectionStarted.UnixNano()/1000, 10),
		"CLAWS_WS_URL="+s.Conn.URL(),
	)
	// set up stdin
	stdin := strings.NewReader(data)
	c.Stdin = stdin

	// run the command
	res, err := c.Output()
	return string(res), err
}

func (s *State) printToOut(f func(io.Writer, ...interface{}) (int, error), str string) {

	oSet := s.Settings.Clone()

	if oSet.Timestamp != "" {
		str = time.Now().Format(oSet.Timestamp) + str
	}

	if str != "" && str[len(str)-1] != '\n' {
		str += "\n"
	}
	s.ExecuteFunc(func(*gocui.Gui) error {
		_, err := f(s.Writer, str)
		return err
	})
}
