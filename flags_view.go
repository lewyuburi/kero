package main

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"kero/internal/settings"
	"kero/locales"
)

// flagsChanged counts the flags the user chose over their defaults.
func (a *App) flagsChanged() int {
	n := 0
	for _, f := range settings.Flags {
		if a.set.FlagState(f.ID) != settings.StateDefault {
			n++
		}
	}
	return n
}

// flagShown tells that a flag is in the list for the search and for "only the
// changed": its id, its name and what it does are what the search reads.
func (a *App) flagShown(id, name, detail string) bool {
	if a.ss.flagsChanged && a.set.FlagState(id) == settings.StateDefault {
		return false
	}
	q := strings.ToLower(strings.TrimSpace(a.ss.flagFilter))
	if q == "" {
		return true
	}
	text := strings.ToLower(id + " " + name + " " + detail)
	for _, word := range strings.Fields(q) {
		if !strings.Contains(text, word) {
			return false
		}
	}
	return true
}

// flagsEntry is what Advanced shows of the flags: a row that opens their page.
func (a *App) flagsEntry(c *ui.Context, pal *palette) {
	settingsGroup(c, pal, locales.SetFlags(), locales.SetFlagsDetail(), func() {
		settingRow(c, pal, locales.SetFlagsRow(), locales.SetFlagsSummary(a.flagsChanged(), len(settings.Flags)), func() {
			if button(c, "open-flags", locales.SetFlagsOpen(), "", false, pal) {
				a.ss.flagsOpen = true
			}
		})
	})
}

// flagsPage lists the flags as Firefox's about:config lists its preferences:
// a search, a switch for only the changed, and a row for each, at the
// pop-up that sets it, with a way back to Advanced.
func (a *App) flagsPage(c *ui.Context, pal *palette) {
	back := ui.ButtonBase(c.Key("flags-back")).Height(28).Padding(0, 10, 0, 6).Gap(4).Radius(8).AlignItems(ui.Center).AlignSelf(ui.Start).Label(locales.SetFlagsBack(locales.PageAdvanced()))
	fg := pal.textMuted
	if back.Hovered() {
		back.Background(pal.rowHover)
		fg = pal.text
	}
	back.TextColor(fg).Children(func() {
		ui.Icon(c, chevronIcon).Size(11, 11).Rotate(180)
		ui.Text(c, locales.PageAdvanced()).FontSize(13)
	})
	if back.Clicked() {
		a.ss.flagsOpen = false
	}
	ui.Text(c, locales.SetFlags()).FontSize(15).FontWeight(650).TextColor(pal.text)
	ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
		textField(c, "flags-search", &a.ss.flagFilter, locales.SetFlagsSearch(), 0, pal)
		ui.Row(c).Gap(8).AlignItems(ui.Center).Shrink(0).Children(func() {
			toggle(c.Key("flags-changed"), &a.ss.flagsChanged, pal)
			ui.Text(c, locales.SetFlagsChangedOnly()).FontSize(13).TextColor(pal.text).SingleLine()
		})
	})
	shown := 0
	ui.Column(c).Radius(12).Background(pal.chip).Clip().Children(func() {
		for _, f := range settings.Flags {
			name, detail := flagText(f.ID)
			if !a.flagShown(f.ID, name, detail) {
				continue
			}
			a.flagRow(c, pal, f, name, detail, shown%2 == 1)
			shown++
		}
		if shown == 0 {
			ui.Text(c, locales.SetFlagsNone()).FontSize(13).TextColor(pal.textFaint).Padding(18, 16)
		}
	})
}

// flagChoice is where a flag's pop-up keeps its choice: a field that lasts from
// one build to the next, as a bound value must (MyGo updates it after the
// pass that builds the pop-up, so a value made fresh in each pass loses the
// edit before it is read), which the settings follow, and follow in turn: when
// they say another thing, as a file edited does, it takes that.
func (a *App) flagChoice(id string) *string {
	if a.ss.flagVals == nil {
		a.ss.flagVals, a.ss.flagSeen = map[string]*string{}, map[string]string{}
	}
	v, seen := a.ss.flagVals[id]
	cur := a.set.FlagState(id)
	if !seen || a.ss.flagSeen[id] != cur {
		if !seen {
			v = new(string)
			a.ss.flagVals[id] = v
		}
		*v, a.ss.flagSeen[id] = cur, cur
	}
	return v
}

// flagRow is a flag: its id as the configuration names it, then its name and
// what it does, and at the right how it stands and the pop-up that sets it.
// A flag the user chose over the default has a dot of the accent's before
// it. Every other row is shaded, as the rows of about:config are.
func (a *App) flagRow(c *ui.Context, pal *palette, f settings.Flag, name, detail string, shaded bool) {
	row := ui.Row(c.Key("flag-row-"+f.ID)).Padding(12, 16).Gap(16).AlignItems(ui.Center)
	if shaded {
		row.Background(pal.text.Alpha(0.035))
	}
	row.Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
			ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
				if a.set.FlagState(f.ID) != settings.StateDefault {
					ui.Box(c).Size(6, 6).Radius(3).Shrink(0).Background(pal.accent)
				}
				ui.Text(c, f.ID).Font("monospace").FontSize(13).FontWeight(600).TextColor(pal.text).SingleLine()
				ui.Text(c, name).FontSize(12.5).TextColor(pal.textMuted).SingleLine().Ellipsis("…").MinWidth(0)
				infoTip(c, pal, f.ID, detail)
			})
		})
		v := a.flagChoice(f.ID)
		popup(c, "flag-"+f.ID, v, []string{settings.StateDefault, settings.StateEnabled, settings.StateDisabled}, flagStateName(f.ID), 150, pal)
		if cur := a.set.FlagState(f.ID); *v != cur {
			wasOn := a.set.Flag(f.ID)
			a.set.SetFlag(f.ID, *v)
			if f.ID == settings.FlagHorizontalTabs && !wasOn && a.set.Flag(f.ID) {
				// Enabling it only offers the choice: the tabs stay where they
				// are, down the side, whatever an earlier choice left in the file.
				a.set.TabLayout = settings.TabsVertical
			}
		}
	})
}
