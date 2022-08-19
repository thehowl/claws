package main

import (
	"fmt"
	"os"

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
			fmt.Fprintln(os.Stderr, E)
			os.Exit(1)
		}
	}()

	oState := State{
		ActionIndex: -1,
		HideHelp:    len(os.Args) > 1,
	}

	// LOAD CONFIG FROM claws.json
	if oState.Settings, E = LoadSettings(); E != nil {
		return
	}

	// CMDLINE CONFIGURATION + HELP
	// MERGE CMDLINE FLAGS INTO SETTINGS
	if E = oState.Settings.ParseFlags(); E != nil {
		return
	}

	g, E := gocui.NewGui(gocui.OutputNormal)
	if E != nil {
		return
	}
	defer g.Close()

	oState.ExecuteFunc = g.Update

	fnLayout := NewLayoutFunc(&oState)
	g.SetManagerFunc(fnLayout)
	g.Cursor = true
	g.InputEsc = true

	if E = g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); E != nil {
		return
	}

	fnClearBuf := func(*gocui.Gui, *gocui.View) error {
		v := oState.Writer.(*gocui.View)
		v.Clear()
		v.SetCursor(0, 0)
		v.SetOrigin(0, 0)
		return nil
	}

	if E = g.SetKeybinding("", gocui.KeyCtrlL, gocui.ModNone, fnClearBuf); E != nil {
		return
	}

	E = g.MainLoop()
	if E == gocui.ErrQuit {
		E = nil
	}
}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}
