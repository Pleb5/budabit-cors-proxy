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
	// Read comma-separated origins from ALLOWED_ORIGINS
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
	if isAllowedOrigin(origin) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	}
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if req.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	targetURL := req.URL.Query().Get("url")
	if targetURL == "" {
		http.Error(w, "Missing 'url' parameter", http.StatusBadRequest)
		return
	}

	parsedURL, err := url.Parse(targetURL)
	if err != nil || !parsedURL.IsAbs() {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	proxyReq, err := http.NewRequest(req.Method, targetURL, req.Body)
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	for k, v := range req.Header {
		if k != "Host" {
			for _, val := range v {
				proxyReq.Header.Add(k, val)
			}
		}
	}

	client := &http.Client{}
	resp, err := client.Do(proxyReq)
	if err != nil {
		http.Error(w, "Request to target failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for k, v := range resp.Header {
		for _, val := range v {
			w.Header().Add(k, val)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func isAllowedOrigin(o string) bool {
	for _, a := range allowedOrigins {
		if o == a {
			return true
		}
	}
	return false
}
