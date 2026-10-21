package main

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"kero/internal/settings"

	"kero/locales"
)

// Chords are the keybinds that are more than one key of the app's menus: a
// sequence of chords (ctrl+a, then c) and the chords of a key mode, a named
// set the user enters and leaves (resize: the arrows move a divider). The
// terminal's keys come here first (terminal.Options.OnKey), so a key a
// binding takes does not reach the program; every other does.

// chordTimeout is how long a sequence waits for its next chord.
const chordTimeout = 1500 * time.Millisecond

// chord is a key and the modifiers held with it.
type chord struct {
	mods ui.Modifiers
	key  ui.Key
}

// chordBinding is a settings.Binding with its chords read as keys.
type chordBinding struct {
	mode   string
	keys   []chord
	action string
}

// parseAccel reads an accelerator, as "CmdOrCtrl+Shift+D", as the key and
// modifiers the window reports for it.
func parseAccel(accel string) (chord, bool) {
	parts := strings.Split(accel, "+")
	var c chord
	for _, p := range parts[:len(parts)-1] {
		switch strings.ToLower(p) {
		case "cmdorctrl", "commandorcontrol":
			c.mods |= ui.Cmd
		case "cmd", "command", "super", "meta", "win":
			c.mods |= ui.Super
		case "ctrl", "control":
			c.mods |= ui.Ctrl
		case "alt", "option":
			c.mods |= ui.Alt
		case "shift":
			c.mods |= ui.Shift
		}
	}
	name := parts[len(parts)-1]
	if name == "" && len(parts) > 1 { // "CmdOrCtrl++"
		name = "+"
	}
	for k, n := range keyNames {
		if strings.EqualFold(n, name) {
			c.key = k
			return c, true
		}
	}
	return chord{}, false
}

// compileBindings reads the chords of bindings; one whose key is none the
// window knows is left out.
func compileBindings(bs []settings.Binding) []chordBinding {
	var out []chordBinding
next:
	for _, b := range bs {
		cb := chordBinding{mode: b.Mode, action: b.Action}
		for _, accel := range b.Keys {
			c, ok := parseAccel(accel)
			if !ok {
				continue next
			}
			cb.keys = append(cb.keys, c)
		}
		out = append(out, cb)
	}
	return out
}

// chordState is where the user is among the bindings: in a mode, part of the
// way through a sequence.
type chordState struct {
	mode    string
	pending []chord
	at      time.Time
}

// active tells that a key may go to a binding: the user is in a mode or in
// the middle of a sequence.
func (s *chordState) active() bool { return s.mode != "" || len(s.pending) > 0 }

// step takes a key at a time, and says whether the bindings took it, and the
// action they ran with it, if one. A key that is none of the bindings' is
// the terminal's, unless the user is in a mode or in the middle of a
// sequence: then it is taken and does nothing, so that what was meant for
// the mode does not go to the program; Escape leaves a mode that binds it to
// nothing, and cancels a sequence.
func (s *chordState) step(binds []chordBinding, c chord, now time.Time) (taken bool, action string) {
	if len(s.pending) > 0 && now.Sub(s.at) > chordTimeout {
		s.pending = nil
	}
	seq := append(append([]chord(nil), s.pending...), c)
	prefix := false
	for _, b := range binds {
		if b.mode != s.mode || len(b.keys) < len(seq) {
			continue
		}
		if !equalChords(b.keys[:len(seq)], seq) {
			continue
		}
		if len(b.keys) == len(seq) {
			s.pending = nil
			return true, b.action
		}
		prefix = true
	}
	if prefix {
		s.pending, s.at = seq, now
		return true, ""
	}
	pending := len(s.pending) > 0
	s.pending = nil
	switch {
	case pending:
		return true, ""
	case s.mode != "":
		if c == (chord{key: ui.KeyEscape}) {
			s.mode = ""
		}
		return true, ""
	}
	return false, ""
}

func equalChords(a, b []chord) bool {
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return len(a) == len(b)
}

// enter sets the mode an action names, and says whether it was one: mode:off
// leaves it.
func (s *chordState) enter(action string) bool {
	name, ok := strings.CutPrefix(action, "mode:")
	if !ok {
		return false
	}
	s.pending = nil
	if name == "off" {
		name = ""
	}
	s.mode = name
	return true
}

// onKey sees a key of a terminal before it does: it reports whether a
// binding took it, running its action.
func (a *App) onKey(mods ui.Modifiers, key ui.Key) bool {
	if len(a.set.Bindings) == 0 && !a.chords.active() {
		return false
	}
	if a.chordBinds == nil || a.chordBindsFor == nil || !sameBindings(a.chordBindsFor, a.set.Bindings) {
		a.chordBinds, a.chordBindsFor = compileBindings(a.set.Bindings), a.set.Bindings
	}
	taken, action := a.chords.step(a.chordBinds, chord{mods, key}, time.Now())
	if action != "" && !a.chords.enter(action) {
		a.run(action, nil)
	}
	if a.chords.active() {
		a.expireChords()
	}
	if a.win != nil {
		a.win.Invalidate()
	}
	return taken
}

// expireChords drops a half-made sequence once it times out, so that the hint
// of it goes.
func (a *App) expireChords() {
	if len(a.chords.pending) == 0 {
		return
	}
	at := a.chords.at
	time.AfterFunc(chordTimeout+50*time.Millisecond, func() {
		a.update(func() {
			if a.chords.at.Equal(at) && len(a.chords.pending) > 0 && time.Since(at) > chordTimeout {
				a.chords.pending = nil
				if a.win != nil {
					a.win.Invalidate()
				}
			}
		})
	})
}

func sameBindings(a, b []settings.Binding) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Mode != b[i].Mode || a[i].Action != b[i].Action || strings.Join(a[i].Keys, ">") != strings.Join(b[i].Keys, ">") {
			return false
		}
	}
	return true
}

// chordHint floats over the bottom of the window while the user is in a mode
// or part of the way through a sequence, saying so.
func (a *App) chordHint(c *ui.Context, pal *palette) {
	if !a.chords.active() {
		return
	}
	text := locales.ChordMode(a.chords.mode)
	if len(a.chords.pending) > 0 {
		keys := make([]string, len(a.chords.pending))
		for i, p := range a.chords.pending {
			keys[i] = chordName(p)
		}
		text = locales.ChordPending(strings.Join(keys, " "))
	}
	ui.Row(c.Key("chord-hint")).Absolute().Left(0).Right(0).Bottom(18).Justify(ui.Center).PassThrough().Children(func() {
		pill := ui.Row(c).Height(30).Padding(0, 14).AlignItems(ui.Center).Radius(15).Background(pal.popover).Label(text)
		floating(pill, pal).Children(func() {
			ui.Text(c, text).FontSize(12.5).FontWeight(500).SingleLine().TextColor(pal.textMuted)
		})
	})
}

// chordName writes a chord as the file does: ctrl+a.
func chordName(c chord) string {
	var parts []string
	for _, m := range []struct {
		bit  ui.Modifiers
		name string
	}{{ui.Super, "cmd"}, {ui.Ctrl, "ctrl"}, {ui.Alt, "alt"}, {ui.Shift, "shift"}} {
		if c.mods&m.bit != 0 {
			parts = append(parts, m.name)
		}
	}
	return strings.Join(append(parts, strings.ToLower(keyNames[c.key])), "+")
}
