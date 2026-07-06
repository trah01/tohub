package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	dockerRegistryURL = "https://registry-1.docker.io"
	dockerAuthURL     = "https://auth.docker.io/token"
)

type DockerHubProxy struct {
	client *http.Client
	tokens sync.Map
}

type cachedToken struct {
	token     string
	expiresAt time.Time
}

func NewDockerHubProxy() *DockerHubProxy {
	return &DockerHubProxy{
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

func (p *DockerHubProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/v2/" || r.URL.Path == "/v2" {
		w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "docker mirror only supports pull requests", http.StatusMethodNotAllowed)
		return
	}

	repository, err := repositoryFromDockerPath(r.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := p.proxyDockerRequest(w, r, repository); err != nil {
		log.Printf("docker proxy error: %v", err)
		http.Error(w, "docker upstream request failed", http.StatusBadGateway)
	}
}

func (p *DockerHubProxy) proxyDockerRequest(w http.ResponseWriter, r *http.Request, repository string) error {
	target, err := url.Parse(dockerRegistryURL + r.URL.RequestURI())
	if err != nil {
		return err
	}

	resp, err := p.doDockerRequest(r.Context(), r, target, "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		_, _ = io.Copy(io.Discard, resp.Body)
		token, err := p.token(r.Context(), repository)
		if err != nil {
			return err
		}
		resp, err = p.doDockerRequest(r.Context(), r, target, token)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
	}

	copyHeaders(w.Header(), resp.Header)
	w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
	w.WriteHeader(resp.StatusCode)
	if r.Method != http.MethodHead {
		_, err = io.Copy(w, resp.Body)
	}
	return err
}

func (p *DockerHubProxy) doDockerRequest(ctx context.Context, original *http.Request, target *url.URL, token string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, original.Method, target.String(), nil)
	if err != nil {
		return nil, err
	}
	copySelectedDockerHeaders(req.Header, original.Header)
	req.Host = "registry-1.docker.io"
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	setForwardHeaders(req, original)
	return p.client.Do(req)
}

func (p *DockerHubProxy) token(ctx context.Context, repository string) (string, error) {
	scope := "repository:" + repository + ":pull"
	if value, ok := p.tokens.Load(scope); ok {
		token := value.(cachedToken)
		if time.Now().Before(token.expiresAt) {
			return token.token, nil
		}
	}

	query := url.Values{}
	query.Set("service", "registry.docker.io")
	query.Set("scope", scope)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dockerAuthURL+"?"+query.Encode(), nil)
	if err != nil {
		return "", err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("docker token status %d", resp.StatusCode)
	}

	var payload struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}

	token := payload.Token
	if token == "" {
		token = payload.AccessToken
	}
	if token == "" {
		return "", errors.New("docker token response is empty")
	}

	expiresIn := payload.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 300
	}
	p.tokens.Store(scope, cachedToken{
		token:     token,
		expiresAt: time.Now().Add(time.Duration(expiresIn-30) * time.Second),
	})
	return token, nil
}

func copySelectedDockerHeaders(dst, src http.Header) {
	for _, key := range []string{"Accept", "Accept-Encoding", "Range", "If-None-Match", "If-Modified-Since", "User-Agent"} {
		if values, ok := src[key]; ok {
			for _, value := range values {
				dst.Add(key, value)
			}
		}
	}
}

func repositoryFromDockerPath(path string) (string, error) {
	trimmed := strings.TrimPrefix(path, "/v2/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 3 {
		return "", errors.New("invalid docker registry path")
	}
	for i, part := range parts {
		if part == "manifests" || part == "blobs" || part == "tags" {
			if i == 0 {
				return "", errors.New("invalid docker repository path")
			}
			return strings.Join(parts[:i], "/"), nil
		}
	}
	return "", errors.New("unsupported docker registry path")
}
