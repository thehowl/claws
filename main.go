package main

import (
	"fmt"
	"log"
	"os"

	"github.com/jroimartin/gocui"
)

var (
	version = "devel"
	commit  = ""
)

func main() {
	g, err := gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		log.Panicln(err)
	}
	defer g.Close()

	state.ExecuteFunc = g.Execute

	g.SetManagerFunc(layout)
	g.Cursor = true
	g.InputEsc = true

	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
		log.Panicln(err)
	}

	if err := g.MainLoop(); err != nil && err != gocui.ErrQuit {
		log.Panicln(err)
	}
}

const welcomeScreen = `
                claws %s
          Awesome WebSocket CLient

    <ESC><ESC>    to quit
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
		go initialise()
		state.FirstDrawDone = true
	}

	return nil
}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}

func initialise() {
	err := state.Settings.Load()
	if err != nil {
		state.Error(err.Error())
	}

	if len(os.Args) > 1 && os.Args[1] != "" {
		state.Settings.LastWebsocketURL = os.Args[1]
		state.Settings.Save()
		connect()
	}
}

func connect() {
	err := state.StartConnection(state.Settings.LastWebsocketURL)
	if err != nil {
		state.Error(err.Error())
	}
}
