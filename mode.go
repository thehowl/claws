package main

import (
	"github.com/jroimartin/gocui"
)

// List of all modes
const (
	modeInsert = iota
	modeOverwrite
	modeEscape
	modeConnect
)

var modeChars = []struct {
	c  rune
	bg gocui.Attribute
}{
	{' ', gocui.ColorGreen},
	{'R', gocui.ColorGreen},
	{' ', gocui.ColorRed},
	{'c', gocui.ColorRed},
}

func modeBox(g *gocui.Gui) {
	maxX, maxY := g.Size()

	for i := 0; i < maxX; i++ {
		g.SetRune(i, maxY-2, '─', gocui.ColorWhite, gocui.ColorBlack)
	}

	ch := modeChars[state.Mode]
	g.SetRune(0, maxY-1, ch.c, gocui.ColorWhite|gocui.AttrBold, ch.bg)
	g.SetRune(1, maxY-1, ' ', gocui.ColorBlack, 0)
}
