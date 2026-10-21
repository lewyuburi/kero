package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"kero/internal/proto"
	"kero/internal/settings"
)

// binds reads the values of keybind lines as the settings' bindings.
func binds(t *testing.T, lines ...string) []settings.Binding {
	t.Helper()
	var out []settings.Binding
	for _, line := range lines {
		b, err := settings.ParseBinding(line)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, b)
	}
	return out
}

func TestChordStateSequences(t *testing.T) {
	bs := compileBindings(binds(t, "ctrl+a>c=session.new", "ctrl+a>ctrl+a=palette.open"))
	ctrlA, c, x := chord{ui.Ctrl, ui.KeyA}, chord{0, ui.KeyC}, chord{0, ui.KeyX}
	var st chordState
	now := time.Now()

	// A key that no binding wants is the terminal's.
	if taken, act := st.step(bs, x, now); taken || act != "" {
		t.Errorf("x: taken %v, %q", taken, act)
	}
	// The first chord of a sequence is taken, the second runs it.
	if taken, act := st.step(bs, ctrlA, now); !taken || act != "" || !st.active() {
		t.Errorf("ctrl+a: taken %v, %q, active %v", taken, act, st.active())
	}
	if taken, act := st.step(bs, c, now.Add(100*time.Millisecond)); !taken || act != "session.new" || st.active() {
		t.Errorf("then c: taken %v, %q, active %v", taken, act, st.active())
	}
	// A sequence is three chords long when its binding is, and a wrong
	// chord ends it, taken, and does nothing.
	st.step(bs, ctrlA, now)
	if taken, act := st.step(bs, x, now); !taken || act != "" || st.active() {
		t.Errorf("ctrl+a, x: taken %v, %q, active %v", taken, act, st.active())
	}
	if taken, _ := st.step(bs, x, now); taken {
		t.Error("after it ended, x was taken")
	}
	// A sequence that waited too long is forgotten, and its second chord is
	// the terminal's.
	st.step(bs, ctrlA, now)
	if taken, act := st.step(bs, c, now.Add(chordTimeout+time.Second)); taken || act != "" {
		t.Errorf("c after a timeout: taken %v, %q", taken, act)
	}
}

func TestChordStateModes(t *testing.T) {
	bs := compileBindings(binds(t,
		"ctrl+a>r=mode:resize",
		"resize:left=pane.resize.left",
		"resize:shift+right=pane.resize.right",
		"resize:q=mode:off",
		"other:left=pane.left"))
	ctrlA, r, left := chord{ui.Ctrl, ui.KeyA}, chord{0, ui.KeyR}, chord{0, ui.KeyLeft}
	var st chordState
	now := time.Now()
	st.step(bs, ctrlA, now)
	taken, act := st.step(bs, r, now)
	if !taken || act != "mode:resize" || !st.enter(act) || st.mode != "resize" {
		t.Fatalf("entering: taken %v, %q, mode %q", taken, act, st.mode)
	}
	if taken, act := st.step(bs, left, now); !taken || act != "pane.resize.left" {
		t.Errorf("left in resize: %v, %q", taken, act)
	}
	if taken, act := st.step(bs, chord{ui.Shift, ui.KeyRight}, now); !taken || act != "pane.resize.right" {
		t.Errorf("shift+right in resize: %v, %q", taken, act)
	}
	// A key of no binding of the mode is taken, and does nothing: it is not
	// for the program.
	if taken, act := st.step(bs, chord{0, ui.KeyX}, now); !taken || act != "" || st.mode != "resize" {
		t.Errorf("x in resize: %v, %q, mode %q", taken, act, st.mode)
	}
	// Escape leaves a mode that binds it to nothing, and mode:off any mode.
	if taken, _ := st.step(bs, chord{0, ui.KeyEscape}, now); !taken || st.mode != "" {
		t.Errorf("escape: mode %q", st.mode)
	}
	st.enter("mode:resize")
	taken, act = st.step(bs, chord{0, ui.KeyQ}, now)
	if !taken || !st.enter(act) || st.mode != "" {
		t.Errorf("q: %v, %q, mode %q", taken, act, st.mode)
	}
	// Outside the mode, its keys are the terminal's.
	if taken, _ := st.step(bs, left, now); taken {
		t.Error("left outside the mode was taken")
	}
}

func TestParseAccel(t *testing.T) {
	for accel, want := range map[string]chord{
		"Ctrl+A":            {ui.Ctrl, ui.KeyA},
		"Shift+Right":       {ui.Shift, ui.KeyRight},
		"Escape":            {0, ui.KeyEscape},
		"CmdOrCtrl+Shift+D": {ui.Cmd | ui.Shift, ui.KeyD},
		"Alt+[":             {ui.Alt, ui.KeyBracketLeft},
	} {
		if got, ok := parseAccel(accel); !ok || got != want {
			t.Errorf("%s: %+v, %v, want %+v", accel, got, ok, want)
		}
	}
	if _, ok := parseAccel("Ctrl+Banana"); ok {
		t.Error("a key nobody knows")
	}
	if got := chordName(chord{ui.Ctrl | ui.Shift, ui.KeyA}); got != "ctrl+shift+a" {
		t.Errorf("name %q", got)
	}
}

// The app takes the keys of its bindings from the terminal, runs their
// actions and says where the user is.
func TestAppChords(t *testing.T) {
	a := testApp(t)
	a.set.Bindings = binds(t,
		"ctrl+a>r=mode:resize",
		"resize:right=pane.resize.right",
		"resize:escape=mode:off",
		"ctrl+a>b=sidebar.toggle")
	var l *proto.Layout
	a.st.Sessions[0].Layout = l.Split("a", "b", "right", 0.5)
	tt := ui.NewTester(a.view, 1000, 600)
	tt.Frame()

	if a.onKey(0, ui.KeyX) {
		t.Fatal("a key of no binding was taken")
	}
	hidden := a.prefs.SidebarHidden
	if !a.onKey(ui.Ctrl, ui.KeyA) || !a.onKey(0, ui.KeyB) || a.prefs.SidebarHidden == hidden {
		t.Errorf("ctrl+a b: the sidebar is hidden: %v, was %v", a.prefs.SidebarHidden, hidden)
	}
	a.onKey(ui.Ctrl, ui.KeyA)
	tt.Frame()
	if !tt.HasText("Waiting for the next key of ctrl+a") {
		t.Errorf("no hint of the sequence; texts %q", tt.Texts())
	}
	a.onKey(0, ui.KeyR)
	tt.Frame()
	if !tt.HasText("resize mode · Esc leaves it") {
		t.Errorf("no hint of the mode; texts %q", tt.Texts())
	}
	shot(t, tt, "chord-hint")
	if !a.onKey(0, ui.KeyRight) {
		t.Error("right in the mode was not taken")
	}
	if got := a.st.Sessions[0].Layout.Ratio; got != 0.55 {
		t.Errorf("the divider is at %v, want 0.55", got)
	}
	if !a.onKey(0, ui.KeyEscape) || a.chords.active() {
		t.Error("escape did not leave the mode")
	}
	tt.Frame()
	if tt.HasText("resize mode · Esc leaves it") {
		t.Error("the hint stayed after the mode")
	}
	if a.onKey(0, ui.KeyRight) {
		t.Error("right outside the mode was taken")
	}
}
