package games

import (
	"os"
	"path/filepath"
	"testing"

	"kero/internal/proto"
)

func TestAGameKeepsWhatItSaves(t *testing.T) {
	t.Setenv("KERO_HOME", t.TempDir())
	type progress struct {
		Chips map[string]int `json:"chips"`
	}
	var got progress
	if err := Load("g", &got); err == nil {
		t.Error("a game that kept nothing loaded something")
	}
	want := progress{Chips: map[string]int{"1-1": 3, "1-2": 1}}
	if err := Save("g", want); err != nil {
		t.Fatal(err)
	}
	if err := Load("g", &got); err != nil || got.Chips["1-1"] != 3 || got.Chips["1-2"] != 1 || len(got.Chips) != 2 {
		t.Errorf("loaded %+v, %v", got, err)
	}
	// It is in the data, under games, and no temporary file is left.
	if _, err := os.Stat(filepath.Join(proto.Dir(), "games", "g.json")); err != nil {
		t.Errorf("no file where it should be: %v", err)
	}
	if _, err := os.Stat(filepath.Join(proto.Dir(), "games", "g.json.tmp")); err == nil {
		t.Error("a temporary file was left")
	}
	// Another game's is another file; saving again replaces.
	if err := Save("h", progress{Chips: map[string]int{"x": 1}}); err != nil {
		t.Fatal(err)
	}
	if err := Save("g", progress{Chips: map[string]int{"1-1": 2}}); err != nil {
		t.Fatal(err)
	}
	var again, other progress
	Load("g", &again)
	Load("h", &other)
	if again.Chips["1-1"] != 2 || len(again.Chips) != 1 || other.Chips["x"] != 1 {
		t.Errorf("g %+v, h %+v", again, other)
	}
	// What is not readable is left alone.
	os.WriteFile(filepath.Join(proto.Dir(), "games", "bad.json"), []byte("{not json"), 0o644)
	keep := progress{Chips: map[string]int{"keep": 1}}
	if err := Load("bad", &keep); err == nil || keep.Chips["keep"] != 1 {
		t.Errorf("an unreadable file changed what was there: %+v, %v", keep, err)
	}
}
