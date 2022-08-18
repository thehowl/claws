package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/jroimartin/gocui"
)

var (
	version = "devel"
	commit  = ""
)

func main() {

	var E error
	defer func() {
		if E != nil {
			log.Panicln(E)
		}
	}()

	// LOAD CONFIG FROM claws.json
	if state.Settings, E = LoadSettings(); E != nil {
		return
	}

	// CMDLINE CONFIGURATION + HELP
	// MERGE CMDLINE FLAGS INTO SETTINGS
	if E = state.Settings.ParseFlags(); E != nil {
		return
	}

	g, E := gocui.NewGui(gocui.OutputNormal)
	if E != nil {
		return
	}
	defer g.Close()

	state.ExecuteFunc = g.Update

	g.SetManagerFunc(layout)
	g.Cursor = true
	g.InputEsc = true

	if E = g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); E != nil {
		return
	}

	if E = g.SetKeybinding("", gocui.KeyCtrlL, gocui.ModNone, clearBuffer); E != nil {
		return
	}

	E = g.MainLoop()
	if E == gocui.ErrQuit {
		E = nil
	}
}

const welcomeScreen = `
                claws %s
          Awesome WebSocket CLient

    C-c           to quit
    <ESC>c        to write an URL of a
                  websocket and connect
    <ESC>q        to close the websocket
    <UP>/<DOWN>   move through your history

           https://howl.moe/claws
`

func layout(g *gocui.Gui) error {
	// Set when doing a double-esc
	if state.ShouldQuit {
		return gocui.ErrQuit
	}

	maxX, maxY := g.Size()
	if v, err := g.SetView("cmd", 1, maxY-2, maxX, maxY); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Frame = false
		v.Editable = true
		v.Editor = gocui.EditorFunc(editor)
		v.Clear()
	}
	v, err := g.SetView("out", -1, -1, maxX, maxY-2)
	if err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Wrap = true
		v.Editor = gocui.EditorFunc(editor)
		v.Editable = true
		state.Writer = v
	}
	// For more information about KeepAutoscrolling, see Scrolling in editor.go
	v.Autoscroll = state.Mode != modeEscape || state.KeepAutoscrolling
	g.Mouse = state.Mode == modeEscape
	if v, err := g.SetView("help", maxX/2-23, maxY/2-6, maxX/2+23, maxY/2+6); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Wrap = true
		v.Title = "Welcome"
		if version == "devel" && commit != "" {
			version = commit
			if len(version) > 5 {
				version = version[:5]
			}
		}
		v.Write([]byte(fmt.Sprintf(welcomeScreen, version)))
	}

	if state.HideHelp {
		g.SetViewOnTop("out")
	} else {
		g.SetViewOnTop("help")
	}

	curView := "cmd"
	if state.Mode == modeEscape {
		curView = "out"
	}

	if _, err := g.SetCurrentView(curView); err != nil {
		return err
	}

	modeBox(g)

	if !state.FirstDrawDone {
		go connectWs()
		state.FirstDrawDone = true
	}

	return nil
}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}

func connectWs() {

	url := strings.TrimSpace(state.Settings.LastWebsocketURL)
	if len(url) == 0 {
		return
	}

	if E := state.StartConnection(url); E != nil {
		state.Error(E.Error())
	}
}
