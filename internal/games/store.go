package games

import (
	"encoding/json"
	"os"
	"path/filepath"

	"kero/internal/proto"
)

// What a game keeps between plays, as a game's progress, is a file of JSON of its
// own in Kero's data, under games/. It is this machine's: it is not shared with
// another, nor with the sessions of a remote host.

func path(name string) string { return filepath.Join(proto.Dir(), "games", name+".json") }

// Load reads what a game kept into v; v is left as it was if the game kept
// nothing, or what it kept is not readable.
func Load(name string, v any) error {
	b, err := os.ReadFile(path(name))
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

// Save keeps v for a game, whole or not at all: it is written beside the file and
// moved over it.
func Save(name string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	p := path(name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}
