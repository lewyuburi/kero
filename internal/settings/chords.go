package settings

import (
	"fmt"
	"regexp"
	"strings"

	"kero/internal/actions"
)

// Binding is a keybind that is more than one chord of the app: a sequence of
// chords (ctrl+a>c), or chords of a key mode (resize:left), named by what
// enters it. Action is an action's id, mode:<name> to enter a mode, or
// ModeOff to leave it.
type Binding struct {
	Mode   string
	Keys   []string // accelerators, as "Ctrl+A"
	Action string
}

// ModeOff leaves the key mode.
const ModeOff = "mode:off"

var modeName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// splitChords splits keys at the > that separates chords, not at one that is
// the key itself (ctrl+>).
func splitChords(keys string) []string {
	var out []string
	start := 0
	for i := 0; i < len(keys); i++ {
		if keys[i] == '>' && i > start && keys[i-1] != '+' && i < len(keys)-1 {
			out = append(out, keys[start:i])
			start = i + 1
		}
	}
	return append(out, keys[start:])
}

// splitMode splits a key mode's name from keys: resize:left.
func splitMode(keys string) (mode, rest string) {
	if i := strings.Index(keys, ":"); i > 0 && modeName.MatchString(keys[:i]) && i < len(keys)-1 {
		return keys[:i], keys[i+1:]
	}
	return "", keys
}

// complexBinding tells that a keybind's keys and action are more than a
// chord for an action, which a Binding holds.
func complexBinding(keys, action string) bool {
	mode, rest := splitMode(strings.TrimSpace(keys))
	return mode != "" || len(splitChords(rest)) > 1 || strings.HasPrefix(action, "mode:")
}

var (
	validMods = map[string]bool{"CmdOrCtrl": true, "Ctrl": true, "Alt": true, "Shift": true}
	fKey      = regexp.MustCompile(`^F([1-9]|1[0-2])$`)
	namedKeys = map[string]bool{
		"Enter": true, "Tab": true, "Space": true, "Backspace": true, "Delete": true, "Escape": true, "Up": true, "Down": true,
		"Left": true, "Right": true, "Home": true, "End": true, "PageUp": true, "PageDown": true, "Plus": true,
	}
)

// chordOf reads a chord as the file writes it, as an accelerator.
func chordOf(chord string) (string, error) {
	accel := accelerator(chord)
	parts := strings.Split(accel, "+")
	for _, m := range parts[:len(parts)-1] {
		if !validMods[m] {
			return "", fmt.Errorf("%q is not a modifier in %q", m, chord)
		}
	}
	key := parts[len(parts)-1]
	switch {
	case namedKeys[key], fKey.MatchString(key):
	case len([]rune(key)) == 1:
	default:
		return "", fmt.Errorf("no key %q in %q", key, chord)
	}
	return accel, nil
}

// ParseBinding reads the value of a keybind that is a sequence, or of a key
// mode: `ctrl+a>c=session.new`, `ctrl+a>r=mode:resize`, `resize:left=pane.resize.left`.
func ParseBinding(line string) (Binding, error) {
	i := strings.LastIndex(line, "=")
	if i <= 0 {
		return Binding{}, fmt.Errorf("%q is not <keys>=<action>", line)
	}
	b := Binding{Action: strings.TrimSpace(line[i+1:])}
	var rest string
	b.Mode, rest = splitMode(strings.TrimSpace(line[:i]))
	if b.Mode == "off" {
		return Binding{}, fmt.Errorf("%q: off is not a mode's name", line)
	}
	for _, chord := range splitChords(rest) {
		accel, err := chordOf(chord)
		if err != nil {
			return Binding{}, err
		}
		b.Keys = append(b.Keys, accel)
	}
	if b.Mode == "" {
		// Outside a mode every key is the terminal's, but those the user
		// holds a modifier for: a sequence has to begin with one.
		first := strings.Split(b.Keys[0], "+")
		if len(first) == 1 || first[0] == "Shift" && len(first) == 2 {
			return Binding{}, fmt.Errorf("%q: the first key of a sequence needs ctrl, alt or cmd", line)
		}
	}
	switch name, isMode := strings.CutPrefix(b.Action, "mode:"); {
	case b.Action == "unbind":
		return Binding{}, fmt.Errorf("%q: unbind frees the keys of an action, not a sequence or a mode's", line)
	case isMode && name != "off" && !modeName.MatchString(name):
		return Binding{}, fmt.Errorf("%q: %q is not a mode's name", line, name)
	case !isMode:
		if _, ok := actions.Find(b.Action); !ok {
			return Binding{}, fmt.Errorf("no action %q", b.Action)
		}
	}
	return b, nil
}

// String writes a binding as the keybind's value.
func (b Binding) String() string {
	keys := make([]string, len(b.Keys))
	for i, k := range b.Keys {
		keys[i] = configKeys(k)
	}
	out := strings.Join(keys, ">")
	if b.Mode != "" {
		out = b.Mode + ":" + out
	}
	return out + "=" + b.Action
}
