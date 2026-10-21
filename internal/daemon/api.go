package daemon

import (
	"reflect"
	"strings"

	"kero/internal/proto"
)

// method describes a control method of call (rpc.go): what it does, and the
// types of its params and of its answer, from which the api method lists
// their fields. A method that takes none, or answers true, has a nil.
type method struct {
	name, summary  string
	params, result any
}

// methods lists every method call takes, which TestMethodsAreAllListed keeps
// to the cases of its switch.
var methods = []method{
	{"state", "everything kerod knows: the projects, and the sessions with their panes", nil, proto.State{}},
	{"subscribe", "send the state, and the app's events, as they happen (an app's: kero watch is for scripts)", nil, nil},
	{"watch", "send what changes, as it happens (kero watch)", nil, nil},
	{"api", "list these methods", nil, []proto.APIMethod{}},
	{"create", "start a session, or with split a pane beside one", proto.CreateParams{}, proto.Session{}},
	{"split", "start a pane beside a pane: create with split", proto.CreateParams{}, proto.Session{}},
	{"close", "end a session", proto.IDParams{}, nil},
	{"pane.close", "end a pane", proto.IDParams{}, nil},
	{"restart", "start an exited pane's shell again", proto.IDParams{}, nil},
	{"rename", "name a session", proto.RenameParams{}, nil},
	{"move", "put a session at a place in a project", proto.MoveParams{}, nil},
	{"focus", "make a pane its session's focused one", proto.FocusParams{}, nil},
	{"zoom", "make a pane fill its session", proto.FocusParams{}, nil},
	{"layout", "arrange a session's panes", proto.LayoutParams{}, nil},
	{"send", "type into a pane", proto.SendParams{}, nil},
	{"read", "a pane's screen, or with scrollback its history", proto.ReadParams{}, proto.ReadResult{}},
	{"view", "tell which session a window shows: it is read", proto.ViewParams{}, nil},
	{"select", "show a session in the app", proto.FocusParams{}, nil},
	{"do", "run an action of the app, if Remote Control is on", proto.DoParams{}, nil},
	{"notify", "flag a pane as needing the user", proto.NotifyParams{}, nil},
	{"wait", "wait for a pane to stop working, to reach a status or to end its command", proto.WaitParams{}, proto.Pane{}},
	{"pane.agentSession", "tell a pane's coding agent's own session (kero hook)", proto.AgentSessionParams{}, proto.AgentSessionResult{}},
	{"project.create", "make a project, or find the one of a directory", proto.ProjectParams{}, proto.Project{}},
	{"project.rename", "name a project", proto.ProjectParams{}, nil},
	{"project.move", "put a project at a place in the list", proto.MoveProjectParams{}, nil},
	{"project.close", "remove a project", proto.IDParams{}, nil},
	{"shutdown", "end every session and stop kerod", nil, nil},
}

// api describes the methods, for the clients that ask.
func api() []proto.APIMethod {
	out := make([]proto.APIMethod, 0, len(methods))
	for _, m := range methods {
		d := proto.APIMethod{Name: m.name, Summary: m.summary, Params: fields(m.params), Result: typeName(m.result)}
		d.ResultFields = fields(m.result)
		out = append(out, d)
	}
	return out
}

// typeName is how an answer reads: true for none, the type's name else.
func typeName(v any) string {
	if v == nil {
		return "true"
	}
	return jsonType(reflect.TypeOf(v))
}

// fields lists the fields of a struct by their JSON names, in order, with
// the types and which may be left out.
func fields(v any) []proto.APIField {
	t := reflect.TypeOf(v)
	if t == nil || t.Kind() != reflect.Struct {
		return nil
	}
	return structFields(t)
}

func structFields(t reflect.Type) []proto.APIField {
	var out []proto.APIField
	for i := range t.NumField() {
		f := t.Field(i)
		name, opts, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "-" || !f.IsExported() {
			continue
		}
		if f.Anonymous && name == "" && f.Type.Kind() == reflect.Struct {
			// An embedded struct's fields are the struct's own in JSON.
			out = append(out, structFields(f.Type)...)
			continue
		}
		if name == "" {
			name = f.Name
		}
		out = append(out, proto.APIField{Name: name, Type: jsonType(f.Type), Optional: strings.Contains(opts, "omitempty")})
	}
	return out
}

// jsonType names a Go type as a JSON client sees it.
func jsonType(t reflect.Type) string {
	switch t.Kind() {
	case reflect.Pointer:
		return jsonType(t.Elem())
	case reflect.String:
		return "string"
	case reflect.Bool:
		return "bool"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "int"
	case reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Slice, reflect.Array:
		return "[]" + jsonType(t.Elem())
	case reflect.Map:
		return "map[" + jsonType(t.Key()) + "]" + jsonType(t.Elem())
	case reflect.Struct:
		return t.Name()
	}
	return t.String()
}
