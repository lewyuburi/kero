package proto

// Event names of a Change.
const (
	ChangeSessionNew    = "session.new"
	ChangeSessionClosed = "session.closed"
	ChangeStatus        = "status"
)

// Change is one thing that happened between two states of the daemon, as
// `kero watch` prints it and the on-status hooks run on it.
type Change struct {
	Event   string `json:"event"`
	Session string `json:"session"`
	Pane    string `json:"pane,omitempty"`
	// Name is the session's name, when it has one; Dir the pane's directory.
	Name string `json:"name,omitempty"`
	Dir  string `json:"dir,omitempty"`
	// Status is what a pane is doing now, Previous what it did before; Message
	// is what the program reported or notified, as the sidebar shows it.
	Status   Status `json:"status,omitempty"`
	Previous Status `json:"previous,omitempty"`
	Message  string `json:"message,omitempty"`
	Agent    string `json:"agent,omitempty"`
}

// Changes lists what is new in a state since another, in the order of the
// new one's sessions: a session that came or went, and a pane whose status
// changed. Panes that are new, or gone with their session, change nothing
// by themselves.
func Changes(old, cur State) []Change {
	before := map[string]*Session{}
	for i := range old.Sessions {
		before[old.Sessions[i].ID] = &old.Sessions[i]
	}
	var out []Change
	seen := map[string]bool{}
	for i := range cur.Sessions {
		s := &cur.Sessions[i]
		seen[s.ID] = true
		prev := before[s.ID]
		if prev == nil {
			out = append(out, Change{Event: ChangeSessionNew, Session: s.ID, Name: s.Name, Dir: s.Dir})
			continue
		}
		for j := range s.Panes {
			p := &s.Panes[j]
			was := prev.PaneByID(p.ID)
			if was == nil || was.Status == p.Status {
				continue
			}
			out = append(out, Change{
				Event: ChangeStatus, Session: s.ID, Pane: p.ID, Name: s.Name, Dir: p.Dir,
				Status: p.Status, Previous: was.Status, Message: p.Message, Agent: p.Agent,
			})
		}
	}
	for i := range old.Sessions {
		if s := &old.Sessions[i]; !seen[s.ID] {
			out = append(out, Change{Event: ChangeSessionClosed, Session: s.ID, Name: s.Name, Dir: s.Dir})
		}
	}
	return out
}
