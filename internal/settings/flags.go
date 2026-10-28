package settings

import (
	"fmt"
	"runtime"
	"strings"
)

// A Flag is an experiment, as Chrome's: enabled or disabled by default, which may
// depend on the platform, and the user's choice over that default, which
// Advanced shows and the configuration holds (`flag = id=enabled`).
type Flag struct {
	ID      string
	Default func(goos string) bool
}

// FlagCustomSpacing lets card-gap and pane-gap apply, and moves the window
// controls with them. It was made on macOS and not looked at elsewhere, and
// like any experiment starts disabled: nothing changes until it is enabled.
const FlagCustomSpacing = "custom-spacing"

// FlagHorizontalTabs offers the sessions' tabs across the top of the window,
// grouped by project in tinted colors as Dia's are, besides down the sidebar:
// with it enabled, Appearance lets the user choose, and ⌥⌘B switches. It
// changes nothing by itself: the choice is tab-layout's, vertical until the
// user makes it. It was made on macOS and not looked at elsewhere, and
// starts disabled.
const FlagHorizontalTabs = "horizontal-tabs"

// FlagLooseSessions lets a session belong to no project: one started in a
// folder that is neither a project's nor a git repository's, one started
// loose on purpose (New Session in Home), one moved out of its project.
// They show first in the sidebar, with no header, and across the top outside
// any group. It starts disabled: sessions are always in a project.
const FlagLooseSessions = "loose-sessions"

// FlagMiddleClickClose closes a session with a click of the middle button on
// its tab or its row in the sidebar. It starts disabled: Kero's own does not.
const FlagMiddleClickClose = "middle-click-close"

// retiredFlags are flags that were, which a configuration may still name:
// they are ignored, not a problem.
var retiredFlags = map[string]bool{"tab-groups": true}

// Flags are the flags there are, in the order Advanced lists them.
var Flags = []Flag{
	{FlagCustomSpacing, func(string) bool { return false }},
	{FlagHorizontalTabs, func(string) bool { return false }},
	{FlagLooseSessions, func(string) bool { return false }},
	{FlagMiddleClickClose, func(string) bool { return false }},
}

// The states of a flag, as Advanced offers them: the default's, or a choice
// over it.
const (
	StateDefault  = "default"
	StateEnabled  = "enabled"
	StateDisabled = "disabled"
)

// FlagByID returns the flag with that id, or nil.
func FlagByID(id string) *Flag {
	for i := range Flags {
		if Flags[i].ID == id {
			return &Flags[i]
		}
	}
	return nil
}

// FlagDefault tells whether a flag is on in a new install on this platform.
func FlagDefault(id string) bool {
	f := FlagByID(id)
	return f != nil && f.Default(runtime.GOOS)
}

// Flag tells whether a flag is on: the user's choice, else the default.
func (s Settings) Flag(id string) bool {
	if v, ok := s.Flags[id]; ok {
		return v
	}
	return FlagDefault(id)
}

// FlagState tells how a flag stands: StateEnabled or StateDisabled when the user chose,
// else StateDefault.
func (s Settings) FlagState(id string) string {
	switch v, ok := s.Flags[id]; {
	case !ok:
		return StateDefault
	case v:
		return StateEnabled
	}
	return StateDisabled
}

// SetFlag makes a flag on, off or the default's.
func (s *Settings) SetFlag(id, state string) {
	if state == StateDefault {
		delete(s.Flags, id)
		if len(s.Flags) == 0 {
			s.Flags = nil // as a file with no flags parses, so that they compare equal
		}
		return
	}
	if s.Flags == nil {
		s.Flags = map[string]bool{}
	}
	s.Flags[id] = state == StateEnabled
}

// flagKey is the configuration's `flag = <id>=enabled|disabled`, one a line.
var flagKey = key{name: "flag", list: true,
	doc: "<id>=enabled or <id>=disabled, over a flag's default, as custom-spacing=enabled; Settings > Advanced lists them",
	get: func(s *Settings) []string {
		var out []string
		for _, f := range Flags {
			if st := s.FlagState(f.ID); st != StateDefault {
				out = append(out, f.ID+"="+st)
			}
		}
		return out
	},
	set: func(s *Settings, v []string) error {
		s.Flags = nil
		var bad []string
		for _, kv := range v {
			id, state, ok := strings.Cut(kv, "=")
			id, state = strings.TrimSpace(id), strings.TrimSpace(state)
			switch {
			case !ok || state != StateEnabled && state != StateDisabled:
				bad = append(bad, fmt.Sprintf("%q is not <id>=enabled or <id>=disabled", kv))
			case retiredFlags[id]:
			case FlagByID(id) == nil:
				bad = append(bad, fmt.Sprintf("no flag %q", id))
			default:
				s.SetFlag(id, state)
			}
		}
		if bad != nil {
			return fmt.Errorf("%s", strings.Join(bad, "; "))
		}
		return nil
	}}
