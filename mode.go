package main

import (
	"github.com/jroimartin/gocui"
)

type UIMode int

// List of all modes
const (
	modeInsert    UIMode = iota
	modeOverwrite UIMode = iota
	modeEscape    UIMode = iota
	modeConnect   UIMode = iota
	modeSetPing   UIMode = iota
)

type ModeStyle struct {
	Char    rune
	BgColor gocui.Attribute
}

var modeChars = map[UIMode]ModeStyle{
	modeInsert:    ModeStyle{' ', gocui.ColorGreen},
	modeOverwrite: ModeStyle{'R', gocui.ColorGreen},
	modeEscape:    ModeStyle{' ', gocui.ColorRed},
	modeConnect:   ModeStyle{'c', gocui.ColorRed},
	modeSetPing:   ModeStyle{'p', gocui.ColorRed},
}

func modeBox(pSt *State, g *gocui.Gui) {

	maxX, maxY := g.Size()

	for i := 0; i < maxX; i++ {
		g.SetRune(i, maxY-2, '─', gocui.ColorWhite, gocui.ColorBlack)
	}

	ch := modeChars[pSt.Mode]
	g.SetRune(0, maxY-1, ch.Char, gocui.ColorWhite|gocui.AttrBold, ch.BgColor)
	g.SetRune(1, maxY-1, ' ', gocui.ColorBlack, 0)
}
