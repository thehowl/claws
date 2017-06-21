package main

import (
	"io"

	"github.com/fatih/color"
)

var state = &State{
	ActionIndex: -1,
}

// State is the central function managing the information of claws.
type State struct {
	LastActions []string
	Mode        int
	ActionIndex int
	Writer      io.Writer
}

// PushAction adds an action to LastActions
func (s *State) PushAction(act string) {
	s.LastActions = append([]string{act}, s.LastActions...)
	if len(s.LastActions) > 100 {
		s.LastActions = s.LastActions[:100]
	}
}

// BrowseActions changes the ActionIndex and returns the value at the specified index.
// move is the number of elements to move (negatives go into more recent history,
// 0 returns the current element, positives go into older history)
func (s *State) BrowseActions(move int) string {
	s.ActionIndex += move
	if s.ActionIndex >= len(s.LastActions) {
		s.ActionIndex = len(s.LastActions) - 1
	} else if s.ActionIndex < -1 {
		s.ActionIndex = -1
	}

	// -1 always indicates the "next" element, thus empty
	if s.ActionIndex == -1 {
		return ""
	}
	return s.LastActions[s.ActionIndex]
}

var (
	printDebug  = color.New(color.FgCyan).Fprint
	printUser   = color.New(color.FgGreen).Fprint
	printServer = color.New(color.FgWhite).Fprint
)

// Debug prints debug information to the Writer, using light blue.
func (s *State) Debug(x string) {
	printDebug(s.Writer, x+"\n")
}

// User prints user-provided messages to the Writer, using green.
func (s *State) User(x string) {
	printUser(s.Writer, x+"\n")
}

// Server prints server-returned messages to the Writer, using white.
func (s *State) Server(x string) {
	printServer(s.Writer, x+"\n")
}
