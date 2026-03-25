package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var allowedOrigins []string
var allowPrivateTargets bool

var targetPathSegmentPattern = regexp.MustCompile(`^(?:[A-Za-z0-9._~+\-@]|%[0-9A-Fa-f]{2})+$`)

func init() {
	allowPrivateTargets = strings.EqualFold(strings.TrimSpace(os.Getenv("ALLOW_PRIVATE_TARGETS")), "true")
}

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
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Git-Protocol, X-Requested-With, Origin, Accept")
	w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Type, Date, ETag, Location, Server, X-Redirected-URL")
	w.Header().Set("Vary", "Origin")

	targetHost, remaining, err := parseProxyPath(req.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := validateTargetHost(targetHost); err != nil {
		log.Printf("Rejected %s %s from %s: target host validation failed: %v", req.Method, req.URL.String(), req.RemoteAddr, err)
		http.Error(w, "Forbidden target host", http.StatusForbidden)
		return
	}

	normalizedRemaining, err := validateGitSmartHTTPRequest(req.Method, remaining, req.URL.Query())
	if err != nil {
		log.Printf("Rejected %s %s from %s: git endpoint validation failed: %v", req.Method, req.URL.String(), req.RemoteAddr, err)
		http.Error(w, "Forbidden request path", http.StatusForbidden)
		return
	}

	// Preflight
	if req.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if req.Method != http.MethodGet && req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// --- Parse target URL ---
	targetURL := "https://" + targetHost + "/" + normalizedRemaining
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
			if shouldSkipResponseHeader(k) {
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

func parseProxyPath(path string) (string, string, error) {
	pathParts := strings.SplitN(strings.TrimPrefix(path, "/"), "/", 2)
	if len(pathParts) < 2 {
		return "", "", fmt.Errorf("invalid path")
	}

	targetHost := strings.TrimSpace(pathParts[0])
	remaining := strings.TrimSpace(pathParts[1])
	if targetHost == "" || remaining == "" {
		return "", "", fmt.Errorf("invalid path")
	}

	return targetHost, remaining, nil
}

func validateTargetHost(targetHost string) error {
	host := targetHost
	if strings.Contains(targetHost, ":") {
		parsedHost, port, err := net.SplitHostPort(targetHost)
		if err != nil {
			return fmt.Errorf("invalid target host/port")
		}
		if _, err := strconv.Atoi(port); err != nil {
			return fmt.Errorf("invalid target port")
		}
		host = parsedHost
	}

	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return fmt.Errorf("missing target host")
	}
	if strings.HasPrefix(host, ".") || strings.HasSuffix(host, ".") || strings.Contains(host, "..") {
		return fmt.Errorf("invalid target host")
	}
	if strings.ContainsAny(host, "/\\?&#%") {
		return fmt.Errorf("invalid target host")
	}

	if ip := net.ParseIP(host); ip != nil {
		if allowPrivateTargets {
			return nil
		}
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
			return fmt.Errorf("private/internal IP targets are not allowed")
		}
		return nil
	}

	if !strings.Contains(host, ".") {
		return fmt.Errorf("target host must be a fully-qualified domain")
	}

	blockedSuffixes := []string{".local", ".localhost", ".internal", ".lan", ".home"}
	for _, suffix := range blockedSuffixes {
		if strings.HasSuffix(host, suffix) {
			if allowPrivateTargets {
				break
			}
			return fmt.Errorf("private/internal target host is not allowed")
		}
	}

	labels := strings.Split(host, ".")
	if len(labels) < 2 {
		return fmt.Errorf("target host must contain a TLD")
	}
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 {
			return fmt.Errorf("invalid host label")
		}
		if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return fmt.Errorf("invalid host label")
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z') && !(c >= '0' && c <= '9') && c != '-' {
				return fmt.Errorf("invalid host label")
			}
		}
	}

	return nil
}

func validateGitSmartHTTPRequest(method, remaining string, query url.Values) (string, error) {
	if method != http.MethodGet && method != http.MethodPost && method != http.MethodOptions {
		return "", fmt.Errorf("method not allowed")
	}

	if strings.Contains(remaining, "\\") {
		return "", fmt.Errorf("invalid path")
	}

	repoPath, endpoint, err := splitGitSmartHTTPRequestPath(remaining)
	if err != nil {
		return "", err
	}

	normalizedRepoPath, err := normalizeGitRepoPath(repoPath)
	if err != nil {
		return "", err
	}

	switch endpoint {
	case "info/refs":
		service := query.Get("service")
		if service != "git-upload-pack" && service != "git-receive-pack" {
			return "", fmt.Errorf("invalid info/refs service")
		}
		if method != http.MethodGet && method != http.MethodOptions {
			return "", fmt.Errorf("info/refs only supports GET")
		}
	case "git-upload-pack", "git-receive-pack":
		if method != http.MethodPost && method != http.MethodOptions {
			return "", fmt.Errorf("pack endpoints only support POST")
		}
	default:
		return "", fmt.Errorf("unsupported git endpoint")
	}

	return normalizedRepoPath + "/" + endpoint, nil
}

func splitGitSmartHTTPRequestPath(remaining string) (string, string, error) {
	for _, endpoint := range []string{"info/refs", "git-upload-pack", "git-receive-pack"} {
		suffix := "/" + endpoint
		if strings.HasSuffix(remaining, suffix) {
			repoPath := strings.TrimSuffix(remaining, suffix)
			if repoPath == "" {
				return "", "", fmt.Errorf("missing repository path")
			}
			return repoPath, endpoint, nil
		}
	}

	return "", "", fmt.Errorf("path is not a git smart-http endpoint")
}

func normalizeGitRepoPath(repoPath string) (string, error) {
	trimmed := strings.Trim(strings.TrimSpace(repoPath), "/")
	if trimmed == "" {
		return "", fmt.Errorf("missing repository path")
	}

	segments := strings.Split(trimmed, "/")
	if len(segments) < 2 {
		return "", fmt.Errorf("repository path must include owner and repo")
	}

	for _, segment := range segments {
		if segment == "" || segment == "." || segment == ".." {
			return "", fmt.Errorf("invalid path")
		}
		if !targetPathSegmentPattern.MatchString(segment) {
			return "", fmt.Errorf("invalid repository path segment")
		}
	}

	return strings.Join(segments, "/"), nil
}

func shouldSkipResponseHeader(name string) bool {
	if strings.EqualFold(name, "Content-Length") {
		return true
	}
	if strings.HasPrefix(strings.ToLower(name), "access-control-") {
		return true
	}
	return false
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
