package main

import (
	"strings"

	"github.com/jroimartin/gocui"
)

func editor(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	if ch != 0 && mod == 0 {
		v.EditWrite(ch)
	}

	switch key {
	// Space, backspace, Del
	case gocui.KeySpace:
		v.EditWrite(' ')
	case gocui.KeyBackspace, gocui.KeyBackspace2:
		v.EditDelete(true)
		moveAhead(v)
	case gocui.KeyDelete:
		v.EditDelete(false)

	// Cursor movement
	case gocui.KeyArrowLeft:
		v.MoveCursor(-1, 0, false)
		moveAhead(v)
	case gocui.KeyArrowRight:
		x, _ := v.Cursor()
		x2, _ := v.Origin()
		x += x2
		buf := v.Buffer()
		// I don't know really how this works, this was mostly obtained through trial
		// and error. Anyway, this system impedes going on a newline by moving right.
		// This is usually possible because once you write something to the buffer
		// it automatically adds " \n", which is two characters. Sooo yeah.
		if buf != "" && len(buf) > (x+2) {
			v.MoveCursor(1, 0, false)
		}

	// Insert/Overwrite
	case gocui.KeyInsert:
		if state.Mode == modeInsert {
			state.Mode = modeOverwrite
		} else {
			state.Mode = modeInsert
		}
		v.Overwrite = state.Mode == modeOverwrite

	// History browse
	case gocui.KeyArrowDown:
		n := state.BrowseActions(-1)
		setText(v, n)
	case gocui.KeyArrowUp:
		n := state.BrowseActions(1)
		setText(v, n)

	case gocui.KeyEnter:
		buf := v.Buffer()
		v.Clear()
		v.SetCursor(0, 0)
		if strings.TrimSpace(buf) == "" {
			return
		}

		buf = buf[:len(buf)-2]
		state.PushAction(buf)
		state.ActionIndex = -1
		if state.Conn != nil {
			state.User(buf)
			state.Conn.Write(buf)
		}

	}
}

func setText(v *gocui.View, text string) {
	v.Clear()
	// Why are we doing this? Because normally when you write a line
	// gocui adds " \n" at the end of it. Whe clearing and adding, though,
	// the space isn't added back.
	v.Write([]byte(text + " "))
	v.SetCursor(len(text), 0)
}

// moveAhead displays the next 10 characters when moving backwards,
// in order to see where we're moving or what we're deleting.
func moveAhead(v *gocui.View) {
	cX, _ := v.Cursor()
	oX, _ := v.Origin()
	if cX < 10 && oX > 0 {
		newOX := oX - 10
		forward := 10
		if newOX < 0 {
			forward += newOX
			newOX = 0
		}
		v.SetOrigin(newOX, 0)
		v.MoveCursor(forward, 0, false)
	}
}
