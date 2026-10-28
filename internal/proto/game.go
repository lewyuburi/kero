package proto

import "strings"

// gameScheme is what a pane's URL starts with when it plays a game, not a
// page: the daemon keeps it as it keeps a browser's, with no process, and
// the app draws the game (games.go).
const gameScheme = "kero-game://"

// GameURL is the URL of a pane playing the named game.
func GameURL(name string) string { return gameScheme + name }

// Game is the name of the game the pane plays, "" for a pane that does not.
func (p *Pane) Game() string {
	if name, ok := strings.CutPrefix(p.URL, gameScheme); ok {
		return strings.Trim(name, "/")
	}
	return ""
}
