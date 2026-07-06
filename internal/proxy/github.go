package proxy

import (
	"bytes"
	"errors"
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

var githubUpstreamHosts = map[string]struct{}{
	"github.com":                        {},
	"api.github.com":                    {},
	"gist.github.com":                   {},
	"github.githubassets.com":           {},
	"avatars.githubusercontent.com":     {},
	"raw.githubusercontent.com":         {},
	"objects.githubusercontent.com":     {},
	"user-images.githubusercontent.com": {},
	"camo.githubusercontent.com":        {},
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

	target, err := p.githubTarget(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

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
	if r.Method == http.MethodHead || !p.shouldRewriteBody(contentType, target.Host) {
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
	body = p.rewriteBody(body, r, contentType, target.Host)
	w.Header().Del("Content-Length")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(body)
}

func (p *GitHubProxy) githubTarget(r *http.Request) (*url.URL, error) {
	path := r.URL.EscapedPath()
	host := "github.com"

	if strings.HasPrefix(path, "/_hubproxy/") {
		trimmed := strings.TrimPrefix(path, "/_hubproxy/")
		hostPart, pathPart, ok := strings.Cut(trimmed, "/")
		if !ok {
			pathPart = ""
		}
		if !isAllowedGitHubHost(hostPart) {
			return nil, errors.New("unsupported github upstream host")
		}
		host = hostPart
		path = "/" + pathPart
	} else if path == "/github" || path == "/github/" {
		path = "/"
	} else if strings.HasPrefix(path, "/github/") {
		path = strings.TrimPrefix(path, "/github")
	}
	if path == "" {
		path = "/"
	}
	return &url.URL{
		Scheme:   "https",
		Host:     host,
		Path:     path,
		RawQuery: r.URL.RawQuery,
	}, nil
}

func (p *GitHubProxy) doGitHubRequest(original *http.Request, target *url.URL) (*http.Response, error) {
	req, err := http.NewRequestWithContext(original.Context(), original.Method, target.String(), nil)
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
	req.Header.Set("Host", target.Host)
	req.Host = target.Host
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
	for host := range githubUpstreamHosts {
		if host == "github.com" {
			continue
		}
		proxied := []byte(p.proxiedHostBase(r, host))
		replacements = append(replacements,
			struct {
				old string
				new []byte
			}{old: "https://" + host, new: proxied},
			struct {
				old string
				new []byte
			}{old: "http://" + host, new: proxied},
			struct {
				old string
				new []byte
			}{old: "//" + host, new: proxied},
		)
	}
	for _, replacement := range replacements {
		body = bytes.ReplaceAll(body, []byte(replacement.old), replacement.new)
	}
	return body
}

func (p *GitHubProxy) shouldRewriteBody(contentType, host string) bool {
	return strings.Contains(contentType, "text/html") ||
		strings.Contains(contentType, "text/css") ||
		(strings.Contains(contentType, "javascript") && host == "github.githubassets.com")
}

func (p *GitHubProxy) rewriteBody(body []byte, r *http.Request, contentType, host string) []byte {
	if strings.Contains(contentType, "text/html") {
		return p.rewriteHTML(body, r)
	}
	if host == "github.githubassets.com" {
		proxiedAssets := []byte(p.proxiedHostBase(r, host) + "/assets/")
		body = bytes.ReplaceAll(body, []byte(`"/assets/`), append([]byte(`"`), proxiedAssets...))
		body = bytes.ReplaceAll(body, []byte(`'/assets/`), append([]byte(`'`), proxiedAssets...))
		body = bytes.ReplaceAll(body, []byte(`(/assets/`), append([]byte(`(`), proxiedAssets...))
	}
	return body
}

func (p *GitHubProxy) rewriteURL(value string, r *http.Request) string {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return value
	}
	if !isAllowedGitHubHost(parsed.Host) {
		return value
	}
	if parsed.Host == "github.com" {
		parsed.Scheme = scheme(r)
		parsed.Host = r.Host
		return parsed.String()
	}
	return p.proxiedHostBase(r, parsed.Host) + parsed.RequestURI()
}

func (p *GitHubProxy) baseURL(r *http.Request) string {
	if p.publicBaseURL != "" {
		return p.publicBaseURL
	}
	return scheme(r) + "://" + r.Host
}

func (p *GitHubProxy) proxiedHostBase(r *http.Request, host string) string {
	return p.baseURL(r) + "/_hubproxy/" + host
}

func isAllowedGitHubHost(host string) bool {
	_, ok := githubUpstreamHosts[strings.ToLower(host)]
	return ok
}
