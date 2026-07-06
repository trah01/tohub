package proxy

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBaseURLUsesRequestHostForLoopback(t *testing.T) {
	proxy := NewGitHubProxy("http://localhost:8080")
	request := httptest.NewRequest("GET", "http://127.0.0.1:8080/trah01/Accnotify/releases", nil)

	got := proxy.baseURL(request)
	want := "http://127.0.0.1:8080"
	if got != want {
		t.Fatalf("baseURL() = %q, want %q", got, want)
	}
}

func TestBaseURLKeepsConfiguredPublicHostForNonLoopback(t *testing.T) {
	proxy := NewGitHubProxy("https://proxy.example.com")
	request := httptest.NewRequest("GET", "http://127.0.0.1:8080/trah01/Accnotify/releases", nil)

	got := proxy.baseURL(request)
	want := "https://proxy.example.com"
	if got != want {
		t.Fatalf("baseURL() = %q, want %q", got, want)
	}
}

func TestRewriteHTMLKeepsLoopbackAssetsSameOrigin(t *testing.T) {
	proxy := NewGitHubProxy("http://localhost:8080")
	request := httptest.NewRequest("GET", "http://127.0.0.1:8080/trah01/Accnotify/releases", nil)
	body := []byte(`<include-fragment src="https://github.com/trah01/Accnotify/releases/expanded_assets/v0.0.1"></include-fragment><link href="https://github.githubassets.com/assets/releases.css">`)

	got := string(proxy.rewriteHTML(body, request))
	for _, want := range []string{
		`src="http://127.0.0.1:8080/trah01/Accnotify/releases/expanded_assets/v0.0.1"`,
		`href="http://127.0.0.1:8080/_tohub/github.githubassets.com/assets/releases.css"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("rewritten HTML missing %q in %q", want, got)
		}
	}
}
