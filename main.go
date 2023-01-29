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
	var err error
	defer func() {
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}()

	oState := State{
		ActionIndex: -1,
		HideHelp:    len(os.Args) > 1,
	}

	// load config from claws.json
	if oState.Settings, err = LoadSettings(); err != nil {
		return
	}

	// cmdline configuration + help
	// merge cmdline flags into settings
	if err = oState.Settings.ParseFlags(); err != nil {
		return
	}

	g, err := gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		return
	}
	defer g.Close()

	oState.ExecuteFunc = g.Update

	fnLayout := NewLayoutFunc(&oState)
	g.SetManagerFunc(fnLayout)
	g.Cursor = true
	g.InputEsc = true

	if err = g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
		return
	}

	fnClearBuf := func(*gocui.Gui, *gocui.View) error {
		v := oState.Writer.(*gocui.View)
		v.Clear()
		v.SetCursor(0, 0)
		v.SetOrigin(0, 0)
		return nil
	}

	if err = g.SetKeybinding("", gocui.KeyCtrlL, gocui.ModNone, fnClearBuf); err != nil {
		return
	}

	err = g.MainLoop()
	if err == gocui.ErrQuit {
		err = nil
	}
}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}
