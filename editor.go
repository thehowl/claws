package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jroimartin/gocui"
)

type LayoutFunc func(*gocui.Gui) error

func NewLayoutFunc(pSt *State) LayoutFunc {

	fnEditor := NewEditorFunc(pSt)

	return func(g *gocui.Gui) error {

		// Set when doing a double-esc
		if pSt.ShouldQuit {
			return gocui.ErrQuit
		}

		maxX, maxY := g.Size()
		if v, err := g.SetView("cmd", 1, maxY-2, maxX, maxY); err != nil {
			if err != gocui.ErrUnknownView {
				return err
			}
			v.Frame = false
			v.Editable = true
			v.Editor = gocui.EditorFunc(fnEditor)
			v.Clear()
		}

		v, err := g.SetView("out", -1, -1, maxX, maxY-2)
		if err != nil {
			if err != gocui.ErrUnknownView {
				return err
			}
			v.Wrap = true
			v.Editor = gocui.EditorFunc(fnEditor)
			v.Editable = true
			pSt.Writer = v
		}

		// For more information about KeepAutoscrolling, see Scrolling in editor.go
		v.Autoscroll = pSt.Mode != modeEscape || pSt.KeepAutoscrolling
		g.Mouse = pSt.Mode == modeEscape
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

		if pSt.HideHelp {
			g.SetViewOnTop("out")
		} else {
			g.SetViewOnTop("help")
		}

		curView := "cmd"
		if pSt.Mode == modeEscape {
			curView = "out"
		}

		if _, err := g.SetCurrentView(curView); err != nil {
			return err
		}

		modeBox(pSt, g)

		if !pSt.FirstDrawDone {
			go pSt.StartConnection("")
			pSt.FirstDrawDone = true
		}

		return nil
	}
}

type ActionFunc func(*State, string)

// enterActions is the actions that can be done when KeyEnter is pressed
// (outside of modeEscape), based on the mode.
var enterActions = map[UIMode]ActionFunc{
	modeInsert:    enterActionSendMessage,
	modeOverwrite: enterActionSendMessage,
	modeConnect:   enterActionConnect,
	modeSetPing:   enterActionSetPing,
}

type EditorFunc func(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier)

func NewEditorFunc(pSt *State) EditorFunc {

	return func(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {

		pSt.HideHelp = true

		if pSt.Mode == modeEscape {
			escEditor(pSt, v, key, ch, mod)
			return
		}

		if ch != 0 && mod == 0 {
			v.EditWrite(ch)
		}

		switch key {
		case gocui.KeyEsc:
			pSt.Mode = modeEscape
			pSt.KeepAutoscrolling = true

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
			if pSt.Mode == modeInsert {
				pSt.Mode = modeOverwrite
			} else {
				pSt.Mode = modeInsert
			}
			v.Overwrite = pSt.Mode == modeOverwrite

		// History browse
		case gocui.KeyArrowDown:
			n := pSt.BrowseActions(-1)
			setText(v, n)
		case gocui.KeyArrowUp:
			n := pSt.BrowseActions(1)
			setText(v, n)

		case gocui.KeyEnter:
			buf := v.Buffer()
			v.Clear()
			v.SetCursor(0, 0)

			if buf != "" {
				buf = buf[:len(buf)-1]
			}
			if strings.TrimSpace(buf) != "" {
				pSt.PushAction(buf)
				pSt.ActionIndex = -1
			}

			enterActions[pSt.Mode](pSt, buf)
		}
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

func enterActionSetPing(pSt *State, buf string) {

	secs, E := strconv.Atoi(strings.TrimSpace(buf))
	if E != nil {
		secs = 0
	}

	pSt.SetPingInterval(secs)

	pSt.Mode = modeInsert
}

func enterActionSendMessage(pSt *State, buf string) {

	if strings.TrimSpace(buf) != "" {
		pSt.DisplayInputFromUser(buf)
		pSt.WsSendMsg(buf)
	}
}

func enterActionConnect(pSt *State, buf string) {

	pSt.Mode = modeInsert
	go pSt.StartConnection(buf)
}

func moveDown(v *gocui.View) {
	_, yPos := v.Cursor()
	if _, err := v.Line(yPos + 1); err == nil {
		v.MoveCursor(0, 1, false)
	}
}

// escEditor handles keys when esc has been pressed.
func escEditor(pSt *State, v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {

	switch key {
	case gocui.KeyEsc:
		// silently ignore - we're already in esc mode!
	case gocui.KeyInsert:
		pSt.Mode = modeInsert

	// Scrolling
	//
	// When one of the movements keys is pressed, autoscrolling is disabled so that
	// the user can move through the output without having text moving underneath
	// the cursor.
	case gocui.KeyArrowUp, gocui.MouseWheelDown:
		pSt.KeepAutoscrolling = false
		v.MoveCursor(0, -1, false)
	case gocui.KeyArrowDown, gocui.MouseWheelUp:
		pSt.KeepAutoscrolling = false
		moveDown(v)
	case gocui.KeyArrowLeft:
		pSt.KeepAutoscrolling = false
		v.MoveCursor(-1, 0, false)
	case gocui.KeyArrowRight:
		pSt.KeepAutoscrolling = false
		_, y := v.Cursor()
		if _, err := v.Word(0, y); err == nil {
			v.MoveCursor(1, 0, false)
		}
	case gocui.KeyPgup:
		pSt.KeepAutoscrolling = false
		_, ySize := v.Size()
		for i := 0; i < ySize; i++ {
			v.MoveCursor(0, -1, false)
		}
	case gocui.KeyPgdn:
		pSt.KeepAutoscrolling = false
		_, ySize := v.Size()
		for i := 0; i < ySize; i++ {
			moveDown(v)
		}
	case gocui.KeyHome:
		pSt.KeepAutoscrolling = false
		v.SetCursor(0, 0)
		v.SetOrigin(0, 0)
	case gocui.KeyEnd:
		pSt.KeepAutoscrolling = false
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
		if pSt.WsIsOpen() {
			pSt.Error(errors.New("You need to close the connection before starting a new one: <ESC>q"))
			return
		}
		pSt.Mode = modeConnect
		return
	case 'p':
		pSt.Mode = modeSetPing
		return
	case 'q':
		if err := pSt.WsClose(); len(err) > 0 {
			for _, e := range err {
				pSt.Error(e)
			}
		}
		pSt.Debug("WebSocket closed (use C-c to quit)")
		return
	case 'i':
		// goes into insert mode
	case 'h':
		pSt.HideHelp = false
	case 'j':
		// toggle JSON formatting
		pSt.Settings.JSONFormatting = !pSt.Settings.JSONFormatting
		err := pSt.Settings.Update("JSONFormatting")
		if err != nil {
			pSt.Error(err)
		}
		e := "disabled"
		if pSt.Settings.JSONFormatting {
			e = "enabled"
		}
		pSt.Debug("JSON formatting " + e)
	case 't':
		// toggle timestamps
		if pSt.Settings.Timestamp == "" {
			pSt.Settings.Timestamp = "2006-01-02 15:04:05 "
		} else {
			pSt.Settings.Timestamp = ""
		}
		err := pSt.Settings.Update("Timestamp")
		if err != nil {
			pSt.Error(err)
		}
		e := "disabled"
		if pSt.Settings.Timestamp != "" {
			e = "enabled"
		}
		pSt.Debug("Timestamps " + e)
	case 'R':
		// overwrite mode
		pSt.Mode = modeOverwrite
		return
	default:
		pSt.Debug("No action for key '" + string(ch) + "'")
		return
	}

	pSt.Mode = modeInsert
}

const welcomeScreen = `
                claws %s
          Awesome WebSocket CLient

    C-c           to quit
    <ESC>c        to write an URL of a
                  websocket and connect
    <ESC>q        to close the websocket
    <ESC>p        set ping interval in seconds
    <UP>/<DOWN>   move through your history

           https://howl.moe/claws
`
