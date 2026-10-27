package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"kero/internal/client"
	"kero/internal/proto"
)

// apiCmd lists the control methods kerod takes, describes one, or calls it
// with JSON params: the same calls the app and kero itself make.
func apiCmd(c *client.Client, args []string) error {
	f, err := parse(args, nil, map[string]string{"json": "json"})
	if err != nil {
		return err
	}
	var methods []proto.APIMethod
	if err := c.Call("api", nil, &methods); err != nil {
		if strings.Contains(err.Error(), "unknown method") {
			return fmt.Errorf("the running kerod predates kero api: run `kero daemon restart` (it restarts sessions)")
		}
		return err
	}
	if len(f.pos) == 0 {
		if f.set["json"] {
			return printJSON(methods)
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "METHOD\tWHAT")
		for _, m := range methods {
			fmt.Fprintf(w, "%s\t%s\n", m.Name, m.Summary)
		}
		return w.Flush()
	}
	var m *proto.APIMethod
	for i := range methods {
		if methods[i].Name == f.pos[0] {
			m = &methods[i]
		}
	}
	if m == nil {
		return fmt.Errorf("%w: no method %q (kero api lists them)", errNotFound, f.pos[0])
	}
	if len(f.pos) == 1 {
		if f.set["json"] {
			return printJSON(m)
		}
		describe(os.Stdout, *m)
		return nil
	}
	if m.Name == "subscribe" || m.Name == "watch" {
		return usageErr("%s streams events: use kero events", m.Name)
	}
	params, err := apiParams(f.pos[1], os.Stdin)
	if err != nil {
		return err
	}
	var out json.RawMessage
	if err := c.Call(m.Name, params, &out); err != nil {
		return err
	}
	if len(out) == 0 {
		out = json.RawMessage("true")
	}
	return printJSON(out)
}

// apiParams reads a call's params: JSON given as an argument, or "-" for
// stdin; none is no params.
func apiParams(arg string, stdin io.Reader) (json.RawMessage, error) {
	if arg == "-" {
		b, err := io.ReadAll(stdin)
		if err != nil {
			return nil, err
		}
		arg = string(b)
	}
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return nil, nil
	}
	if !json.Valid([]byte(arg)) {
		return nil, usageErr("the params are not JSON: %s", arg)
	}
	return json.RawMessage(arg), nil
}

// describe writes a method: what it does, what it takes and what it
// answers.
func describe(w io.Writer, m proto.APIMethod) {
	fmt.Fprintf(w, "%s: %s\n", m.Name, m.Summary)
	table := func(title string, fields []proto.APIField) {
		if len(fields) == 0 {
			return
		}
		fmt.Fprintf(w, "\n%s\n", title)
		tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		for _, f := range fields {
			opt := ""
			if f.Optional {
				opt = "optional"
			}
			fmt.Fprintf(tw, "  %s\t%s\t%s\n", f.Name, f.Type, opt)
		}
		tw.Flush()
	}
	table("Params (JSON):", m.Params)
	fmt.Fprintf(w, "\nAnswers: %s\n", m.Result)
	table("", m.ResultFields)
}
