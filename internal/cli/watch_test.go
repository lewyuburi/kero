package cli

import (
	"bytes"
	"strings"
	"testing"

	"kero/internal/proto"
)

func TestWatchFilter(t *testing.T) {
	waiting := proto.Change{Event: proto.ChangeStatus, Session: "s1", Pane: "p1", Status: proto.Waiting, Previous: proto.Working}
	idle := proto.Change{Event: proto.ChangeStatus, Session: "s2", Pane: "p2", Status: proto.Idle, Previous: proto.Working}
	came := proto.Change{Event: proto.ChangeSessionNew, Session: "s3"}
	for name, c := range map[string]struct {
		f    watchFilter
		want []bool // waiting, idle, came
	}{
		"all":          {watchFilter{}, []bool{true, true, true}},
		"a session":    {watchFilter{session: "s1"}, []bool{true, false, false}},
		"a pane":       {watchFilter{session: "s2", pane: "p2"}, []bool{false, true, false}},
		"a status":     {watchFilter{statuses: []proto.Status{proto.Waiting}}, []bool{true, false, false}},
		"two statuses": {watchFilter{statuses: []proto.Status{proto.Waiting, proto.Idle}}, []bool{true, true, false}},
	} {
		for i, ch := range []proto.Change{waiting, idle, came} {
			if got := c.f.match(ch); got != c.want[i] {
				t.Errorf("%s: change %d matched %v, want %v", name, i, got, c.want[i])
			}
		}
	}
}

func TestWriteChange(t *testing.T) {
	ch := proto.Change{Event: proto.ChangeStatus, Session: "s1", Pane: "p1", Name: "api", Status: proto.Waiting, Previous: proto.Working, Message: "Approve\tthe\ndeploy?"}
	var b bytes.Buffer
	if err := writeChange(&b, ch, false); err != nil {
		t.Fatal(err)
	}
	if want := "status\ts1\tp1\twaiting\tworking\tapi\tApprove the deploy?\n"; b.String() != want {
		t.Errorf("line %q, want %q", b.String(), want)
	}
	b.Reset()
	writeChange(&b, proto.Change{Event: proto.ChangeSessionClosed, Session: "s2"}, false)
	if want := "session.closed\ts2\n"; b.String() != want {
		t.Errorf("line %q, want %q", b.String(), want)
	}
	b.Reset()
	writeChange(&b, ch, true)
	if !strings.Contains(b.String(), `"status":"waiting"`) || !strings.HasSuffix(b.String(), "}\n") {
		t.Errorf("json %q", b.String())
	}
}
