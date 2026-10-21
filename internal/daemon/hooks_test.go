package daemon

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"kero/internal/proto"
	"kero/internal/settings"
)

// A hook runs its command with what happened in the environment.
func TestRunHook(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the hook's command is a shell's")
	}
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	h := settings.StatusHook{Status: proto.Waiting, Command: `printf '%s|%s|%s|%s|%s|%s' "$KERO_EVENT" "$KERO_SESSION" "$KERO_PANE" "$KERO_STATUS" "$KERO_PREVIOUS" "$KERO_MESSAGE" > ` + out + `; pwd >> ` + out}
	ch := proto.Change{Event: proto.ChangeStatus, Session: "s1", Pane: "p1", Dir: dir, Status: proto.Waiting, Previous: proto.Working, Message: "Approve?"}
	if err := runHook(h, ch); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	if !strings.HasPrefix(got, "status|s1|p1|waiting|working|Approve?") {
		t.Errorf("the hook saw %q", got)
	}
	if want, _ := filepath.EvalSymlinks(dir); !strings.HasSuffix(strings.TrimSpace(got), want) {
		t.Errorf("the hook ran in %q, want %q", got, want)
	}
}

// Only the hooks of the status a pane came to run, and no more than
// hookLimit at once.
func TestRunHooksMatchStatus(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the hook's command is a shell's")
	}
	home := t.TempDir()
	t.Setenv("KERO_HOME", home)
	t.Setenv("KERO_CONFIG", filepath.Join(home, "config"))
	marks := filepath.Join(home, "marks")
	os.WriteFile(filepath.Join(home, "config"), []byte(
		"on-status = waiting echo waiting >> "+marks+"\non-status = idle echo idle >> "+marks+"\n"), 0o600)
	d := &Daemon{hookSlots: make(chan struct{}, hookLimit), log: log.New(io.Discard, "", 0)}
	d.runHooks([]proto.Change{
		{Event: proto.ChangeSessionNew, Session: "s1"},
		{Event: proto.ChangeStatus, Session: "s1", Pane: "p1", Status: proto.Waiting, Previous: proto.Working},
	})
	deadline := time.Now().Add(5 * time.Second)
	for {
		b, _ := os.ReadFile(marks)
		if strings.TrimSpace(string(b)) == "waiting" {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("hooks wrote %q, want just waiting", b)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// Watchers get the changes, and a subscriber, the app, does not: nothing
// the app answers goes to a watcher either.
func TestWatchSendsChangesToWatchers(t *testing.T) {
	watcher := &controlConn{out: make(chan proto.Message, 4)}
	app := &controlConn{out: make(chan proto.Message, 4)}
	d := &Daemon{
		watchers: map[*controlConn]struct{}{watcher: {}},
		subs:     map[*controlConn]struct{}{app: {}},
	}
	d.watch(nil)
	d.watch([]proto.Change{{Event: proto.ChangeStatus, Session: "s1", Pane: "p1", Status: proto.Idle, Previous: proto.Working}})
	select {
	case m := <-watcher.out:
		if m.Event != "change" || m.Change == nil || m.Change.Status != proto.Idle {
			t.Errorf("the watcher got %+v", m)
		}
	default:
		t.Fatal("the watcher got nothing")
	}
	select {
	case m := <-app.out:
		t.Errorf("the app got %+v", m)
	default:
	}
}
