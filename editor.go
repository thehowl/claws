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
		state.KeepAutoscrolling = true

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
		// Position of cursor should be on space that gocui adds at the end if at end
		if buf != "" && len(strings.TrimRight(buf, "\r\n")) > x {
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
			buf = buf[:len(buf)-1]
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
	v.Write([]byte(text))
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

func moveDown(v *gocui.View) {
	_, yPos := v.Cursor()
	if _, err := v.Line(yPos + 1); err == nil {
		v.MoveCursor(0, 1, false)
	}
}

// escEditor handles keys when esc has been pressed.
func escEditor(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	switch key {
	case gocui.KeyEsc:
		// silently ignore - we're already in esc mode!
	case gocui.KeyInsert:
		state.Mode = modeInsert

	// Scrolling
	//
	// When one of the movements keys is pressed, autoscrolling is disabled so that
	// the user can move through the output without having text moving underneath
	// the cursor.
	case gocui.KeyArrowUp, gocui.MouseWheelDown:
		state.KeepAutoscrolling = false
		v.MoveCursor(0, -1, false)
	case gocui.KeyArrowDown, gocui.MouseWheelUp:
		state.KeepAutoscrolling = false
		moveDown(v)
	case gocui.KeyArrowLeft:
		state.KeepAutoscrolling = false
		v.MoveCursor(-1, 0, false)
	case gocui.KeyArrowRight:
		state.KeepAutoscrolling = false
		_, y := v.Cursor()
		if _, err := v.Word(0, y); err == nil {
			v.MoveCursor(1, 0, false)
		}
	case gocui.KeyPgup:
		state.KeepAutoscrolling = false
		_, ySize := v.Size()
		for i := 0; i < ySize; i++ {
			v.MoveCursor(0, -1, false)
		}
	case gocui.KeyPgdn:
		state.KeepAutoscrolling = false
		_, ySize := v.Size()
		for i := 0; i < ySize; i++ {
			moveDown(v)
		}
	case gocui.KeyHome:
		state.KeepAutoscrolling = false
		v.SetCursor(0, 0)
		v.SetOrigin(0, 0)
	case gocui.KeyEnd:
		state.KeepAutoscrolling = false
		lines := len(strings.Split(v.ViewBuffer(), "\n"))
		_, y := v.Size()

		origin := lines - y - 1
		if origin < 0 {
			origin = 0
		}
		v.SetOrigin(0, origin)

		cursorY := y - 1
		if lines <= y {
			cursorY = lines - 2
		}
		v.SetCursor(0, cursorY)
	}
	if ch == 0 {
		return
	}

	switch ch {
	case 'c':
		if state.Conn != nil {
			state.Error("You need to close the connection before starting a new one: <ESC>q")
			return
		}
		state.Mode = modeConnect
		return
	case 'q':
		err := state.Conn.Close()
		if err != nil {
			state.Error(err.Error())
		}
		state.Debug("WebSocket closed")
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
