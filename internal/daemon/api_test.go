package daemon

import (
	"os"
	"regexp"
	"slices"
	"testing"

	"kero/internal/proto"
)

// Every method call takes is listed, and every one listed is taken: the
// api method is how a client learns what there is.
func TestMethodsAreAllListed(t *testing.T) {
	src, err := os.ReadFile("rpc.go")
	if err != nil {
		t.Fatal(err)
	}
	var taken []string
	for _, m := range regexp.MustCompile(`(?m)^\tcase ((?:"[^"]+"(?:, )?)+):`).FindAllStringSubmatch(string(src), -1) {
		for _, n := range regexp.MustCompile(`"([^"]+)"`).FindAllStringSubmatch(m[1], -1) {
			taken = append(taken, n[1])
		}
	}
	var listed []string
	for _, m := range methods {
		if m.summary == "" {
			t.Errorf("method %s has no summary", m.name)
		}
		listed = append(listed, m.name)
	}
	for _, n := range taken {
		if !slices.Contains(listed, n) {
			t.Errorf("rpc.go takes %q, which api does not list", n)
		}
	}
	for _, n := range listed {
		if !slices.Contains(taken, n) {
			t.Errorf("api lists %q, which rpc.go does not take", n)
		}
	}
	if len(taken) < 20 {
		t.Errorf("found only %d methods in rpc.go: %q", len(taken), taken)
	}
}

func TestAPIFields(t *testing.T) {
	var create proto.APIMethod
	for _, m := range api() {
		if m.Name == "create" {
			create = m
		}
	}
	if create.Result != "Session" || len(create.ResultFields) == 0 {
		t.Errorf("create answers %q with fields %v", create.Result, create.ResultFields)
	}
	want := map[string]proto.APIField{
		"split":   {Name: "split", Type: "string", Optional: true},
		"ratio":   {Name: "ratio", Type: "number", Optional: true},
		"command": {Name: "command", Type: "[]string", Optional: true},
	}
	got := map[string]proto.APIField{}
	for _, f := range create.Params {
		got[f.Name] = f
	}
	for name, f := range want {
		if got[name] != f {
			t.Errorf("create's %s is %+v, want %+v", name, got[name], f)
		}
	}
	// A struct embedded in the answer, as the pane in a session, has its
	// fields listed as JSON has them, among the struct's.
	names := map[string]bool{}
	for _, f := range create.ResultFields {
		names[f.Name] = true
	}
	for _, name := range []string{"id", "panes", "status", "Pane"} {
		if names[name] != (name != "Pane") {
			t.Errorf("a session's fields have %q: %v", name, names[name])
		}
	}
}
