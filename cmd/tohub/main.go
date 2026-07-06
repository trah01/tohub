package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"tohub/internal/proxy"
)

func main() {
	addr := env("ADDR", ":8080")
	publicBaseURL := strings.TrimRight(env("PUBLIC_BASE_URL", ""), "/")

	dockerProxy := proxy.NewDockerHubProxy()
	githubProxy := proxy.NewGitHubProxy(publicBaseURL)
	assets := http.FileServer(http.Dir("web"))

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("/v2/", dockerProxy.ServeHTTP)
	mux.HandleFunc("/_tohub/", githubProxy.ServeHTTP)
	mux.HandleFunc("/_hubproxy/", githubProxy.ServeHTTP)
	mux.HandleFunc("/github", githubProxy.ServeHTTP)
	mux.HandleFunc("/github/", githubProxy.ServeHTTP)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || strings.HasPrefix(r.URL.Path, "/assets/") {
			assets.ServeHTTP(w, r)
			return
		}
		githubProxy.ServeHTTP(w, r)
	})

	server := &http.Server{
		Addr:              addr,
		Handler:           securityHeaders(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("tohub listening on %s", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func env(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}
