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

	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
		log.Panicln(err)
	}

	if err := g.MainLoop(); err != nil && err != gocui.ErrQuit {
		log.Panicln(err)
	}
}

func layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	if v, err := g.SetView("cmd", 1, maxY-2, maxX, maxY); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Frame = false
		v.Editable = true
		v.Editor = gocui.EditorFunc(editor)
		v.Clear()
		if _, err := g.SetCurrentView("cmd"); err != nil {
			return err
		}
	}
	if v, err := g.SetView("out", -1, -1, maxX, maxY-2); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Autoscroll = true
		v.Wrap = true
		state.Writer = v
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
	if len(os.Args) > 1 {
		wsURL := os.Args[1]
		err := state.StartConnection(wsURL)
		if err != nil {
			state.Error(err.Error())
		}
	}
}
