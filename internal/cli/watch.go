package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"kero/internal/client"
	"kero/internal/proto"
)

// watchFilter picks the changes `kero watch` shows: those of a session or a
// pane, and status changes to some statuses.
type watchFilter struct {
	session, pane string
	statuses      []proto.Status
}

// match tells that a change is shown. Statuses limit the status changes: a
// session that came or went is not one of them.
func (f watchFilter) match(ch proto.Change) bool {
	if f.session != "" && ch.Session != f.session {
		return false
	}
	if f.pane != "" && ch.Pane != f.pane {
		return false
	}
	if len(f.statuses) > 0 && (ch.Event != proto.ChangeStatus || !slices.Contains(f.statuses, ch.Status)) {
		return false
	}
	return true
}

// writeChange writes a change as a line: its fields, tab separated, or
// JSON.
func writeChange(w io.Writer, ch proto.Change, asJSON bool) error {
	if asJSON {
		return json.NewEncoder(w).Encode(ch)
	}
	fields := []string{ch.Event, ch.Session, ch.Pane, string(ch.Status), string(ch.Previous), ch.Name, ch.Message}
	for i, f := range fields {
		fields[i] = strings.NewReplacer("\t", " ", "\n", " ", "\r", " ").Replace(f)
	}
	_, err := fmt.Fprintln(w, strings.TrimRight(strings.Join(fields, "\t"), "\t"))
	return err
}

// watch prints what changes in the sessions as it happens, until it is
// stopped, or after the first change shown with --once.
func watch(c *client.Client, st proto.State, args []string) error {
	f, err := parse(args, map[string]string{"for": "for"}, map[string]string{"json": "json", "once": "once"})
	if err != nil {
		return err
	}
	var filter watchFilter
	if len(f.pos) > 0 {
		t, err := find(st, f.pos[0])
		if err != nil {
			return err
		}
		filter.session, filter.pane = t.s.ID, t.pane
	}
	for _, w := range strings.Split(f.vals["for"], ",") {
		if w = strings.TrimSpace(w); w != "" {
			filter.statuses = append(filter.statuses, proto.Status(w))
		}
	}
	changes := make(chan proto.Change, 64)
	c.OnEvent(func(m proto.Message) {
		if m.Event == "change" && m.Change != nil {
			select {
			case changes <- *m.Change:
			default:
			}
		}
	})
	if err := c.Call("watch", nil, nil); err != nil {
		if strings.Contains(err.Error(), "unknown method") {
			return fmt.Errorf("the running kerod predates kero watch: run `kero daemon restart` (it restarts sessions)")
		}
		return err
	}
	for {
		select {
		case ch := <-changes:
			if !filter.match(ch) {
				continue
			}
			if err := writeChange(os.Stdout, ch, f.set["json"]); err != nil {
				return err
			}
			if f.set["once"] {
				return nil
			}
		case <-c.Done():
			return fmt.Errorf("%w: kerod stopped", errUnreachable)
		}
	}
}
