package settings

import (
	"fmt"
	"strings"

	"kero/internal/proto"
)

// StatusHook is a command to run as a pane comes to a status.
type StatusHook struct {
	Status  proto.Status
	Command string
}

// ParseStatusHook reads an on-status line: a status, then the command.
func ParseStatusHook(line string) (StatusHook, error) {
	word, command, _ := strings.Cut(strings.TrimSpace(line), " ")
	command = strings.TrimSpace(command)
	switch st := proto.Status(word); st {
	case proto.Working, proto.Waiting, proto.Idle, proto.Failed, proto.Exited:
		if command == "" {
			return StatusHook{}, fmt.Errorf("on-status %q has no command", line)
		}
		return StatusHook{Status: st, Command: command}, nil
	}
	return StatusHook{}, fmt.Errorf("on-status %q: %q is not a status (working, waiting, idle, error or exited)", line, word)
}

// StatusHooks are the on-status lines, those that read.
func (s Settings) StatusHooks() []StatusHook {
	var out []StatusHook
	for _, line := range s.OnStatus {
		if h, err := ParseStatusHook(line); err == nil {
			out = append(out, h)
		}
	}
	return out
}
