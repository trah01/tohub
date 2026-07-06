package proxy

import "net/http"

var hopByHopHeaders = map[string]struct{}{
	"Connection":          {},
	"Keep-Alive":          {},
	"Proxy-Authenticate":  {},
	"Proxy-Authorization": {},
	"Te":                  {},
	"Trailer":             {},
	"Transfer-Encoding":   {},
	"Upgrade":             {},
}

func copyHeaders(dst, src http.Header) {
	for key, values := range src {
		if _, skip := hopByHopHeaders[key]; skip {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func setForwardHeaders(req *http.Request, original *http.Request) {
	req.Header.Set("X-Forwarded-Host", original.Host)
	req.Header.Set("X-Forwarded-Proto", scheme(original))
	if original.RemoteAddr != "" {
		req.Header.Set("X-Forwarded-For", original.RemoteAddr)
	}
}

func scheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	if forwarded := r.Header.Get("X-Forwarded-Proto"); forwarded != "" {
		return forwarded
	}
	return "http"
}
