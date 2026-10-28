package main

import (
	"slices"

	"github.com/egoist/mygo/ui"

	"kero/internal/proto"
	"kero/internal/settings"
)

// tabDrag is a session dragged by its tab or its row in the sidebar.
type tabDrag struct{ id string }

// reorderable makes a tab, or a row of the sidebar, the handle of its
// session, dragged to another place in its project, and a place for another
// of the project's to drop. It returns where a drop would go, for dropMark
// to draw in the row's children: -1 before, 1 after, 0 none.
func (a *App) reorderable(row ui.Element, pal *palette, s *proto.Session, horizontal bool) int {
	if !a.set.Flag(settings.FlagTabReorder) || a.renaming == s.ID {
		return 0
	}
	row.Drag(tabDrag{s.ID})
	if row.Dragging() {
		// What follows the pointer is a copy of the row: a chip.
		floating(row.Background(pal.popover), pal)
	}
	mark := 0
	if d, over := ui.DragOver[tabDrag](row); over && a.reorders(d.id, s) {
		mark = dropSide(row, horizontal)
	}
	if d, ok := ui.Drop[tabDrag](row); ok && a.reorders(d.id, s) {
		id, target, after := d.id, s.ID, dropSide(row, horizontal) > 0
		a.defer_(func() { a.moveBeside(id, target, after) })
	}
	return mark
}

// dropSide is which half of a row the pointer is over, along the axis the
// rows follow: -1 the first, before, and 1 the rest, after.
func dropSide(row ui.Element, horizontal bool) int {
	x, y, in := row.PointerPosition()
	b := row.Bounds()
	if !in {
		return -1
	}
	if !horizontal {
		x, b.W = y, b.H
	}
	if x > b.W/2 {
		return 1
	}
	return -1
}

// dropMark draws where a drop would go, a line along the row's edge, to be
// called in its children.
func dropMark(c *ui.Context, pal *palette, mark int, horizontal bool) {
	if mark == 0 {
		return
	}
	line := ui.Box(c.Key("drop-mark")).Absolute().PassThrough().Background(pal.accent)
	switch {
	case horizontal && mark < 0:
		line.Top(3).Bottom(3).Left(-1).Width(2)
	case horizontal:
		line.Top(3).Bottom(3).Right(-1).Width(2)
	case mark < 0:
		line.Left(6).Right(6).Top(-1).Height(2)
	default:
		line.Left(6).Right(6).Bottom(-1).Height(2)
	}
}

// reorders reports whether the session dragged may be dropped by s: another
// one, of the same project.
func (a *App) reorders(dragged string, s *proto.Session) bool {
	d := a.session(dragged)
	return d != nil && d.ID != s.ID && d.Project == s.Project && a.hostOf(d.ID) == a.hostOf(s.ID)
}

// moveBeside puts a session before another of its project's, or after it.
func (a *App) moveBeside(id, target string, after bool) {
	d, t := a.session(id), a.session(target)
	if d == nil || t == nil || d.Project != t.Project {
		return
	}
	// Index counts the project's sessions without the one moved.
	index, n := -1, 0
	for i := range a.st.Sessions {
		o := &a.st.Sessions[i]
		if o.ID == id || o.Project != d.Project || a.hostOf(o.ID) != a.hostOf(id) {
			continue
		}
		if o.ID == target {
			index = n
			if after {
				index++
			}
		}
		n++
	}
	if index < 0 || index >= n {
		index = -1
	}
	// At once; the daemon's state follows.
	moved := *d
	at := slices.IndexFunc(a.st.Sessions, func(o proto.Session) bool { return o.ID == id })
	a.st.Sessions = slices.Delete(a.st.Sessions, at, at+1)
	to := slices.IndexFunc(a.st.Sessions, func(o proto.Session) bool { return o.ID == target })
	if after {
		to++
	}
	a.st.Sessions = slices.Insert(a.st.Sessions, to, moved)
	a.call(id, "move", proto.MoveParams{ID: id, Project: moved.Project, Index: index}, nil)
}

// groupDrag is a project dragged by its name in the tabs.
type groupDrag struct{ id string }

// groupHandle makes label, a project's name in the tabs, the handle that
// drags it.
func (a *App) groupHandle(label ui.Element, pal *palette, g group) {
	if !a.set.Flag(settings.FlagTabReorder) || g.project.ID == "" || a.renamingProject == g.project.ID {
		return
	}
	label.Drag(groupDrag{g.project.ID})
	if label.Dragging() {
		floating(label.Background(pal.popover), pal)
	}
}

// reorderableGroup makes a project's capsule in the tabs, or its header in
// the sidebar, a place to drop another project of its host's. It returns
// where a drop would go, as reorderable does. With halves the header's
// lower half is after the project; without, a drop is always before it, as
// the header is above the sessions the project holds.
func (a *App) reorderableGroup(capsule ui.Element, g group, horizontal, halves bool) int {
	if !a.set.Flag(settings.FlagTabReorder) {
		return 0
	}
	side := func() int {
		if halves {
			return dropSide(capsule, horizontal)
		}
		return -1
	}
	mark := 0
	if d, over := ui.DragOver[groupDrag](capsule); over && a.reorderesGroup(d.id, g) {
		mark = side()
	}
	if d, ok := ui.Drop[groupDrag](capsule); ok && a.reorderesGroup(d.id, g) {
		id, target, after := d.id, g.project.ID, side() > 0
		a.defer_(func() { a.moveGroupBeside(id, target, after) })
	}
	return mark
}

// reorderesGroup reports whether the project dragged may be dropped by g:
// another one, of the same host.
func (a *App) reorderesGroup(dragged string, g group) bool {
	return dragged != g.project.ID && a.project(dragged) != nil && a.hostOf(dragged) == g.host
}

// moveGroupBeside puts a project before another of its host's, or after it.
func (a *App) moveGroupBeside(id, target string, after bool) {
	if a.project(id) == nil || a.project(target) == nil || a.hostOf(id) != a.hostOf(target) {
		return
	}
	h := a.hostOf(id)
	// Index counts the host's projects without the one moved.
	index, n := -1, 0
	for _, p := range a.st.Projects {
		if p.ID == id || a.hostOf(p.ID) != h {
			continue
		}
		if p.ID == target {
			index = n
			if after {
				index++
			}
		}
		n++
	}
	if index < 0 {
		return
	}
	// At once; the daemon's state follows.
	at := slices.IndexFunc(a.st.Projects, func(p proto.Project) bool { return p.ID == id })
	moved := a.st.Projects[at]
	a.st.Projects = slices.Delete(a.st.Projects, at, at+1)
	to := slices.IndexFunc(a.st.Projects, func(p proto.Project) bool { return p.ID == target })
	if after {
		to++
	}
	a.st.Projects = slices.Insert(a.st.Projects, to, moved)
	a.call(id, "project.move", proto.MoveProjectParams{ID: id, Index: index}, nil)
}
