package main

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/user"
	"time"

	"github.com/fatih/color"
	"github.com/jroimartin/gocui"
)

var state = &State{
	ActionIndex: -1,
	Settings:    &Settings{},
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

	Settings *Settings
}

// PushAction adds an action to LastActions
func (s *State) PushAction(act string) {
	s.Settings.LastActions = append([]string{act}, s.Settings.LastActions...)
	if len(s.Settings.LastActions) > 100 {
		s.Settings.LastActions = s.Settings.LastActions[:100]
	}
	s.Settings.Save()
}

// BrowseActions changes the ActionIndex and returns the value at the specified index.
// move is the number of elements to move (negatives go into more recent history,
// 0 returns the current element, positives go into older history)
func (s *State) BrowseActions(move int) string {
	s.ActionIndex += move
	if s.ActionIndex >= len(s.Settings.LastActions) {
		s.ActionIndex = len(s.Settings.LastActions) - 1
	} else if s.ActionIndex < -1 {
		s.ActionIndex = -1
	}

	// -1 always indicates the "next" element, thus empty
	if s.ActionIndex == -1 {
		return ""
	}
	return s.Settings.LastActions[s.ActionIndex]
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
	go s.wsReader()
	return nil
}

func (s *State) wsReader() {
	ch := s.Conn.ReadChannel()
	for msg := range ch {
		if s.Settings.JSONFormatting {
			msg = attemptJSONFormatting(msg)
		}
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
	s.printToOut(printUser, x)
}

// Server prints server-returned messages to the Writer, using white.
func (s *State) Server(x string) {
	s.printToOut(printServer, x)
}

func (s *State) printToOut(f func(io.Writer, ...interface{}) (int, error), str string) {
	if s.Settings.Timestamp != "" {
		str = time.Now().Format(s.Settings.Timestamp) + str
	}

	if str != "" && str[len(str)-1] != '\n' {
		str += "\n"
	}
	s.ExecuteFunc(func(*gocui.Gui) error {
		_, err := f(s.Writer, str)
		return err
	})
}

// Settings contains persistent information about the usage of claws.
type Settings struct {
	JSONFormatting   bool
	Timestamp        string
	LastWebsocketURL string
	LastActions      []string
}

// Load loads settings from ~/.config/claws.json
func (s *Settings) Load() error {
	folder, err := getConfigFolder()
	if err != nil {
		return err
	}

	f, err := os.Open(folder + "claws.json")
	if err != nil {
		// silently ignore ErrNotExist
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	return json.NewDecoder(f).Decode(s)
}

// Save saves settings to ~/.config/claws.json
func (s Settings) Save() error {
	folder, err := getConfigFolder()
	if err != nil {
		return err
	}

	f, err := os.Create(folder + "claws.json")
	if err != nil {
		return err
	}
	defer f.Close()

	e := json.NewEncoder(f)
	e.SetIndent("", "\t")
	return e.Encode(s)
}

func getConfigFolder() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	folder := u.HomeDir + "/.config/"

	err = os.MkdirAll(folder, 0755)

	return folder, err
}
