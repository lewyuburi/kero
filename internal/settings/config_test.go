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

// A gap of 0 is a choice, not a missing value: it stays 0, while one out of
// range goes back to the default.
func TestCardGap(t *testing.T) {
	for file, want := range map[string]int{"": DefaultCardGap, "card-gap = 0": 0, "card-gap = 3": 3, "card-gap = 99": DefaultCardGap, "card-gap = -2": DefaultCardGap} {
		s, _ := parse(file)
		s.normalize()
		if s.CardGap != want {
			t.Errorf("%q: card-gap %d, want %d", file, s.CardGap, want)
		}
	}
	// Only whole numbers in range are taken: anything else is a problem and
	// the default stays, NaN, infinity and fractions included.
	for _, bad := range []string{"99", "-2", "nan", "NaN", "inf", "3.5", "1e3", "wide"} {
		s, probs := parse("card-gap = " + bad)
		if len(probs) != 1 || s.CardGap != DefaultCardGap {
			t.Errorf("card-gap = %s: %d, problems %q", bad, s.CardGap, probs)
		}
	}
	s := Default()
	s.CardGap = 0
	if out := format(s, ""); !strings.Contains(out, "card-gap = 0") {
		t.Errorf("a gap of 0 is not written:\n%s", out)
	}
}

// pane-gap is "same", or its own points.
func TestPaneGap(t *testing.T) {
	for file, want := range map[string][2]any{
		"":                {true, DefaultPaneGap},
		"pane-gap = same": {true, DefaultPaneGap},
		"pane-gap = 0":    {false, 0},
		"pane-gap = 5":    {false, 5},
		"pane-gap = 16":   {false, 16},
	} {
		s, probs := parse(file)
		s.normalize()
		if s.PaneGapSame != want[0] || s.PaneGap != want[1] {
			t.Errorf("%q: same %v, gap %d, want %v, %v (%q)", file, s.PaneGapSame, s.PaneGap, want[0], want[1], probs)
		}
	}
	for _, bad := range []string{"wide", "nan", "inf", "2.5", "17", "-1"} {
		if s, probs := parse("pane-gap = " + bad); len(probs) != 1 || !s.PaneGapSame {
			t.Errorf("pane-gap = %s: same %v, problems %q", bad, s.PaneGapSame, probs)
		}
	}
	s := Default()
	s.PaneGapSame, s.PaneGap = false, 0
	if out := format(s, ""); !strings.Contains(out, "pane-gap = 0") {
		t.Errorf("an own gap of 0 is not written:\n%s", out)
	}
}

// tab-layout is vertical until the user chooses horizontal; anything else is
// a problem and keeps the default.
func TestTabLayout(t *testing.T) {
	if got := Default().TabLayout; got != TabsVertical {
		t.Errorf("default tab layout %q", got)
	}
	if s, probs := parse("tab-layout = horizontal"); s.TabLayout != TabsHorizontal || len(probs) != 0 {
		t.Errorf("horizontal: %q, problems %q", s.TabLayout, probs)
	}
	if s, probs := parse("tab-layout = diagonal"); s.TabLayout != TabsVertical || len(probs) != 1 {
		t.Errorf("diagonal: %q, problems %q", s.TabLayout, probs)
	}
	s := Default()
	s.TabLayout = TabsHorizontal
	out := format(s, "")
	if back, _ := parse(out); back.TabLayout != TabsHorizontal {
		t.Errorf("the choice does not come back from:\n%s", out)
	}
	if out := format(Default(), ""); strings.Contains(out, "\ntab-layout =") {
		t.Errorf("the default is written:\n%s", out)
	}
}
