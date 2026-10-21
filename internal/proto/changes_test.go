package proto

import (
	"reflect"
	"testing"
)

func TestChanges(t *testing.T) {
	pane := func(id string, st Status) Pane { return Pane{ID: id, Status: st, Dir: "/tmp"} }
	session := func(id string, panes ...Pane) Session {
		return Session{ID: id, Name: id + "-name", Panes: panes}
	}
	old := State{Sessions: []Session{
		session("a", pane("a1", Working), pane("a2", Idle)),
		session("b", pane("b1", Idle)),
	}}
	cur := State{Sessions: []Session{
		session("a", pane("a1", Waiting), pane("a2", Idle), pane("a3", Working)),
		session("c", pane("c1", Idle)),
	}}
	cur.Sessions[0].Panes[0].Message = "Approve?"
	want := []Change{
		{Event: ChangeStatus, Session: "a", Pane: "a1", Name: "a-name", Dir: "/tmp", Status: Waiting, Previous: Working, Message: "Approve?"},
		{Event: ChangeSessionNew, Session: "c", Name: "c-name"},
		{Event: ChangeSessionClosed, Session: "b", Name: "b-name"},
	}
	if got := Changes(old, cur); !reflect.DeepEqual(got, want) {
		t.Errorf("changes\n got %+v\nwant %+v", got, want)
	}
	if got := Changes(cur, cur); len(got) != 0 {
		t.Errorf("a state against itself changed %+v", got)
	}
}
