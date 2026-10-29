package main

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"kero/locales"
)

// fileManager is the system's file manager, as buttons that show a file
// in it name it.
func fileManager() string {
	switch runtime.GOOS {
	case "darwin":
		return locales.FileManagerFinder()
	case "windows":
		return locales.FileManagerExplorer()
	}
	return locales.FileManagerFiles()
}

// openPath opens a file or folder in the app that opens it.
func openPath(path string) { mygo.Shell.OpenPath(path) }

// revealPath shows a file in the file manager.
func revealPath(path string) { mygo.Shell.ShowItemInFolder(path) }

// openTextFile opens a text file, as the configuration, which has no
// extension to tell its app by, in the text editor.
func openTextFile(path string) {
	switch runtime.GOOS {
	case "darwin":
		exec.Command("/usr/bin/open", "-t", path).Start()
	case "windows":
		exec.Command("notepad.exe", path).Start()
	default:
		exec.Command("xdg-open", path).Start()
	}
}

// ownControls tells that Kero draws the window frame itself, without the
// system's buttons: on Windows and Linux. macOS keeps its traffic lights.
// Whether those buttons are drawn is windowButtons: Omarchy's chrome flag
// hides them.
var ownControls = runtime.GOOS != "darwin"

// omarchy is this machine: Omarchy's compositor rounds the window and
// closes, minimizes and maximizes it, so Kero does not draw those buttons
// and its panels take that corner.
var omarchy = omarchyOS()

// systemRounding is Hyprland's decoration:rounding, in layout pixels.
// systemRoundingOK is false when this is not Omarchy or hyprctl did not
// answer, and the panels then keep Kero's own curve.
var systemRounding, systemRoundingOK = hyprRounding()

// omarchyOS reports whether this is Omarchy, from the OS identity.
func omarchyOS() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	b, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(b), "\n") {
		if line == "ID=omarchy" {
			return true
		}
	}
	return false
}

// hyprRounding reads the compositor's corner. Anywhere but Omarchy it
// reports nothing, and the panels keep their own radius.
func hyprRounding() (float32, bool) {
	if !omarchy {
		return 0, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "hyprctl", "getoption", "decoration:rounding").Output()
	if err != nil {
		return 0, false
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "int:") {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "int:")))
		if err != nil || n < 0 {
			return 0, false
		}
		return float32(n), true
	}
	return 0, false
}

// framed sets a window's frame: the title bar hidden, under the traffic
// lights on macOS and without its buttons elsewhere (ownControls), which
// keeps the edges that resize the window.
func framed(mid float32, o mygo.WindowOptions) mygo.WindowOptions {
	if ownControls {
		o.TitleBarStyle = mygo.TitleBarHidden
		o.HideWindowButtons = true
		// No menu bar over the sidebar and the cards: Alt and F10 show the
		// menus, whose shortcuts work all the same.
		o.AutoHideMenuBar = true
		return o
	}
	o.TitleBarStyle = mygo.TitleBarHiddenInset
	o.TitleBarHeight = int(2 * mid)
	// The lights' buttons are 16 tall, and MyGo puts them a DIP above the
	// position: centered on barMid, as the sidebar's buttons and the
	// panes' headers are.
	o.TrafficLightPosition = &mygo.Point{X: lightsX, Y: lightsY(mid)}
	return o
}

// windowRadius is the corner of the window the cards sit in, as the system
// rounds it. On macOS it is 16, measured on a window like Kero's (hidden
// inset title bar); Windows 11 and GNOME draw 8 and 12 by their documentation,
// and neither was measured here. Omarchy's chrome flag uses the
// compositor's own corner (systemRounding) for the panels instead.
var windowRadius = func() float32 {
	switch runtime.GOOS {
	case "darwin":
		return 16
	case "windows":
		return 8
	}
	return 12
}()

// lightsX is where the traffic lights start, from the window's left.
const lightsX = 18

// lightsY is where they start from its top, for a bar whose line is mid: the
// buttons are 16 tall, and MyGo puts them a DIP above the position.
func lightsY(mid float32) int { return int(mid) - 7 }

// moveLights puts a window's traffic lights on the bar's line, mid, as it
// changes with the window open. A window of Windows or Linux has none to
// move.
func moveLights(w *mygo.Window, mid float32) {
	if w != nil && !ownControls {
		w.SetTrafficLightPosition(lightsX, lightsY(mid))
	}
}

var (
	minimizeIcon = icon(`<path d="M3.5 8h9"/>`)
	maximizeIcon = icon(`<rect x="3.5" y="3.5" width="9" height="9" rx="1.5"/>`)
	restoreIcon  = icon(`<rect x="3.5" y="5.5" width="7" height="7" rx="1.2"/><path d="M5.5 3.5h5.5a1.5 1.5 0 0 1 1.5 1.5v5.5"/>`)
)

// windowControls are a window's minimize, maximize and close buttons, as
// Kero draws them (ownControls), at the right of a bar. draw is
// windowButtons: off on macOS, and on Omarchy while its chrome flag is on.
// A view built without a window, as the tests do, still lays them out.
func windowControls(c *ui.Context, pal *palette, w *mygo.Window, draw bool) {
	if !draw {
		return
	}
	ui.Row(c.Key("window-controls")).Gap(2).AlignItems(ui.Center).Shrink(0).Margin(0, 0, 0, 6).Children(func() {
		iconButton(c, "minimize", minimizeIcon, locales.WindowMinimize(), pal, func() {
			if w != nil {
				w.Minimize()
			}
		})
		// The system's maximize button for the pointer, which brings
		// Windows 11's snap layouts as it rests there: no tooltip over them.
		maximized := w != nil && w.IsMaximized()
		label, svg := locales.WindowMaximize(), maximizeIcon
		if maximized {
			label, svg = locales.WindowRestore(), restoreIcon
		}
		m := ui.ButtonBase(c.Key("maximize")).Size(28, 26).Radius(6).Center().TextColor(pal.textMuted).Label(label).MaximizeButton()
		if m.Pressed() {
			m.Background(pal.rowHover.Alpha(0.12))
		} else if m.Hovered() {
			m.Background(pal.rowHover).TextColor(pal.text)
		}
		m.Children(func() { ui.Icon(c, svg).Size(16, 16) })
		if m.Clicked() && w != nil {
			if maximized {
				w.Unmaximize()
			} else {
				w.Maximize()
			}
		}
		b := ui.ButtonBase(c.Key("close-window")).Size(28, 26).Radius(6).Center().TextColor(pal.textMuted).Label(locales.CommonClose()).Tooltip(locales.CommonClose())
		if b.Hovered() || b.Pressed() {
			b.Background(pal.danger).TextColor(ui.RGB(255, 255, 255))
		}
		b.Children(func() { ui.Icon(c, closeIcon).Size(16, 16) })
		if b.Clicked() && w != nil {
			w.Close()
		}
	})
}
