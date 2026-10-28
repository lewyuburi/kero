package main

import (
	"fmt"
	"hash/fnv"
	"time"

	"github.com/egoist/mygo/ui"

	"kero/internal/actions"
	"kero/internal/proto"
	"kero/internal/settings"

	"kero/locales"
)

// tabBar is the sessions' tabs across the top of the window, where the
// sidebar's list is without settings.FlagHorizontalTabs: each project a
// capsule tinted in its color, as Dia's tab groups are, with its name and its
// sessions' tabs in it, and at the right what the sidebar's footer holds. It
// sits on the title bar's line, after the traffic lights on macOS.
func (a *App) tabBar(c *ui.Context, pal *palette, bar ui.TitleBar) {
	w := a.tabWidth(c, bar)
	ui.Row(c.Key("tabbar")).Height(tabBarH).Shrink(0).AlignItems(ui.Center).Padding(0, 10, 0, lead(bar, a.chrome.gap)).Gap(4).DragWindow().Children(func() {
		if a.chrome.gap == 0 {
			tabLine(c, pal)
		}
		ui.ScrollHorizontal(c.Key("tabs")).Shrink(1).MinWidth(0).Height(tabBarH - 6).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Height(tabBarH - 6).Children(func() {
				if !a.loaded {
					return
				}
				n := 0 // the tabs shown, for ⌘1–9 (Ctrl+1–9 elsewhere)
				for _, g := range a.groups() {
					a.tabGroup(c, pal, g, &n, w)
				}
			})
		})
		iconButton(c, "new-session", plusIcon, a.tip(locales.ActionNewSession(), "session.new"), pal, func() { a.newSession("") })
		ui.Spacer(c)
		// What is at the right keeps its room in a narrow window: the tabs
		// scroll instead.
		ui.Row(c).Shrink(0).Gap(2).AlignItems(ui.Center).Children(func() {
			a.footerStatus(c, pal)
			a.footerButtons(c, pal)
			windowControls(c, pal, a.win)
		})
	})
}

// shownTabs are the sessions of a project the tabs show: all, or the one
// chosen while the project is folded, as the sidebar does.
func (a *App) shownTabs(g group) []*proto.Session {
	if !a.collapsed(g) {
		return g.sessions
	}
	for _, s := range g.sessions {
		if s.ID == a.selected {
			return []*proto.Session{s}
		}
	}
	return nil
}

// tabWidth is the width of a tab: what the window leaves after the groups'
// names, the new-session button and the buttons at the right, shared by the
// tabs shown, between tabMinW and tabMaxW, below which the bar scrolls.
func (a *App) tabWidth(c *ui.Context, bar ui.TitleBar) float32 {
	const rightRoom, groupRoom = 250, 124
	win, _ := c.Size()
	tabs, groups := 0, 0
	for _, g := range a.groups() {
		groups++
		tabs += len(a.shownTabs(g))
	}
	if tabs == 0 {
		return tabMaxW
	}
	room := win - lead(bar, a.chrome.gap) - rightRoom - float32(groups)*groupRoom
	return min(max(room/float32(tabs)-2, tabMinW), tabMaxW)
}

// tabGroup is a project: a capsule tinted in its color, rounded, with no
// edge, holding its name, a hairline and its tabs. The chosen tab is lifted
// in the card's color, lighter than the capsule, as the page it shows.
func (a *App) tabGroup(c *ui.Context, pal *palette, g group, n *int, w float32) {
	key := g.key()
	if g.project.ID == "" && looseOn(a.set) {
		// The sessions of no project are tabs with no group around them.
		for _, s := range g.sessions {
			a.tab(c, pal, s, n, w)
		}
		return
	}
	col := a.projectColor(key)
	tint := col.Fill.Alpha(0.2)
	if pal.dark {
		tint = col.Fill.Alpha(0.3)
	}
	capsule := ui.Row(c.Key("tabgroup-"+key)).Height(tabBarH-8).Padding(0, 3).Gap(2).AlignItems(ui.Center).Radius(12).Shrink(0).Background(tint)
	mark := a.reorderableGroup(capsule, g, true, true)
	capsule.Children(func() {
		a.tabLabel(c, pal, g, col)
		dropMark(c, pal, mark, true)
		shown := a.shownTabs(g)
		if len(shown) > 0 {
			ui.Box(c.Key("sep")).Size(1, 14).Margin(0, 3).Shrink(0).Background(pal.text.Alpha(0.14))
		}
		for _, s := range shown {
			a.tab(c, pal, s, n, w)
		}
	})
}

// tabLabel is a project's name in its group, after a dot of its color: a
// click folds the project to the tab shown, and its menu is the sidebar
// header's, with the color to choose.
func (a *App) tabLabel(c *ui.Context, pal *palette, g group, col projectColor) {
	id, key := g.project.ID, g.key()
	collapsed := a.collapsed(g)
	lbl := ui.Row(c.Key("tabproject-"+key)).Height(tabBarH-14).Padding(0, 8).Gap(6).AlignItems(ui.Center).Radius(9).Shrink(0)
	renaming := id != "" && a.renamingProject == id
	if lbl.Hovered() && !renaming {
		lbl.Background(pal.text.Alpha(0.07))
	}
	if lbl.Clicked() && !renaming {
		a.toggleProject(key)
	}
	pointerTooltip(c, lbl, "tip-tabproject-"+key, g.host.shortDir(g.project.Dir))
	lbl.ContextMenu(func(m *ui.Menu) { a.projectMenu(m, g, collapsed, true) })
	a.groupHandle(lbl, pal, g)
	lbl.Children(func() {
		ui.Box(c).Size(8, 8).Radius(4).Shrink(0).Background(col.Fill)
		if renaming {
			in := ui.TextInputBase(c.Key("rename"), &a.renameBuf).Width(120).FontSize(12.5).FontWeight(600).AutoFocus()
			if in.Shortcut(0, ui.KeyEscape) {
				a.renamingProject = ""
				a.focusTerminal()
			} else if in.Submitted() || (!in.Focused() && !in.Pressed() && in.Valid() && a.renameStarted(c)) {
				a.commitProjectRename()
			}
			return
		}
		ui.Text(c, g.project.Name).FontSize(12.5).FontWeight(600).TextColor(pal.text).SingleLine().Ellipsis("…").MaxWidth(140)
		if collapsed {
			ui.Textf(c, "%d", len(g.sessions)).FontSize(12.5).TextColor(pal.textMuted).FontFeatures("tnum")
		}
	})
}

// projectColor is a project's color in the tabs: Chrome's, as the user chose
// it in the project's menu, else one the project's id picks and keeps.
type projectColor struct {
	ID         string
	Fill, Text ui.Color
}

var projectColors = []projectColor{
	{"grey", ui.Hex("#5f6368"), ui.Hex("#ffffff")},
	{"blue", ui.Hex("#1a73e8"), ui.Hex("#ffffff")},
	{"red", ui.Hex("#d93025"), ui.Hex("#ffffff")},
	{"yellow", ui.Hex("#fbbc04"), ui.Hex("#202124")},
	{"green", ui.Hex("#1e8e3e"), ui.Hex("#ffffff")},
	{"pink", ui.Hex("#d01884"), ui.Hex("#ffffff")},
	{"purple", ui.Hex("#9334e6"), ui.Hex("#ffffff")},
	{"cyan", ui.Hex("#007b83"), ui.Hex("#ffffff")},
	{"orange", ui.Hex("#e8710a"), ui.Hex("#ffffff")},
}

func (a *App) projectColor(id string) projectColor {
	for _, col := range projectColors {
		if col.ID == a.prefs.ProjectColors[id] {
			return col
		}
	}
	if id == "" {
		return projectColors[0] // the sessions of no project
	}
	h := fnv.New32a()
	h.Write([]byte(id))
	return projectColors[1+int(h.Sum32())%(len(projectColors)-1)] // not the grey
}

func (a *App) setProjectColor(id, color string) {
	if a.prefs.ProjectColors == nil {
		a.prefs.ProjectColors = map[string]string{}
	}
	a.prefs.ProjectColors[id] = color
	a.savePrefs()
}

// colorName is a color's name in the app's language.
func colorName(id string) string {
	switch id {
	case "grey":
		return locales.ColorGrey()
	case "blue":
		return locales.ColorBlue()
	case "red":
		return locales.ColorRed()
	case "yellow":
		return locales.ColorYellow()
	case "green":
		return locales.ColorGreen()
	case "pink":
		return locales.ColorPink()
	case "purple":
		return locales.ColorPurple()
	case "cyan":
		return locales.ColorCyan()
	}
	return locales.ColorOrange()
}

// tab is a session as a tab, w wide: the chosen one lifted in the card's
// color, as the page it shows, over its group's tint; its status, but for a session at rest, which
// has nothing to tell; its name, dimmer while it is not the one shown; ⌘ and
// its number while ⌘ is held, else a button that closes it as the pointer
// is over the tab. The menu, rename and tip are the sidebar row's.
func (a *App) tab(c *ui.Context, pal *palette, s *proto.Session, n *int, w float32) {
	*n++
	index := *n
	selected := s.ID == a.selected
	row := ui.Row(c.Key("tab-"+s.ID)).Width(w).Height(tabBarH-14).Shrink(0).Padding(0, 8, 0, 10).Gap(8).AlignItems(ui.Center).Radius(9)
	hovered := row.Hovered()
	// Room for the lift's edge in every tab, so that choosing one moves no text.
	row.Border(1, ui.Transparent)
	bg := ui.Transparent
	switch {
	case selected:
		bg = pal.card
		chosen(row, pal)
	case hovered:
		bg = pal.rowHover
	}
	row.Background(row.AnimateColor("bg", bg, ui.Animation{Duration: 110 * time.Millisecond}))
	middle := a.set.Flag(settings.FlagMiddleClickClose)
	if (row.Pressed() || row.Clicked()) && !(middle && row.MiddlePressed()) && !selected {
		a.selectSession(s.ID)
	}
	if middle && row.MiddleClicked() {
		id := s.ID
		a.defer_(func() { a.closeSession(id, true) })
	}
	mark := a.reorderable(row, pal, s, true)
	if selected && a.revealedTab != s.ID {
		// Chosen by a shortcut or a new session, it comes into view in the
		// tabs, once; in every frame it would keep the user from scrolling.
		row.ScrollIntoView()
		a.revealedTab = s.ID
	}
	if row.DoubleClicked() {
		a.startRename(s.ID)
	}
	tip := proto.ShortDir(s.WorkDir())
	if s.Process != "" {
		tip = s.Process + " · " + tip
	}
	if s.Message != "" {
		tip = s.Message + "\n" + tip
	}
	pointerTooltip(c, row, "tip-tab-"+s.ID, tip)
	row.ContextMenu(func(m *ui.Menu) { a.sessionMenu(m, s) })
	row.Children(func() {
		dropMark(c, pal, mark, true)
		if a.renaming == s.ID {
			in := ui.TextInputBase(c.Key("rename"), &a.renameBuf).Grow(1).MinWidth(0).FontSize(13).AutoFocus()
			if in.Submitted() || in.Shortcut(0, ui.KeyEscape) {
				if in.Shortcut(0, ui.KeyEscape) {
					a.renaming = ""
					a.focusTerminal()
				} else {
					a.commitRename()
				}
			} else if !in.Focused() && !in.Pressed() && in.Valid() && a.renameStarted(c) {
				a.commitRename()
			}
			return
		}
		if s.Status != proto.Idle || s.Unread {
			statusIconP(c, pal, s.Status, s.Unread, s.Progress, 15)
		}
		name := ui.Text(c, s.DisplayName(a.set.SessionName)).FontSize(13).SingleLine().Ellipsis("…").Grow(1).MinWidth(0).TextColor(pal.text)
		switch {
		case s.Unread:
			name.FontWeight(650)
		case !selected:
			name.TextColor(pal.textMuted)
		}
		switch {
		case c.Modifiers() == ui.Cmd && index <= 9:
			ui.Text(c, actions.Glyphs(fmt.Sprintf("CmdOrCtrl+%d", index))).FontSize(12).TextColor(pal.textMuted).FontFeatures("tnum").SingleLine()
		case hovered:
			id := s.ID
			b := ui.ButtonBase(c.Key("close-"+id)).Size(18, 18).Radius(5).Center().Shrink(0).TextColor(pal.textMuted).Label(locales.ActionCloseSession()).Tooltip(locales.ActionCloseSession())
			if b.Hovered() {
				b.Background(pal.rowHover).TextColor(pal.text)
			}
			b.Children(func() { ui.Icon(c, closeIcon).Size(11, 11) })
			if b.Clicked() {
				a.defer_(func() { a.closeSession(id, true) })
			}
		}
	})
}
