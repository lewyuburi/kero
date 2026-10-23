package daemon

import (
	"context"
	"io"
	"os"
	"os/exec"
	"runtime"
	"time"

	"kero/internal/proto"
	"kero/internal/settings"
)

// hookTimeout ends a hook that does not, and hookLimit is how many run at
// once: a program flapping between statuses queues no more than this.
const (
	hookTimeout = time.Minute
	hookLimit   = 8
)

// runHooks runs the on-status commands of the settings for the status
// changes among changes, each on its own, off the broadcast loop.
func (d *Daemon) runHooks(changes []proto.Change) {
	if len(changes) == 0 {
		return
	}
	hooks := settings.Load().StatusHooks()
	if len(hooks) == 0 {
		return
	}
	for _, ch := range changes {
		if ch.Event != proto.ChangeStatus {
			continue
		}
		for _, h := range hooks {
			if h.Status != ch.Status {
				continue
			}
			select {
			case d.hookSlots <- struct{}{}:
				go func(h settings.StatusHook, ch proto.Change) {
					defer func() { <-d.hookSlots }()
					if err := runHook(h, ch); err != nil {
						d.log.Printf("on-status %s %q: %v", h.Status, h.Command, err)
					}
				}(h, ch)
			default:
				d.log.Printf("on-status %s %q: too many hooks running", h.Status, h.Command)
			}
		}
	}
}

// runHook runs a hook's command in the shell for a change and waits for it.
func runHook(h settings.StatusHook, ch proto.Change) error {
	ctx, cancel := context.WithTimeout(context.Background(), hookTimeout)
	defer cancel()
	cmd := hookCommand(ctx, h, ch)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, io.Discard, io.Discard
	return cmd.Run()
}

// hookCommand is the command of a hook, with what happened in its
// environment: KERO_SESSION and KERO_PANE are the pane's, as in the pane
// itself, so that `kero` in the command reaches it.
func hookCommand(ctx context.Context, h settings.StatusHook, ch proto.Change) *exec.Cmd {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", h.Command)
	} else {
		cmd = exec.CommandContext(ctx, "/bin/sh", "-c", h.Command)
	}
	cmd.Env = append(os.Environ(),
		"KERO_EVENT="+ch.Event,
		"KERO_SESSION="+ch.Session,
		"KERO_PANE="+ch.Pane,
		"KERO_SESSION_NAME="+ch.Name,
		"KERO_STATUS="+string(ch.Status),
		"KERO_PREVIOUS="+string(ch.Previous),
		"KERO_MESSAGE="+ch.Message,
		"KERO_AGENT="+ch.Agent,
		"KERO_DIR="+ch.Dir,
	)
	if ch.Dir != "" {
		if st, err := os.Stat(ch.Dir); err == nil && st.IsDir() {
			cmd.Dir = ch.Dir
		}
	}
	return cmd
}
