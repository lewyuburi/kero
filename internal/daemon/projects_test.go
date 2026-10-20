package daemon

import (
	"os"
	"path/filepath"
	"testing"

	"kero/internal/proto"
)

// A session's project: with loose-sessions disabled every session has one,
// made for its folder where it has none; enabled, a folder in no project and
// no git repository is in none.
func TestProjectFor(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	plain := filepath.Join(root, "plain")
	for _, dir := range []string{filepath.Join(repo, ".git"), filepath.Join(repo, "src"), plain} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	config := filepath.Join(t.TempDir(), "config")
	t.Setenv("KERO_CONFIG", config)

	t.Run("disabled", func(t *testing.T) {
		os.Remove(config)
		d := &Daemon{}
		if id := d.projectFor("", plain); id == "" {
			t.Error("a session in a plain folder has no project with the flag disabled")
		}
		if id := d.projectFor("", ""); id == "" {
			t.Error("a session of no folder has no project with the flag disabled")
		}
	})

	t.Run("enabled", func(t *testing.T) {
		if err := os.WriteFile(config, []byte("flag = loose-sessions=enabled\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		d := &Daemon{}
		if id := d.projectFor("", plain); id != "" || len(d.projects) != 0 {
			t.Errorf("a plain folder is in project %q, %d made", id, len(d.projects))
		}
		if id := d.projectFor("", ""); id != "" {
			t.Errorf("no folder is in project %q", id)
		}
		inRepo := d.projectFor("", filepath.Join(repo, "src"))
		if inRepo == "" || len(d.projects) != 1 || d.projects[0].Dir != repo {
			t.Fatalf("a repository's folder: project %q, projects %+v", inRepo, d.projects)
		}
		if again := d.projectFor("", repo); again != inRepo || len(d.projects) != 1 {
			t.Errorf("the repository's own folder is in %q, not its project %q", again, inRepo)
		}
		if named := d.projectFor("work", ""); named == "" || named == inRepo {
			t.Errorf("a project named is %q", named)
		}
		// Within a project's folder, even a plain one, is in it.
		d.projects = append(d.projects, proto.Project{ID: "held", Name: "held", Dir: plain})
		if id := d.projectFor("", filepath.Join(plain, "deep")); id != "held" {
			t.Errorf("a folder in a project's is in %q, not held", id)
		}
	})
}
