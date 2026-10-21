package settings

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"kero/internal/actions"
)

func TestFormatKeepsTheUsersFile(t *testing.T) {
	file := `# my kero
font-size = 14

# colors
theme = light:Catppuccin Latte,dark:Catppuccin Mocha
some-future-key = yes
env = A=1
env = B=2
`
	s, probs := parse(file)
	if s.FontSize != 14 || s.LightTheme != "Catppuccin Latte" || s.DarkTheme != "Catppuccin Mocha" {
		t.Fatalf("parsed %+v", s)
	}
	if !reflect.DeepEqual(s.Env, []string{"A=1", "B=2"}) {
		t.Errorf("env %q", s.Env)
	}
	if len(probs) != 1 || !strings.Contains(probs[0], "some-future-key") {
		t.Errorf("problems %q", probs)
	}
	s.FontSize = 15
	s.Env = []string{"C=3"}
	s.CursorBlink = false
	out := format(s, file)
	want := `# my kero
font-size = 15

# colors
theme = light:Catppuccin Latte,dark:Catppuccin Mocha
some-future-key = yes
env = C=3
cursor-style-blink = false
`
	if out != want {
		t.Errorf("formatted:\n%s\nwant:\n%s", out, want)
	}
	back, _ := parse(out)
	if !reflect.DeepEqual(back, s) {
		t.Errorf("read back %+v, want %+v", back, s)
	}
}

func TestFormatNewFileHasOnlyChanges(t *testing.T) {
	s := Default()
	settingLines := func(out string) int {
		n := 0
		for _, l := range strings.Split(out, "\n") {
			if l != "" && !strings.HasPrefix(l, "#") {
				n++
			}
		}
		return n
	}
	if out := format(s, ""); settingLines(out) != 0 {
		t.Errorf("defaults wrote settings:\n%s", out)
	}
	s.FontFamily, s.Appearance = "Iosevka Term", "dark"
	out := format(s, "")
	if !strings.Contains(out, "window-theme = dark\n") || !strings.Contains(out, "font-family = Iosevka Term\n") {
		t.Errorf("formatted:\n%s", out)
	}
	if n := settingLines(out); n != 2 {
		t.Errorf("%d settings written, want 2:\n%s", n, out)
	}
}

func TestKeybinds(t *testing.T) {
	s, probs := parse("keybind = cmd+shift+x=pane.split.right\nkeybind = cmd+d=unbind\nkeybind = cmd+==text.bigger\nkeybind = ctrl+q=nope\n")
	if s.Shortcuts["pane.split.right"] != "CmdOrCtrl+Shift+X" {
		t.Errorf("split right %q", s.Shortcuts["pane.split.right"])
	}
	if v, ok := s.Shortcuts["pane.split.right"]; !ok || v == "" {
		t.Errorf("split right lost to the unbind: %q", v)
	}
	if s.Shortcuts["text.bigger"] != "CmdOrCtrl+=" {
		t.Errorf("bigger %q", s.Shortcuts["text.bigger"])
	}
	if len(probs) != 1 || !strings.Contains(probs[0], "nope") {
		t.Errorf("problems %q", probs)
	}
	// Unbinding keys frees the action that has them.
	split, _ := actions.Find("pane.split.right")
	keys := configKeys(split.Keys) // cmd+d on macOS
	s2, _ := parse("keybind = " + keys + "=unbind\n")
	if v, ok := s2.Shortcuts["pane.split.right"]; !ok || v != "" {
		t.Errorf("%s unbound, split right %q %v", keys, v, ok)
	}
	if out := format(s2, ""); !strings.Contains(out, "keybind = "+keys+"=unbind\n") {
		t.Errorf("formatted:\n%s", out)
	}
}

func TestBadValuesKeepDefaults(t *testing.T) {
	s, probs := parse("cursor-style = triangle\nfont-size = big\nconfirm-close = maybe\n")
	d := Default()
	if s.Cursor != d.Cursor || s.FontSize != d.FontSize || s.ConfirmClose != d.ConfirmClose {
		t.Errorf("bad values changed settings: %+v", s)
	}
	if len(probs) != 3 {
		t.Errorf("problems %q", probs)
	}
}

func TestMigratesSettingsJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("KERO_HOME", dir)
	t.Setenv("KERO_CONFIG", filepath.Join(dir, "config", "kero", "config"))
	os.WriteFile(filepath.Join(dir, "settings.json"), []byte(`{"fontSize": 16, "appearance": "dark", "background": "rex"}`), 0o600)
	mu.Lock()
	cache = nil
	mu.Unlock()
	s := Load()
	if s.FontSize != 16 || s.Appearance != "dark" {
		t.Fatalf("migrated %+v", s)
	}
	b, err := os.ReadFile(Path())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "font-size = 16") || strings.Contains(string(b), "background") {
		t.Errorf("config:\n%s", b)
	}
	if _, err := os.Stat(filepath.Join(dir, "settings.json")); err == nil {
		t.Error("settings.json stayed")
	}
}

// A keybind is one chord of the app, as it was; with > a sequence, with a
// name: a chord of a key mode. They come back as written.
func TestKeyBindings(t *testing.T) {
	file := `keybind = cmd+shift+d=pane.split.down
keybind = ctrl+a>c=session.new
keybind = ctrl+a>r=mode:resize
keybind = resize:left=pane.resize.left
keybind = resize:shift+right=pane.resize.right
keybind = resize:escape=mode:off
keybind = alt+x>ctrl+>=palette.open
`
	s, probs := parse(file)
	if len(probs) != 0 {
		t.Fatalf("problems %q", probs)
	}
	if s.Shortcuts["pane.split.down"] != "CmdOrCtrl+Shift+D" {
		t.Errorf("shortcuts %v", s.Shortcuts)
	}
	want := []Binding{
		{Keys: []string{"Ctrl+A", "C"}, Action: "session.new"},
		{Keys: []string{"Ctrl+A", "R"}, Action: "mode:resize"},
		{Mode: "resize", Keys: []string{"Left"}, Action: "pane.resize.left"},
		{Mode: "resize", Keys: []string{"Shift+Right"}, Action: "pane.resize.right"},
		{Mode: "resize", Keys: []string{"Escape"}, Action: ModeOff},
		{Keys: []string{"Alt+X", "Ctrl+>"}, Action: "palette.open"},
	}
	if !reflect.DeepEqual(s.Bindings, want) {
		t.Errorf("bindings\n got %+v\nwant %+v", s.Bindings, want)
	}
	out := format(s, "")
	for _, line := range []string{"keybind = ctrl+a>c=session.new", "keybind = resize:escape=mode:off", "keybind = alt+x>ctrl+>=palette.open"} {
		if !strings.Contains(out, line+"\n") {
			t.Errorf("no %q in:\n%s", line, out)
		}
	}
	if back, probs := parse(out); len(probs) != 0 || !reflect.DeepEqual(back.Bindings, want) || !reflect.DeepEqual(back.Shortcuts, s.Shortcuts) {
		t.Errorf("read back %+v, shortcuts %v, problems %q", back.Bindings, back.Shortcuts, probs)
	}
	clone := s.Clone()
	clone.Bindings[0].Keys[0] = "Ctrl+Z"
	if s.Bindings[0].Keys[0] != "Ctrl+A" {
		t.Error("a clone shares the bindings' keys")
	}
}

func TestKeyBindingProblems(t *testing.T) {
	for line, why := range map[string]string{
		"keybind = ctrl+a>c=nope":              "no action",
		"keybind = a>c=session.new":            "needs ctrl, alt or cmd",
		"keybind = shift+a>c=session.new":      "needs ctrl, alt or cmd",
		"keybind = ctrl+a>c=unbind":            "unbind",
		"keybind = ctrl+a>=session.new":        "",
		"keybind = ctrl+a>c=mode:Bad Name":     "mode's name",
		"keybind = off:left=pane.left":         "off is not a mode",
		"keybind = hyper+a>c=session.new":      "not a modifier",
		"keybind = ctrl+a>banana=session.new":  "no key",
		"keybind = ctrl+d>x=session.new":       "", // cmd+d is the shortcut of split on macOS only
		"keybind = cmd+d>x=session.new":        "begins a sequence and is also the shortcut",
		"keybind = resize:left":                "not <keys>=<action>",
		"keybind = resize:left=pane.resize.up": "-",
	} {
		s, probs := parse(line)
		switch why {
		case "-":
			if len(probs) != 0 || len(s.Bindings) != 1 {
				t.Errorf("%q: %+v, problems %q", line, s.Bindings, probs)
			}
		case "":
			// valid, or a clash that depends on the platform: only no crash.
		default:
			if len(probs) != 1 || !strings.Contains(probs[0], why) {
				t.Errorf("%q: problems %q, want one with %q", line, probs, why)
			}
		}
	}
}

func TestSplitChords(t *testing.T) {
	for keys, want := range map[string][]string{
		"ctrl+a":        {"ctrl+a"},
		"ctrl+a>c":      {"ctrl+a", "c"},
		"ctrl+a>ctrl+>": {"ctrl+a", "ctrl+>"},
		"ctrl+a>c>d":    {"ctrl+a", "c", "d"},
		"ctrl+>":        {"ctrl+>"},
		">":             {">"},
		"alt+x>>":       {"alt+x", ">"},
	} {
		if got := splitChords(keys); !reflect.DeepEqual(got, want) {
			t.Errorf("%q split as %q, want %q", keys, got, want)
		}
	}
}
