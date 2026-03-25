package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func setAllowedOrigins(t *testing.T, origins []string) {
	t.Helper()
	old := allowedOrigins
	allowedOrigins = append([]string(nil), origins...)
	t.Cleanup(func() {
		allowedOrigins = old
	})
}

func setAllowPrivateTargets(t *testing.T, value bool) {
	t.Helper()
	old := allowPrivateTargets
	allowPrivateTargets = value
	t.Cleanup(func() {
		allowPrivateTargets = old
	})
}

func trustTLSServer(t *testing.T, ts *httptest.Server) {
	t.Helper()
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		t.Fatalf("http.DefaultTransport type %T", http.DefaultTransport)
	}
	clientTransport, ok := ts.Client().Transport.(*http.Transport)
	if !ok {
		t.Fatalf("tls client transport type %T", ts.Client().Transport)
	}
	oldTLS := transport.TLSClientConfig
	transport.TLSClientConfig = clientTransport.TLSClientConfig
	transport.CloseIdleConnections()
	t.Cleanup(func() {
		transport.TLSClientConfig = oldTLS
		transport.CloseIdleConnections()
	})
}

func TestAllowCorsForOrigin(t *testing.T) {
	setAllowedOrigins(t, []string{"https://budabit.club"})

	if !allowCorsForOrigin("https://budabit.club") {
		t.Fatalf("expected allowlist origin to be allowed")
	}
	if allowCorsForOrigin("https://example.com") {
		t.Fatalf("expected non-allowlist origin to be denied")
	}
}

func TestAllowCorsForOriginDevDomain(t *testing.T) {
	setAllowedOrigins(t, []string{"https://budabit.club", "https://dev.budabit.club"})

	if !allowCorsForOrigin("https://dev.budabit.club") {
		t.Fatalf("expected dev origin to be allowed")
	}
}

func TestAllowCorsForOriginLocalhost(t *testing.T) {
	setAllowedOrigins(t, []string{"https://budabit.club"})

	if !allowCorsForOrigin("http://localhost:3000") {
		t.Fatalf("expected localhost to be allowed")
	}
	if !allowCorsForOrigin("http://127.0.0.1:3000") {
		t.Fatalf("expected 127.0.0.1 to be allowed")
	}
}

func TestRewriteLocation(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://proxy.local/github.com/owner/repo", nil)

	loc := rewriteLocation("https://github.com/owner/repo?x=1", req)
	if loc != "/github.com/owner/repo?x=1" {
		t.Fatalf("unexpected rewritten location: %s", loc)
	}

	loc = rewriteLocation("/relative/path", req)
	if loc != "/relative/path" {
		t.Fatalf("unexpected relative location: %s", loc)
	}
}

func TestHandleRequestInvalidPath(t *testing.T) {
	setAllowedOrigins(t, []string{"https://budabit.club"})
	req := httptest.NewRequest(http.MethodGet, "http://proxy.local/", nil)
	rec := httptest.NewRecorder()

	handleRequestAndRedirect(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "invalid path") {
		t.Fatalf("expected invalid path response, got %q", string(body))
	}
}

func TestHandleRequestPreflight(t *testing.T) {
	setAllowedOrigins(t, []string{"https://budabit.club"})
	req := httptest.NewRequest(
		http.MethodOptions,
		"http://proxy.local/github.com/owner/repo.git/info/refs?service=git-upload-pack",
		nil,
	)
	req.Header.Set("Origin", "https://budabit.club")
	rec := httptest.NewRecorder()

	handleRequestAndRedirect(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "https://budabit.club" {
		t.Fatalf("unexpected allow origin header: %q", got)
	}
}

func TestHandleRequestPreflightWithoutDotGit(t *testing.T) {
	setAllowedOrigins(t, []string{"https://budabit.club"})
	req := httptest.NewRequest(
		http.MethodOptions,
		"http://proxy.local/github.com/owner/repo/info/refs?service=git-upload-pack",
		nil,
	)
	req.Header.Set("Origin", "https://budabit.club")
	rec := httptest.NewRecorder()

	handleRequestAndRedirect(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, resp.StatusCode)
	}
}

func TestHandleRequestProxyAndRewrite(t *testing.T) {
	setAllowedOrigins(t, []string{"https://budabit.club"})
	setAllowPrivateTargets(t, true)

	var gotUserAgent string
	var gotPath string
	var gotQuery string
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserAgent = r.Header.Get("User-Agent")
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.Header().Set("Location", "https://example.com/redirect?ok=1")
		w.Header().Set("Content-Length", "123")
		w.WriteHeader(http.StatusFound)
		io.WriteString(w, "redirected")
	}))
	defer upstream.Close()
	trustTLSServer(t, upstream)

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatalf("parse upstream url: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodGet,
		"http://proxy.local/"+upstreamURL.Host+"/owner/repo.git/info/refs?service=git-upload-pack",
		nil,
	)
	req.Header.Set("Origin", "https://budabit.club")
	rec := httptest.NewRecorder()

	handleRequestAndRedirect(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, resp.StatusCode)
	}
	if gotPath != "/owner/repo.git/info/refs" {
		t.Fatalf("unexpected upstream path: %q", gotPath)
	}
	if gotQuery != "service=git-upload-pack" {
		t.Fatalf("unexpected upstream query: %q", gotQuery)
	}
	if gotUserAgent != "git/budabit-go-cors-proxy" {
		t.Fatalf("unexpected upstream user agent: %q", gotUserAgent)
	}
	if got := resp.Header.Get("Location"); got != "/example.com/redirect?ok=1" {
		t.Fatalf("unexpected location header: %q", got)
	}
	if got := resp.Header.Get("X-Redirected-URL"); got != "/example.com/redirect?ok=1" {
		t.Fatalf("unexpected redirected header: %q", got)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "https://budabit.club" {
		t.Fatalf("unexpected allow origin header: %q", got)
	}
	if got := resp.Header.Get("Content-Length"); got != "" {
		t.Fatalf("expected no content-length header, got %q", got)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "redirected" {
		t.Fatalf("unexpected body: %q", string(body))
	}
}

func TestHandleRequestAllowsMissingDotGit(t *testing.T) {
	setAllowedOrigins(t, []string{"https://budabit.club"})
	setAllowPrivateTargets(t, true)

	var gotPath string
	var gotQuery string
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "ok")
	}))
	defer upstream.Close()
	trustTLSServer(t, upstream)

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatalf("parse upstream url: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodGet,
		"http://proxy.local/"+upstreamURL.Host+"/owner/repo/info/refs?service=git-upload-pack",
		nil,
	)
	req.Header.Set("Origin", "https://budabit.club")
	rec := httptest.NewRecorder()

	handleRequestAndRedirect(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
	if gotPath != "/owner/repo/info/refs" {
		t.Fatalf("unexpected upstream path: %q", gotPath)
	}
	if gotQuery != "service=git-upload-pack" {
		t.Fatalf("unexpected upstream query: %q", gotQuery)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "ok" {
		t.Fatalf("unexpected body: %q", string(body))
	}
}

func TestHandleRequestRejectsNonGitPath(t *testing.T) {
	setAllowedOrigins(t, []string{"https://budabit.club"})
	req := httptest.NewRequest(http.MethodGet, "http://proxy.local/github.com/.git/config", nil)
	req.Header.Set("Origin", "https://budabit.club")
	rec := httptest.NewRecorder()

	handleRequestAndRedirect(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, resp.StatusCode)
	}
}

func TestValidateTargetHostRejectsPrivateIP(t *testing.T) {
	setAllowPrivateTargets(t, false)
	if err := validateTargetHost("127.0.0.1"); err == nil {
		t.Fatalf("expected private IP to be rejected")
	}
}

func TestValidateGitSmartHTTPRequestAllowsMissingDotGit(t *testing.T) {
	normalized, err := validateGitSmartHTTPRequest(
		http.MethodGet,
		"owner/repo/info/refs",
		url.Values{"service": {"git-upload-pack"}},
	)
	if err != nil {
		t.Fatalf("expected request to validate: %v", err)
	}
	if normalized != "owner/repo/info/refs" {
		t.Fatalf("unexpected normalized path: %q", normalized)
	}
}

func TestValidateGitSmartHTTPRequestAllowsSourceHutStylePath(t *testing.T) {
	normalized, err := validateGitSmartHTTPRequest(
		http.MethodGet,
		"~sircmpwn/scdoc/info/refs",
		url.Values{"service": {"git-upload-pack"}},
	)
	if err != nil {
		t.Fatalf("expected sourcehut-style path to validate: %v", err)
	}
	if normalized != "~sircmpwn/scdoc/info/refs" {
		t.Fatalf("unexpected normalized path: %q", normalized)
	}
}

func TestValidateGitSmartHTTPRequestAllowsEncodedSegments(t *testing.T) {
	normalized, err := validateGitSmartHTTPRequest(
		http.MethodGet,
		"group/%40team/repo/info/refs",
		url.Values{"service": {"git-upload-pack"}},
	)
	if err != nil {
		t.Fatalf("expected encoded path to validate: %v", err)
	}
	if normalized != "group/%40team/repo/info/refs" {
		t.Fatalf("unexpected normalized path: %q", normalized)
	}
}
