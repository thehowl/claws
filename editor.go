package main

import (
	"strings"

	"github.com/jroimartin/gocui"
)

func editor(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	state.HideHelp = true

	if state.Mode == modeEscape {
		escEditor(v, key, ch, mod)
		return
	}

	if ch != 0 && mod == 0 {
		v.EditWrite(ch)
	}

	switch key {
	case gocui.KeyEsc:
		state.Mode = modeEscape

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

		if buf != "" {
			buf = buf[:len(buf)-2]
		}
		if strings.TrimSpace(buf) != "" {
			state.PushAction(buf)
			state.ActionIndex = -1
		}

		enterActions[state.Mode](buf)
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

// enterActions is the actions that can be done when KeyEnter is pressed
// (outside of modeEscape), based on the mode.
var enterActions = map[int]func(buf string){
	modeInsert:    enterActionSendMessage,
	modeOverwrite: enterActionSendMessage,
	modeConnect:   enterActionConnect,
}

func enterActionSendMessage(buf string) {
	if state.Conn != nil && strings.TrimSpace(buf) != "" {
		state.User(buf)
		state.Conn.Write(buf)
	}
}

func enterActionConnect(buf string) {
	if buf != "" {
		state.Settings.LastWebsocketURL = buf
		state.Settings.Save()
	}
	state.Mode = modeInsert
	go connect()
}

// escEditor handles keys when esc has been pressed.
func escEditor(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	switch key {
	case gocui.KeyEsc:
		state.ShouldQuit = true
		return
	case gocui.KeyInsert:
		state.Mode = modeInsert
		return
	}

	switch ch {
	case 'c':
		state.Mode = modeConnect
		return
	case 'q':
		err := state.Conn.Close()
		if err != nil {
			state.Error(err.Error())
		}
		return
	case 'i':
		// goes into insert mode
	case 'h':
		state.HideHelp = false
	case 'j':
		// toggle JSON formatting
		state.Settings.JSONFormatting = !state.Settings.JSONFormatting
		err := state.Settings.Save()
		if err != nil {
			state.Error(err.Error())
		}
		e := "disabled"
		if state.Settings.JSONFormatting {
			e = "enabled"
		}
		state.Debug("JSON formatting " + e)
	case 't':
		// toggle timestamps
		if state.Settings.Timestamp == "" {
			state.Settings.Timestamp = "2006-01-02 15:04:05 "
		} else {
			state.Settings.Timestamp = ""
		}
		err := state.Settings.Save()
		if err != nil {
			state.Error(err.Error())
		}
		e := "disabled"
		if state.Settings.Timestamp != "" {
			e = "enabled"
		}
		state.Debug("Timestamps " + e)
	case 'R':
		// overwrite mode
		state.Mode = modeOverwrite
		return
	default:
		state.Debug("No action for key '" + string(ch) + "'")
		return
	}

	state.Mode = modeInsert
}
