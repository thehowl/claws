package main

import (
	"github.com/jroimartin/gocui"
)

type UIMode int

// List of all modes
const (
	modeInsert UIMode = iota
	modeOverwrite
	modeEscape
	modeConnect
	modeSetPing
	modeMax
)

type ModeStyle struct {
	Char    rune
	BgColor gocui.Attribute
	Descr   string
}

var modeChars = [modeMax]ModeStyle{
	modeInsert:    ModeStyle{' ', gocui.ColorGreen, "INS"},
	modeOverwrite: ModeStyle{'R', gocui.ColorGreen, "OVR"},
	modeEscape:    ModeStyle{' ', gocui.ColorRed, "ESC"},
	modeConnect:   ModeStyle{'c', gocui.ColorRed, "CON"},
	modeSetPing:   ModeStyle{'p', gocui.ColorRed, "PNG"},
}
