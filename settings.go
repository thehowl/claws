package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/user"
	"strings"
	"sync"

	"github.com/fatih/structs"
)

/*
	TODO: issues

		! goroutine safety (esp global Conn pointer)
		- done/wait channels for read/write pumps
		- binary messages
		- pipe persistence
*/

func getConfigFolder() (string, error) {

	u, err := user.Current()
	if err != nil {
		return "", err
	}
	folder := u.HomeDir + "/.config/"

	err = os.MkdirAll(folder, 0755)

	return folder, err
}

type SettingsBase struct {
	Info             string
	JSONFormatting   bool
	Timestamp        string
	LastWebsocketURL string
	LastActions      []string
	PingSeconds      int
	Pipe             struct {
		In  []string
		Out []string
	}
}

// persistent information about the usage of claws
type Settings struct {
	SettingsBase
	sync.RWMutex `json:"-"`
}

// for goroutine-safe read access to settings
func (s *Settings) Clone() SettingsBase {

	s.RLock()
	defer s.RUnlock()

	RET := s.SettingsBase

	fnCopy := func(dst *[]string, src []string) {
		if src == nil {
			return
		}
		*dst = make([]string, len(src))
		copy(*dst, src)
	}
	fnCopy(&RET.LastActions, s.LastActions)
	fnCopy(&RET.Pipe.In, s.Pipe.In)
	fnCopy(&RET.Pipe.Out, s.Pipe.Out)

	return RET
}

// loads settings from ~/.config/claws.json
func LoadSettings() (oSet Settings, E error) {

	folder, E := getConfigFolder()
	if E != nil {
		return
	}

	f, E := os.Open(folder + "claws.json")
	if E != nil {
		// silently ignore ErrNotExist
		if os.IsNotExist(E) {
			E = nil
			return
		}
		return
	}
	defer f.Close()

	E = json.NewDecoder(f).Decode(&oSet.SettingsBase)
	return
}

// saves settings to ~/.config/claws.json
func (s *Settings) Save() error {

	folder, err := getConfigFolder()
	if err != nil {
		return err
	}

	f, err := os.Create(folder + "claws.json")
	if err != nil {
		return err
	}
	defer f.Close()

	s.RLock()
	defer s.RUnlock()

	s.Info = "Claws configuration file; more information can be found at https://howl.moe/claws"
	e := json.NewEncoder(f)
	e.SetIndent("", "\t")
	return e.Encode(s.SettingsBase)
}

// applies ONLY specified fields of current settings to claws.json
func (s *Settings) Update(fields ...string) error {

	oPrev, E := LoadSettings()
	if E != nil {
		return E
	}

	s.RLock()
	defer s.RUnlock()

	strDst := structs.New(&oPrev)
	strSrc := structs.New(s)

	bDirty := false
	for _, fldName := range fields {

		if fldDst, ok := strDst.FieldOk(fldName); ok {

			fldSrc := strSrc.Field(fldName)
			E = fldDst.Set(fldSrc.Value())
			if E != nil {
				return E
			}

			bDirty = true
		}
	}

	if bDirty {
		return oPrev.Save()
	}

	return nil
}

// adds an action to LastActions
func (s *Settings) PushAction(act string) error {

	s.Lock()
	s.LastActions = append([]string{act}, s.LastActions...)
	if len(s.LastActions) > 100 {
		s.LastActions = s.LastActions[:100]
	}
	s.Unlock()

	return s.Update("LastActions")
}

// displays CLI `--help` information
// writes specified flags/opts into settings
func (pSet *Settings) ParseFlags() error {

	// HELP MESSAGE
	flag.Usage = func() {

		fmt.Fprint(os.Stdout, SZ_HELP_PREFIX)
		flag.PrintDefaults()
		fmt.Fprint(os.Stdout, SZ_HELP_SUFFIX)
		fmt.Fprint(os.Stdout, "\n")
	}

	flag.BoolVar(&pSet.JSONFormatting, "j", pSet.JSONFormatting, "Start with JSON formatting enabled.")
	flag.StringVar(&pSet.Timestamp, "t", pSet.Timestamp, "Golang date format for timestamps.\nDisabled when blank.")
	flag.IntVar(&pSet.PingSeconds, "p", pSet.PingSeconds, "PING interval.\nDisabled when <= 0.")

	flag.Parse()

	// GRAB WEBSOCKET URL IF PRESENTED
	sArgs := flag.Args()
	for _, wsurl := range sArgs {
		wsurl := strings.TrimSpace(wsurl)
		if len(wsurl) > 0 {
			pSet.LastWebsocketURL = wsurl
			return pSet.Update("LastWebsocketURL")
		}
	}

	return nil
}

const SZ_HELP_PREFIX = `COMMAND

  claws [OPTION...] [WEBSOCKET_URL]

OPTIONS

`

const SZ_HELP_SUFFIX = `
USAGE

  Key Action
  --- ---------------------------------------------------------------
  Esc Enter command mode. (<Ctrl-[> also works)
  c   Create a new connection. Prompts for WebSocket URL.
      If nothing is passed, previous URL will be used.
  h   View help/welcome screen with quick commands.
  i   Go to insert mode. (<Ins> key also works)
  j   Toggle auto-detection of JSON in server messages and
      automatic tab indentation.
  p   Set ping interval in seconds.  Will prompt for an interval.
      If nothing is passed, pings will be disabled.
  q   Close current connection.
  R   Go into replace/overtype mode.
      (can also be done by pressing <Ins> a couple of times)
  t   Toggle timestamps before messages in console.
`
