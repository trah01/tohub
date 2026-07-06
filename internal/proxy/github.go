package proxy

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type GitHubProxy struct {
	client        *http.Client
	publicBaseURL string
}

func NewGitHubProxy(publicBaseURL string) *GitHubProxy {
	return &GitHubProxy{
		publicBaseURL: strings.TrimRight(publicBaseURL, "/"),
		client: &http.Client{
			Timeout: 0,
			Transport: &http.Transport{
				Proxy:                 http.ProxyFromEnvironment,
				MaxIdleConns:          100,
				MaxIdleConnsPerHost:   20,
				IdleConnTimeout:       90 * time.Second,
				ResponseHeaderTimeout: 30 * time.Second,
			},
		},
	}
}

func (p *GitHubProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "github proxy only supports read requests", http.StatusMethodNotAllowed)
		return
	}

	target := p.githubTarget(r)
	resp, err := p.doGitHubRequest(r, target)
	if err != nil {
		log.Printf("github proxy error: %v", err)
		http.Error(w, "github upstream request failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	copyHeaders(w.Header(), resp.Header)
	p.rewriteLocation(w.Header(), r)
	w.Header().Del("Content-Security-Policy")
	w.Header().Del("Content-Security-Policy-Report-Only")

	contentType := resp.Header.Get("Content-Type")
	if r.Method == http.MethodHead || !strings.Contains(contentType, "text/html") {
		w.WriteHeader(resp.StatusCode)
		if r.Method != http.MethodHead {
			_, _ = io.Copy(w, resp.Body)
		}
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "github upstream body read failed", http.StatusBadGateway)
		return
	}
	body = p.rewriteHTML(body, r)
	w.Header().Del("Content-Length")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(body)
}

func (p *GitHubProxy) githubTarget(r *http.Request) string {
	path := r.URL.EscapedPath()
	if path == "/github" || path == "/github/" {
		path = "/"
	} else if strings.HasPrefix(path, "/github/") {
		path = strings.TrimPrefix(path, "/github")
	}
	if path == "" {
		path = "/"
	}
	target := url.URL{
		Scheme:   "https",
		Host:     "github.com",
		Path:     path,
		RawQuery: r.URL.RawQuery,
	}
	return target.String()
}

func (p *GitHubProxy) doGitHubRequest(original *http.Request, target string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(original.Context(), original.Method, target, nil)
	if err != nil {
		return nil, err
	}
	for _, key := range []string{"Accept", "If-None-Match", "If-Modified-Since", "Range", "User-Agent"} {
		if values, ok := original.Header[key]; ok {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}
	}
	req.Header.Set("Host", "github.com")
	req.Host = "github.com"
	setForwardHeaders(req, original)
	return p.client.Do(req)
}

func (p *GitHubProxy) rewriteLocation(header http.Header, r *http.Request) {
	location := header.Get("Location")
	if location == "" {
		return
	}
	header.Set("Location", p.rewriteURL(location, r))
}

func (p *GitHubProxy) rewriteHTML(body []byte, r *http.Request) []byte {
	base := []byte(p.baseURL(r))
	replacements := []struct {
		old string
		new []byte
	}{
		{old: `https://github.com`, new: base},
		{old: `http://github.com`, new: base},
		{old: `//github.com`, new: append([]byte("//"), bytes.TrimPrefix(base, []byte("https://"))...)},
	}
	for _, replacement := range replacements {
		body = bytes.ReplaceAll(body, []byte(replacement.old), replacement.new)
	}
	return body
}

func (p *GitHubProxy) rewriteURL(value string, r *http.Request) string {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return value
	}
	if parsed.Host == "github.com" {
		parsed.Scheme = scheme(r)
		parsed.Host = r.Host
		return parsed.String()
	}
	return value
}

func (p *GitHubProxy) baseURL(r *http.Request) string {
	if p.publicBaseURL != "" {
		return p.publicBaseURL
	}
	return scheme(r) + "://" + r.Host
}
