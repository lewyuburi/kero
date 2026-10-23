package proto

import "testing"

func TestBrowserURL(t *testing.T) {
	for in, want := range map[string]string{
		"":                        BlankURL,
		"  https://kero.sh/docs ": "https://kero.sh/docs",
		"about:blank":             "about:blank",
		"file:///tmp/a.html":      "file:///tmp/a.html",
		"localhost:3000":          "http://localhost:3000",
		"localhost:5173/app?x=1":  "http://localhost:5173/app?x=1",
		"localhost":               "http://localhost",
		"127.0.0.1:8080":          "http://127.0.0.1:8080",
		"[::1]:8080":              "http://[::1]:8080",
		"app.localhost:3000":      "http://app.localhost:3000",
		"192.168.1.20:8000":       "http://192.168.1.20:8000",
		"kero.sh":                 "https://kero.sh",
		"github.com/egoist/kero":  "https://github.com/egoist/kero",
		"how to quit vim":         "https://www.google.com/search?q=how+to+quit+vim",
		"golang":                  "https://www.google.com/search?q=golang",
	} {
		if got := BrowserURL(in); got != want {
			t.Errorf("BrowserURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBrowserPaneName(t *testing.T) {
	p := Pane{URL: "https://www.example.com/a"}
	if got := p.PaneName(); got != "example.com" {
		t.Errorf("untitled page named %q", got)
	}
	p.Title = "Example Domain"
	if got := p.PaneName(); got != "Example Domain" {
		t.Errorf("titled page named %q", got)
	}
}

func TestIsAddress(t *testing.T) {
	for in, want := range map[string]bool{
		"localhost:3000": true, " kero.sh ": true, "https://x": true, "about:blank": true,
		"": false, "split right": false, "settings": false,
	} {
		if got := IsAddress(in); got != want {
			t.Errorf("IsAddress(%q) = %v", in, got)
		}
	}
}

// A web page, or the blank one, is what a program that drives a page may
// open: not a file, a script or another kind of address.
func TestIsWebAddress(t *testing.T) {
	for text, want := range map[string]bool{
		"localhost:3000":         true,
		"http://localhost:3000/": true,
		"kero.sh":                true,
		"https://kero.sh/docs":   true,
		"about:blank":            true,
		"file:///etc/passwd":     false,
		"FILE:///etc/passwd":     false,
		"javascript:alert(1)":    false,
		"data:text/html,<b>x":    false,
		"ftp://example.com":      false,
		"http://":                false,
	} {
		if got := IsWebAddress(text); got != want {
			t.Errorf("IsWebAddress(%q) = %v, want %v", text, got, want)
		}
	}
}
