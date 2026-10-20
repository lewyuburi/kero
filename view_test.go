package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"kero/internal/actions"
	"kero/internal/proto"
	"kero/internal/settings"

	"kero/locales"
)

// testApp is the app with a session of two panes side by side, a | b,
// and no kerod.
func testApp(t *testing.T) *App {
	t.Helper()
	home := t.TempDir()
	t.Setenv("KERO_HOME", home)
	// The configuration too: never the user's.
	t.Setenv("KERO_CONFIG", filepath.Join(home, "config"))
	a := newApp()
	var l *proto.Layout
	a.st = proto.State{
		Projects: []proto.Project{{ID: "p1", Name: "demo", Dir: "/tmp"}},
		Sessions: []proto.Session{{
			ID: "s1", Project: "p1", Name: "demo",
			Panes: []proto.Pane{
				{ID: "a", Shell: "fish", Dir: "/tmp", Status: proto.Idle, Progress: -1},
				{ID: "b", Shell: "zsh", Dir: "/tmp", Status: proto.Idle, Progress: -1},
			},
			Layout: l.Split("a", "b", "right", 0),
			Focus:  "a",
		}},
	}
	a.loaded, a.selected = true, "s1"
	return a
}

// shot writes the tester's frame to $KERO_SHOTS, to look at.
func shot(t *testing.T, tt *ui.Tester, name string) {
	dir := os.Getenv("KERO_SHOTS")
	if dir == "" {
		return
	}
	f, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	png.Encode(f, tt.Image())
}

func TestDragPaneToSide(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.view, 1100, 640)
	tt.Frame()
	grip, ok := tt.Find("zsh")
	if !ok {
		t.Fatalf("no title of pane b; texts %q", tt.Texts())
	}
	// Pick pane b up by its title, and carry it over a's left edge.
	x, y := grip.X+grip.W/2, grip.Y+grip.H/2
	ar, _ := tt.Find("fish") // pane a's title
	tx, ty := ar.X+10, float32(340)
	tt.Press(x, y)
	for i := 1; i <= 12; i++ {
		f := float32(i) / 12
		tt.Move(x+(tx-x)*f, y+(ty-y)*f)
		tt.Advance(16 * time.Millisecond)
		tt.Frame()
	}
	tt.Advance(400 * time.Millisecond)
	tt.Frame()
	if !tt.HasText("Split Left") {
		t.Errorf("no preview of a split on the left; texts %q", tt.Texts())
	}
	shot(t, tt, "drag-left")

	// Over a's middle: the panes would swap.
	tt.Move(ar.X+120, 340)
	tt.Advance(400 * time.Millisecond)
	tt.Frame()
	if !tt.HasText("Swap Panes") {
		t.Errorf("no preview of a swap; texts %q", tt.Texts())
	}
	shot(t, tt, "drag-swap")

	tt.Release(ar.X+120, 340)
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	if got := a.st.Sessions[0].Layout.Panes(); !slices.Equal(got, []string{"b", "a"}) {
		t.Fatalf("after the drop, panes %v, want b then a", got)
	}
	if a.focusOf(&a.st.Sessions[0]) != "b" {
		t.Errorf("the pane moved has not the focus")
	}
	shot(t, tt, "dropped")
}

// The sidebar's animation moves the header's title one way only, in
// whole pixels: a title that steps back and forth shakes.
func TestSidebarAnimationMonotonic(t *testing.T) {
	bar := ui.TitleBar{Left: 78, Height: 60}
	for _, width := range []float32{200, 268, 333, 440} {
		prevX := float32(-1)
		prevEdge := float32(-1)
		for i := 0; i <= 20000; i++ {
			ch := chromeFor(float32(i)/20000, width, 8, bar)
			x := ch.sideW + ch.mainPad + ch.headerPad // the title's left, in the window
			edge := ch.sideW + ch.mainPad             // the card's left edge
			for _, v := range []float32{ch.sideW, ch.mainPad, ch.headerPad} {
				if v != float32(int(v)) {
					t.Fatalf("width %v, progress %v: %v is not a whole pixel", width, ch.shown, v)
				}
			}
			if x < prevX {
				t.Fatalf("width %v, step %d: the title moved back, %v to %v", width, i, prevX, x)
			}
			if edge < prevEdge {
				t.Fatalf("width %v, step %d: the card's edge moved back, %v to %v", width, i, prevEdge, edge)
			}
			prevX, prevEdge = x, edge
		}
		if ch := chromeFor(1, width, 8, bar); ch.headerPad != 16 || ch.mainPad != 0 || ch.sideW != width {
			t.Errorf("width %v shown: %+v", width, ch)
		}
		if ch := chromeFor(0, width, 8, bar); ch.sideW != 0 || ch.mainPad != 8 {
			t.Errorf("width %v hidden: %+v", width, ch)
		}
	}
}

// The sidebar's animation never holds still before it lands: a frame
// that does not move, then one that does, reads as a stutter.
func TestSidebarAnimationNoHolds(t *testing.T) {
	ease := sidebarMotion.Ease
	frames := int(sidebarMotion.Duration / (time.Second / 60))
	prev := float32(-1)
	for i := 0; i <= frames; i++ {
		w := chromeFor(ease(float32(i)/float32(frames)), 268, 8, ui.TitleBar{}).sideW
		if w == prev && w != 268 {
			t.Errorf("frame %d: the sidebar held at %v before landing", i, w)
		}
		prev = w
	}
}

// The window moves from the panes' headers and the gaps around them, and
// not from a press on a pane, which focuses it.
func TestWindowDragRegions(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.view, 1100, 640)
	tt.Frame()
	// Pane b's header, past its title: it moves the window, and leaves
	// the focus where it was.
	b, _ := tt.Find("zsh")
	tt.ClickAt(b.X+200, b.Y+b.H/2)
	tt.Frame()
	if f := a.focusOf(&a.st.Sessions[0]); f != "a" {
		t.Errorf("a press on a header's empty stretch moved the focus to %q", f)
	}
	// Pane b's body: it takes the press, and the focus.
	tt.ClickAt(b.X+200, 400)
	tt.Frame()
	if f := a.focusOf(&a.st.Sessions[0]); f != "b" {
		t.Errorf("a press on pane b left the focus on %q", f)
	}
}

// A selection copied as it ends says so; a copy the user asked for, or
// a program's, does not.
func TestCopyOnSelectToast(t *testing.T) {
	a := testApp(t)
	o := a.termOptions("a", nil)
	o.OnCopy("x", terminal.CopyUser)
	o.OnCopy("x", terminal.CopyProgram)
	if a.pendingToast != "" {
		t.Errorf("toast %q for a copy that shows itself", a.pendingToast)
	}
	o.OnCopy("x", terminal.CopySelect)
	if a.pendingToast != "Copied to clipboard" {
		t.Errorf("toast %q after copying a selection", a.pendingToast)
	}
}

// The sidebar's foot counts what wants a look, each count going to the
// next session of it, and says nothing while nothing does ($KERO_SHOTS).
func TestSidebarFooter(t *testing.T) {
	a := testApp(t)
	a.conn = connReady
	tt := ui.NewTester(a.view, 1100, 700)
	tt.Frame()
	for _, txt := range tt.Texts() {
		if strings.Contains(txt, "running") {
			t.Errorf("the foot still says %q", txt)
		}
	}
	base := a.st.Sessions[0]
	add := func(id, name string, st proto.Status, unread bool) {
		s := base
		s.ID, s.Name, s.Status, s.Unread = id, name, st, unread
		a.st.Sessions = append(a.st.Sessions, s)
	}
	add("s2", "deploy", proto.Waiting, true)
	add("s3", "refactor", proto.Working, false)
	add("s4", "tests", proto.Working, false)
	add("s5", "docs", proto.Idle, true)
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	shot(t, tt, "sidebar-footer")
	for label, want := range map[string]string{"1 session needs you": "s2", "2 sessions working": "s3", "1 session done, unseen": "s5"} {
		a.selected = "s1"
		tt.Frame()
		if err := tt.Click(label); err != nil {
			t.Errorf("no %q in the foot: %v", label, err)
			continue
		}
		tt.Frame()
		if a.selected != want {
			t.Errorf("%q went to %s, want %s", label, a.selected, want)
		}
	}
}

// A session's tooltip shows below the pointer where it rested, as macOS
// shows tooltips, not centered over the row ($KERO_SHOTS).
func TestSessionTooltipAtPointer(t *testing.T) {
	for _, dx := range []float32{20, 160} {
		a := testApp(t)
		a.st.Sessions[0].Process = "claude"
		a.st.Sessions[0].Dir = "/Users/someone/dev/kero"
		tt := ui.NewTester(a.view, 1100, 700)
		tt.Frame()
		r, ok := tt.Find("1") // the project header's count, above the row
		if !ok {
			t.Fatal("no project header")
		}
		x, y := 12+dx, r.Y+r.H/2+34 // the session row's middle
		tt.Move(x, y)
		tt.Advance(2 * time.Second)
		tt.Frame()
		var tip ui.Rect
		found := false
		for _, txt := range tt.Texts() {
			if strings.HasPrefix(txt, "claude · ") {
				tip, found = tt.Find(txt)
			}
		}
		if !found {
			t.Fatalf("dx %g: no tooltip; texts %q", dx, tt.Texts())
		}
		if d := tip.X - x; d < 0 || d > 16 {
			t.Errorf("dx %g: tooltip text at x %g, the pointer at %g", dx, tip.X, x)
		}
		if d := tip.Y - y; d < 12 || d > 36 {
			t.Errorf("dx %g: tooltip text at y %g, the pointer at %g", dx, tip.Y, y)
		}
		shot(t, tt, fmt.Sprintf("session-tooltip-%g", dx))
	}
}

// A kerod of another version of Kero gets a chip to restart it, which asks
// first while a program runs, as the restart ends it.
func TestRestartStaleDaemon(t *testing.T) {
	a := testApp(t)
	a.conn, a.daemonApp = connReady, "1.9.0"
	a.st.Sessions[0].Process = "claude"
	tt := ui.NewTester(a.view, 1100, 640)
	tt.Frame()
	if err := tt.Click("Restart Service"); err != nil {
		t.Fatalf("no chip to restart kerod: %v; texts %q", err, tt.Texts())
	}
	tt.Frame()
	if a.confirm == nil || !strings.Contains(a.confirm.detail, "Kero 1.9.0") {
		t.Fatalf("no question before the restart: %+v", a.confirm)
	}

	a.confirm, a.daemonApp = nil, mygo.App.Version()
	tt.Frame()
	if tt.HasText("Restart Service") {
		t.Error("a chip to restart a kerod of this version")
	}
}

// idleConn is a Conn that shows only what a test feeds the terminal.
type idleConn struct{ closed chan struct{} }

func (c idleConn) Read([]byte) (int, error) {
	<-c.closed
	return 0, os.ErrClosed
}
func (c idleConn) Write(b []byte) (int, error) { return len(b), nil }
func (c idleConn) Close() error {
	select {
	case <-c.closed:
	default:
		close(c.closed)
	}
	return nil
}

// Find searches the pane the user is in, its scrollback too: the bar
// counts the matches as one types, Enter and the buttons go through them,
// and Escape closes it, the highlights going; Find Next opens it again
// with the last query.
func TestFindBar(t *testing.T) {
	if testing.Short() {
		t.Skip("downloads libghostty-vt")
	}
	if err := terminal.Load(); err != nil {
		t.Fatal(err)
	}
	a := testApp(t)
	term, err := terminal.New(terminal.Options{Conn: idleConn{make(chan struct{})}})
	if err != nil {
		t.Fatal(err)
	}
	defer term.Close()
	var b strings.Builder
	for i := range 200 {
		if i%50 == 7 {
			b.WriteString("error: build failed\r\n")
		} else {
			b.WriteString("ok " + itoa(i) + "\r\n")
		}
	}
	term.Feed([]byte(b.String()))
	a.terms["a"] = &termEntry{term: term}
	tt := ui.NewTester(a.view, 1100, 640)
	tt.Frame()
	a.run("find.open", nil)
	tt.Frame()
	tt.Type("ERROR")
	for range 50 {
		tt.Frame()
		if s := term.SearchStatus(); !s.Searching && s.Selected > 0 {
			break
		}
	}
	tt.Frame()
	if !tt.HasText("1 of 4") {
		t.Fatalf("no count; texts %q", tt.Texts())
	}
	shot(t, tt, "find")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	tt.Key(ui.Shift, ui.KeyEnter)
	tt.Frame()
	if !tt.HasText("2 of 4") {
		t.Errorf("after Enter twice and Shift+Enter; texts %q", tt.Texts())
	}
	if err := tt.Click(a.tip(locales.ActionFindNext(), "find.next")); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("3 of 4") {
		t.Errorf("after the button for the next; texts %q", tt.Texts())
	}
	shot(t, tt, "find-scrolled")
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	tt.Frame()
	if a.find.pane != "" || tt.HasText("3 of 4") || term.SearchStatus().Text != "" {
		t.Errorf("Escape left the bar open or the search on: %+v; texts %q", term.SearchStatus(), tt.Texts())
	}
	a.run("find.next", nil)
	tt.Frame()
	tt.Frame()
	if a.find.pane != "a" || a.find.query != "ERROR" || term.SearchStatus().Text != "ERROR" {
		t.Errorf("Find Next did not search again: %+v, %+v", a.find.query, term.SearchStatus())
	}
}

// Command over the path of an image a pane printed, relative to its
// directory, shows the image beside it, until Command is let go of,
// which lets go of the image too.
func TestImagePeek(t *testing.T) {
	if testing.Short() {
		t.Skip("downloads libghostty-vt")
	}
	if err := terminal.Load(); err != nil {
		t.Fatal(err)
	}
	a := testApp(t)
	dir := t.TempDir()
	a.st.Sessions[0].Panes[0].Dir = dir
	f, err := os.Create(filepath.Join(dir, "shot.png"))
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 30, 20))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.RGBA{230, 120, 40, 255}), image.Point{}, draw.Src)
	png.Encode(f, img)
	f.Close()
	term, err := terminal.New(terminal.Options{Conn: idleConn{make(chan struct{})}, OpenLink: func(link string) bool { return a.openLink("a", link) }})
	if err != nil {
		t.Fatal(err)
	}
	defer term.Close()
	term.Feed([]byte("saved shot.png:3 and notes.txt\r\n"))
	a.terms["a"] = &termEntry{term: term}
	tt := ui.NewTester(a.view, 1100, 640)
	tt.Frame()
	b, ok := tt.Find("Terminal")
	if !ok {
		t.Fatalf("no terminal; texts %q", tt.Texts())
	}
	// Along the first row to the path, Command pressed with the pointer
	// still.
find:
	for y := b.Y; y < b.Y+40; y += 4 {
		for x := b.X; x < b.X+b.W/2; x += 3 {
			tt.Move(x, y)
			tt.HoldModifiers(ui.Cmd)
			if link, _, _ := term.HoveredLink(); link != "" {
				break find
			}
		}
	}
	tt.Frame()
	if link, path, _ := term.HoveredLink(); link != "shot.png:3" || !path {
		t.Fatalf("hovered %q, %v; text %q", link, path, term.Text())
	}
	if want := filepath.Join(dir, "shot.png"); a.peek.path != want {
		t.Fatalf("peeks at %q, want %q", a.peek.path, want)
	}
	// The decoded image lands, as the window's update would bring it, and
	// shows in the flyout.
	data, _ := os.ReadFile(a.peek.path)
	a.peek.bm, _ = ui.DecodeBitmap(data)
	tt.Frame()
	ft := ui.NewTester(a.peekView, peekMinW, 20+peekPad+peekCaption)
	ft.Frame()
	if !ft.HasText("shot.png") || !ft.HasText("30 × 20") {
		t.Fatalf("no preview; texts %q", ft.Texts())
	}
	shot(t, ft, "peek")
	tt.HoldModifiers(0)
	tt.Frame()
	if a.peek.bm != nil || a.peek.path != "" {
		t.Errorf("the preview stays once Command is let go of: %+v", a.peek)
	}
}

// The cards' corners: the default gap keeps the 14 they always had, none
// takes the window's own, and a wide gap does not square them.
func TestCardRadius(t *testing.T) {
	for gap, want := range map[float32]float32{0: windowRadius, 8: defaultRadius, 4: (windowRadius + defaultRadius) / 2} {
		if got := cardRadius(gap); got != want {
			t.Errorf("cardRadius(%v) = %v, want %v", gap, got, want)
		}
	}
	if got := cardRadius(100); got != minRadius {
		t.Errorf("cardRadius(100) = %v, want %v", got, float32(minRadius))
	}
}

// A header keeps its height whatever the gap, and its middle is the bar's
// line, which the sidebar's bar centers on too: at the default gap, the 30
// the bar always had. With no gap the cards touch the window's edge,
// sidebar shown or not.
func TestCardGapLayout(t *testing.T) {
	for _, gap := range []float32{0, 4, 8, settings.MaxCardGap} {
		if mid := gap + headerH/2; mid != barMid(gap) || topBar(gap) != 2*mid {
			t.Errorf("gap %v: header centered on %v, bar on %v, top bar %v", gap, mid, barMid(gap), topBar(gap))
		}
	}
	if barMid(settings.DefaultCardGap) != 30 {
		t.Errorf("the bar's line at the default gap is %v, want 30", barMid(settings.DefaultCardGap))
	}
	for _, p := range []float32{0, 0.5, 1} {
		if ch := chromeFor(p, 268, 0, ui.TitleBar{}); ch.mainPad != 0 {
			t.Errorf("no gap, sidebar at %v: the cards' left padding is %v, want 0", p, ch.mainPad)
		}
	}
}

// Panes follow the panel gap unless told otherwise, and the corners where
// they meet are as round as their gap: square with none.
func TestPaneGap(t *testing.T) {
	s := settings.Default()
	s.CardGap = 0
	if got := paneGapOf(s); got != 0 {
		t.Errorf("panes follow a panel gap of 0 and have %v", got)
	}
	s.PaneGapSame, s.PaneGap = false, 6
	if got := paneGapOf(s); got != 6 {
		t.Errorf("an own pane gap of 6 is %v", got)
	}
	for gap, want := range map[float32]float32{0: 0, 4: 7, settings.DefaultPaneGap: defaultRadius} {
		if got := paneRadius(gap, defaultRadius); got != want {
			t.Errorf("paneRadius(%v) = %v, want %v", gap, got, want)
		}
	}
	if got := paneRadius(8, 5); got != 5 {
		t.Errorf("a pane's corner %v is rounder than the card's 5", got)
	}
}

// A pane split down is built, as one split right is: its box takes its key
// once, as a Column, and the panes show one above the other.
func TestSplitDown(t *testing.T) {
	a := testApp(t)
	var l *proto.Layout
	a.st.Sessions[0].Layout = l.Split("a", "b", "down", 0)
	var first, second ui.Rect
	tt := ui.NewTester(func(c *ui.Context) {
		a.view(c)
		first, second = a.paneRef("card-a").Bounds(c), a.paneRef("card-b").Bounds(c)
	}, 900, 600)
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	if first.W == 0 || second.W == 0 || first.Y+first.H > second.Y || first.X != second.X {
		t.Errorf("panes split down: %+v over %+v", first, second)
	}
}

// With no gap between panes, the corners that touch are square and the
// window's stay the card's.
func TestPaneCorners(t *testing.T) {
	a := testApp(t)
	a.set.PaneGapSame, a.set.PaneGap = false, 0
	a.chrome.gap = 8
	// the left of two panes side by side: a window corner at the top left
	// and bottom left, a neighbor on the right
	got := a.paneCorners(true, true, false, true)
	want := [4]float32{14, 0, 0, 14}
	if got != want {
		t.Errorf("left pane's corners %v, want %v", got, want)
	}
	got = a.paneCorners(true, true, true, true)
	if want := [4]float32{14, 14, 14, 14}; got != want {
		t.Errorf("a lone pane's corners %v, want %v", got, want)
	}
}

// The tabs across the top put the traffic lights on the tab bar's line, and
// the cards' headers no longer carry the sidebar's buttons or the window's
// controls, which the bar holds.
func TestHorizontalTabsLayout(t *testing.T) {
	s := settings.Default()
	if tabsHorizontal(s) {
		t.Fatal("tabs are horizontal by default")
	}
	if barLine(s) != 30 {
		t.Errorf("the bar's line with vertical tabs is %v, want 30", barLine(s))
	}
	if tabsHorizontal(s) {
		t.Error("the flag alone put the tabs across the top")
	}
	s.TabLayout = settings.TabsHorizontal
	if barLine(s) != tabBarH/2 || lightsY(barLine(s)) != 13 {
		t.Errorf("with horizontal tabs the line is %v and the lights at %d, want %v and 13", barLine(s), lightsY(barLine(s)), float32(tabBarH/2))
	}
}

// With tabs across the top there is one tab a session, a click on one shows
// its session, and the sidebar is not there to hide.
func TestHorizontalTabs(t *testing.T) {
	a := testApp(t)
	a.set.TabLayout = settings.TabsHorizontal
	a.st.Projects = append(a.st.Projects, proto.Project{ID: "p2", Name: "other", Dir: "/tmp"})
	a.st.Sessions = append(a.st.Sessions, proto.Session{
		ID: "s2", Project: "p2", Name: "second",
		Panes: []proto.Pane{{ID: "c", Shell: "zsh", Dir: "/tmp", Status: proto.Idle, Progress: -1}},
		Focus: "c",
	})
	tt := ui.NewTester(a.view, 1100, 640)
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	for _, want := range []string{"demo", "other", "second"} {
		if !tt.HasText(want) {
			t.Errorf("no %q on the tab bar; texts %q", want, tt.Texts())
		}
	}
	if a.chrome.sideW != 0 || a.chrome.headerPad != 16 || a.chrome.shown != 1 {
		t.Errorf("horizontal tabs leave chrome %+v", a.chrome)
	}
	if err := tt.Click("second"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.selected != "s2" {
		t.Errorf("selected %q after a click on the tab", a.selected)
	}
	hidden := a.prefs.SidebarHidden
	a.toggleSidebar()
	if a.prefs.SidebarHidden != hidden {
		t.Error("the sidebar toggle hid something with tabs across the top")
	}
	shot(t, tt, "horizontal-tabs")
}

// The tab bar in both themes, with a project folded and a color picked
// ($KERO_SHOTS).
func TestHorizontalTabsShots(t *testing.T) {
	for _, dark := range []bool{false, true} {
		a := testApp(t)
		a.set.TabLayout = settings.TabsHorizontal
		a.st.Projects = append(a.st.Projects, proto.Project{ID: "p2", Name: "website", Dir: "/tmp"}, proto.Project{ID: "p3", Name: "api", Dir: "/tmp"})
		for _, id := range []string{"s2", "s3", "s4"} {
			proj := map[string]string{"s2": "p2", "s3": "p2", "s4": "p3"}[id]
			a.st.Sessions = append(a.st.Sessions, proto.Session{ID: id, Project: proj, Name: "session " + id,
				Panes: []proto.Pane{{ID: "x" + id, Shell: "zsh", Dir: "/tmp", Status: proto.Idle, Progress: -1}}, Focus: "x" + id})
		}
		a.st.Sessions[0].Status = proto.Idle
		a.st.Sessions[1].Status, a.st.Sessions[2].Status = proto.Waiting, proto.Working
		a.setProjectColor("p3", "yellow")
		a.prefs.Collapsed["p3"] = true
		a.selected = "s2"
		tt := ui.NewTester(a.view, 1000, 220)
		tt.SetDark(dark)
		tt.Frame()
		tt.Advance(time.Second)
		tt.Frame()
		name := "tabs-light"
		if dark {
			name = "tabs-dark"
		}
		shot(t, tt, name)
	}
}

// A project's color in the tabs is its own until the user picks one, which
// the app keeps; the sessions of no project are grey.
func TestProjectColor(t *testing.T) {
	a := testApp(t)
	first := a.projectColor("p1")
	if first.ID == "grey" || first.ID != a.projectColor("p1").ID {
		t.Errorf("p1's own color is %q, which is grey or changes", first.ID)
	}
	if a.projectColor("").ID != "grey" {
		t.Errorf("no project is %q, want grey", a.projectColor("").ID)
	}
	pick := "yellow"
	if first.ID == pick {
		pick = "green"
	}
	a.setProjectColor("p1", pick)
	if got := a.projectColor("p1").ID; got != pick {
		t.Errorf("after picking %s p1 is %q", pick, got)
	}
	if got := a.projectColor("p2").ID; got == pick && a.projectColor("p2").ID != a.projectColor("p2").ID {
		t.Errorf("a pick for p1 changed p2 to %q", got)
	}
	for _, col := range projectColors {
		if colorName(col.ID) == "" {
			t.Errorf("color %q has no name", col.ID)
		}
	}
}

// A tab is as wide as the window leaves it, between its least and its most,
// and a session at rest shows no status on it.
func TestTabWidth(t *testing.T) {
	a := testApp(t)
	a.set.TabLayout = settings.TabsHorizontal
	for _, win := range []float32{400, 1100, 3000} {
		tt := ui.NewTester(func(c *ui.Context) {
			if w := a.tabWidth(c, ui.TitleBar{}); w < tabMinW || w > tabMaxW {
				t.Errorf("a tab %v wide in a window %v wide, outside %d to %d", w, win, tabMinW, tabMaxW)
			}
			a.view(c)
		}, int(win), 400)
		tt.Frame()
	}
}

// ⇧⌘B puts the tabs across the top and back, saving the choice.
func TestToggleTabs(t *testing.T) {
	act, ok := actions.Find("tabs.toggle")
	if !ok || act.Title == "" || act.Menu != "View" {
		t.Fatalf("no tabs.toggle action in the View menu: %+v", act)
	}
	if runtime.GOOS == "darwin" && act.Keys != "CmdOrCtrl+Shift+B" {
		t.Errorf("tabs.toggle is bound to %q, want CmdOrCtrl+Shift+B", act.Keys)
	}
	if side, _ := actions.Find("sidebar.toggle"); side.Keys == act.Keys {
		t.Errorf("tabs.toggle and sidebar.toggle share %q", act.Keys)
	}
	a := testApp(t)
	if tabsHorizontal(a.set) {
		t.Fatal("tabs are horizontal by default")
	}
	for i, want := range []string{settings.TabsHorizontal, settings.TabsVertical, settings.TabsHorizontal} {
		a.run("tabs.toggle", nil)
		if a.set.TabLayout != want || tabsHorizontal(a.set) != (want == settings.TabsHorizontal) {
			t.Errorf("toggle %d: layout %q, horizontal %v", i+1, a.set.TabLayout, tabsHorizontal(a.set))
		}
		if saved := settings.Load(); saved.TabLayout != want {
			t.Errorf("toggle %d saved layout %q", i+1, saved.TabLayout)
		}
	}
}

// What is chosen in Settings is in the file, and stays when the app reads it
// again.
func TestTabLayoutPersists(t *testing.T) {
	a := testApp(t)
	a.ss.page = "appearance"
	tt := ui.NewTester(a.settingsView, 820, 700)
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	if err := tt.Click("Horizontal"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	if saved := settings.Load(); saved.TabLayout != settings.TabsHorizontal {
		t.Errorf("in the file: layout %q", saved.TabLayout)
	}
	b, _ := os.ReadFile(settings.Path())
	if !strings.Contains(string(b), "tab-layout = horizontal") {
		t.Errorf("the file:\n%s", b)
	}
}

// panesTester shows a session's panes with the tabs and gaps for a test: no
// gap between the cards and the window's edge or between the panes, the
// sidebar hidden, and each pane's card bounds read after every frame.
func panesTester(t *testing.T, direction string) (*App, *ui.Tester, map[string]ui.Rect) {
	t.Helper()
	a := testApp(t)
	a.set.CardGap, a.set.PaneGapSame, a.set.PaneGap = 0, false, 0
	a.prefs.SidebarHidden = true
	var l *proto.Layout
	a.st.Sessions[0].Layout = l.Split("a", "b", direction, 0)
	rects := map[string]ui.Rect{}
	tt := ui.NewTester(func(c *ui.Context) {
		a.view(c)
		for _, id := range []string{"a", "b"} {
			rects[id] = a.paneRef("card-" + id).Bounds(c)
		}
	}, 900, 600)
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	return a, tt, rects
}

// With the panes touching, a drag at their edge resizes them: the strip over
// it takes the place of the gap.
func TestTouchingPanesResize(t *testing.T) {
	for _, direction := range []string{"right", "down"} {
		a, tt, rects := panesTester(t, direction)
		l := a.st.Sessions[0].Layout
		before := l.Ratio
		first := rects["a"]
		x, y := first.X+first.W/2, first.Y+first.H/2
		to := [2]float32{x, y}
		if direction == "right" {
			x, to = first.X+first.W, [2]float32{first.X + first.W + 90, y}
			y, to[1] = first.Y+first.H/2, y
		} else {
			y, to = first.Y+first.H, [2]float32{x, first.Y + first.H + 70}
		}
		tt.Move(x, y)
		tt.Frame()
		tt.Press(x, y)
		tt.Frame()
		tt.Move(to[0], to[1])
		tt.Frame()
		tt.Release(to[0], to[1])
		tt.Frame()
		if l.Ratio <= before {
			t.Errorf("%s: a drag at the edge left the ratio at %v, was %v", direction, l.Ratio, before)
		}
	}
}

// With the panes touching, the corners where they meet are square and the
// others, at the window's edges, are rounded, in a column as in a row. The
// pane in focus is opaque, so its corners show against its center.
func TestTouchingPanesCorners(t *testing.T) {
	corners := map[string]func(ui.Rect) (float32, float32){
		"top left":     func(r ui.Rect) (float32, float32) { return r.X + 1, r.Y + 1 },
		"top right":    func(r ui.Rect) (float32, float32) { return r.X + r.W - 2, r.Y + 1 },
		"bottom left":  func(r ui.Rect) (float32, float32) { return r.X + 1, r.Y + r.H - 2 },
		"bottom right": func(r ui.Rect) (float32, float32) { return r.X + r.W - 2, r.Y + r.H - 2 },
	}
	// The corners that meet the other pane, by direction and the pane in focus.
	meets := map[[2]string][]string{
		{"right", "a"}: {"top right", "bottom right"},
		{"right", "b"}: {"top left", "bottom left"},
		{"down", "a"}:  {"bottom left", "bottom right"},
		{"down", "b"}:  {"top left", "top right"},
	}
	for _, direction := range []string{"right", "down"} {
		for _, focus := range []string{"a", "b"} {
			a, tt, rects := panesTester(t, direction)
			a.st.Sessions[0].Focus = focus
			tt.Frame()
			tt.Advance(time.Second)
			tt.Frame()
			img, card := tt.Image(), rects[focus]
			mid := img.RGBAAt(int(card.X+card.W/2), int(card.Y+card.H/2))
			touching := map[string]bool{}
			for _, name := range meets[[2]string{direction, focus}] {
				touching[name] = true
			}
			for name, at := range corners {
				x, y := at(card)
				if square := img.RGBAAt(int(x), int(y)) == mid; square != touching[name] {
					t.Errorf("%s, %s in focus, %s corner: square %v, want %v", direction, focus, name, square, touching[name])
				}
			}
		}
	}
}
