package actions

import "testing"

// No two actions share a default shortcut, on a Mac or elsewhere: with one
// key for two, the menu takes one and the other's never runs.
func TestDefaultKeysAreOwnedOnce(t *testing.T) {
	check := func(where string, keys func(Action) string) {
		owner := map[string]string{}
		for _, a := range All {
			k := keys(a)
			if k == "" {
				continue
			}
			if other, ok := owner[k]; ok {
				t.Errorf("%s: %s and %s are both %s", where, other, a.ID, k)
			}
			owner[k] = a.ID
		}
	}
	check("as built", func(a Action) string { return a.Keys })
	check("Windows and Linux", func(a Action) string {
		if k, ok := pcKeys[a.ID]; ok {
			return k
		}
		return a.Keys
	})
}
