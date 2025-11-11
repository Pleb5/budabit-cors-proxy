package main

import (
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
)

var allowedOrigins []string

func main() {
	env := os.Getenv("ALLOWED_ORIGINS")
	if env == "" {
		log.Fatal("ALLOWED_ORIGINS not set")
	}
	for _, o := range strings.Split(env, ",") {
		allowedOrigins = append(allowedOrigins, strings.TrimSpace(o))
	}

	http.HandleFunc("/", handleRequestAndRedirect)

	port := "8080"
	log.Println("CORS proxy running on port " + port)
	log.Printf("Allowed origins: %v\n", allowedOrigins)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func handleRequestAndRedirect(w http.ResponseWriter, req *http.Request) {
	origin := req.Header.Get("Origin")

	// --- CORS headers ---
	if allowCorsForOrigin(origin) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	} else if isAllowedOrigin("*") {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	}
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Git-Protocol, X-Requested-With, Origin, Accept")
	w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Type, Date, ETag, Location, Server, X-Redirected-URL")

	// Preflight
	if req.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// --- Parse target URL ---
	pathParts := strings.SplitN(strings.TrimPrefix(req.URL.Path, "/"), "/", 2)
	if len(pathParts) < 2 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	domain := pathParts[0]
	remaining := pathParts[1]
	targetURL := "https://" + domain + "/" + remaining
	if req.URL.RawQuery != "" {
		targetURL += "?" + req.URL.RawQuery
	}

	log.Printf("Proxying %s %s -> %s\n", req.Method, req.URL.String(), targetURL)

	// --- Create proxy request ---
	proxyReq, err := http.NewRequest(req.Method, targetURL, req.Body)
	if err != nil {
		http.Error(w, "Failed to create proxy request: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// --- Copy headers directly from the original request ---
	for name, values := range req.Header {
		for _, value := range values {
			// Preserve Authorization (Bearer tokens), Content-Type, etc.
			proxyReq.Header.Add(name, value)
		}
	}

	// Set a consistent User-Agent if missing
	if ua := proxyReq.Header.Get("User-Agent"); ua == "" {
		proxyReq.Header.Set("User-Agent", "git/budabit-go-cors-proxy")
	}

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Avoid redirect loops
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Do(proxyReq)
	if err != nil {
		http.Error(w, "Request to target failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// --- Copy response headers ---
	for k, v := range resp.Header {
		for _, val := range v {
			if strings.EqualFold(k, "Content-Length") {
				continue
			}
			w.Header().Add(k, val)
		}
	}

	if loc := resp.Header.Get("Location"); loc != "" {
		newLoc := rewriteLocation(loc, req)
		w.Header().Set("Location", newLoc)
		w.Header().Set("X-Redirected-URL", newLoc)
	}

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// rewriteLocation converts e.g. https://github.com/... -> /github.com/...
func rewriteLocation(loc string, req *http.Request) string {
	u, err := url.Parse(loc)
	if err != nil || u.Host == "" {
		return loc
	}
	newLoc := "/" + u.Host + u.Path
	if u.RawQuery != "" {
		newLoc += "?" + u.RawQuery
	}
	return newLoc
}

// allow localhost or explicitly allowed origins
func allowCorsForOrigin(o string) bool {
	if o == "" {
		return false
	}
	if strings.Contains(o, "localhost") || strings.Contains(o, "127.0.0.1") {
		return true
	}
	return isAllowedOrigin(o)
}

func isAllowedOrigin(o string) bool {
	for _, a := range allowedOrigins {
		if a == "*" || a == o {
			return true
		}
	}
	return false
}
