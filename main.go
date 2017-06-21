package main

import (
	"log"
	"os"

	"github.com/jroimartin/gocui"
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
	if v, err := g.SetView("out", -1, -1, maxX, maxY-2); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Autoscroll = true
		v.Wrap = true
		state.Writer = v
	}

	g.Cursor = !notInsert[state.Mode]

	if _, err := g.SetCurrentView("cmd"); err != nil {
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
