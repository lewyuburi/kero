package cli

import (
	"bytes"
	"strings"
	"testing"

	"kero/internal/proto"
)

func TestAPIParams(t *testing.T) {
	if got, err := apiParams(`{"id":"a"}`, nil); err != nil || string(got) != `{"id":"a"}` {
		t.Errorf("given: %q, %v", got, err)
	}
	if got, err := apiParams("-", strings.NewReader(" {\"id\": 1}\n")); err != nil || string(got) != `{"id": 1}` {
		t.Errorf("from stdin: %q, %v", got, err)
	}
	if got, err := apiParams("  ", nil); err != nil || got != nil {
		t.Errorf("blank: %q, %v", got, err)
	}
	if _, err := apiParams("{id:a}", nil); err == nil || !strings.Contains(err.Error(), "not JSON") {
		t.Errorf("bad JSON: %v", err)
	}
}

func TestDescribe(t *testing.T) {
	var b bytes.Buffer
	describe(&b, proto.APIMethod{
		Name: "send", Summary: "type into a pane",
		Params: []proto.APIField{{Name: "id", Type: "string"}, {Name: "data", Type: "string"}, {Name: "ratio", Type: "number", Optional: true}},
		Result: "true",
	})
	got := b.String()
	for _, want := range []string{"send: type into a pane", "Params (JSON):", "  id     string", "  ratio  number  optional", "Answers: true"} {
		if !strings.Contains(got, want) {
			t.Errorf("no %q in:\n%s", want, got)
		}
	}
}
