package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"kero/internal/proto"
	"kero/internal/settings"
)

// manySessions is an app with n sessions in projects of ten, each of two panes.
func manySessions(b *testing.B, n int) *App {
	b.Helper()
	a := testApp(&testing.T{}) // only for its fixtures: no failures to report
	a.st.Sessions, a.st.Projects = nil, nil
	for i := 0; i < n; i++ {
		pid := fmt.Sprintf("p%d", i/10)
		if i%10 == 0 {
			a.st.Projects = append(a.st.Projects, proto.Project{ID: pid, Name: "project " + pid, Dir: "/tmp"})
		}
		var l *proto.Layout
		a.st.Sessions = append(a.st.Sessions, proto.Session{
			ID: fmt.Sprintf("s%d", i), Project: pid, Name: fmt.Sprintf("session %d", i),
			Panes: []proto.Pane{
				{ID: fmt.Sprintf("a%d", i), Shell: "fish", Dir: "/tmp", Status: proto.Idle, Progress: -1},
				{ID: fmt.Sprintf("b%d", i), Shell: "zsh", Dir: "/tmp", Status: proto.Working, Progress: -1},
			},
			Layout: l.Split(fmt.Sprintf("a%d", i), fmt.Sprintf("b%d", i), "right", 0),
			Focus:  fmt.Sprintf("a%d", i),
		})
	}
	a.selected = "s0"
	return a
}

// BenchmarkFrame is what one frame of the main window costs to build: the
// sidebar and the tabs across the top, with a few sessions and with many.
func BenchmarkFrame(b *testing.B) {
	for _, horizontal := range []bool{false, true} {
		for _, n := range []int{3, 30, 200} {
			name := fmt.Sprintf("sidebar/%d", n)
			if horizontal {
				name = fmt.Sprintf("tabs/%d", n)
			}
			b.Run(name, func(b *testing.B) {
				a := manySessions(b, n)
				if horizontal {
					a.set.SetFlag(settings.FlagHorizontalTabs, settings.StateEnabled)
					a.set.TabLayout = settings.TabsHorizontal
				}
				tt := ui.NewTester(a.view, 1400, 800)
				tt.Frame()
				tt.Advance(time.Second)
				tt.Frame()
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					tt.Frame()
				}
			})
		}
	}
}
