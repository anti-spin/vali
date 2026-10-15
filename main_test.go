package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsValidDomainInput(t *testing.T) {
	tests := []struct {
		domain string
		valid  bool
	}{
		{"example.com", true},
		{"sub.example.net", true},
		{"my-domain.org", true},
		{"a.b.c.de", true},
		// Invalid cases
		{"", false},
		{"example", false}, // Missing TLD
		{"-example.com", false},
		{"example..com", false},
		{"example.com/", false},
		{"example.com;DROP TABLE domains;", false},
		{"invalid_char!.com", false},
		{"a_b.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.domain, func(t *testing.T) {
			got := isValidDomainInput(tt.domain)
			if got != tt.valid {
				t.Errorf("isValidDomainInput(%q) = %v; want %v", tt.domain, got, tt.valid)
			}
		})
	}
}

func TestValidateStaticDomains(t *testing.T) {
	// Set up static domains for testing
	staticDomains = []string{"example.com", "*.internal.net"}
	useRedis = false
	useMariaDB = false

	router := setupRouter()

	tests := []struct {
		name           string
		urlPath        string
		expectedStatus int
	}{
		{"Exact match", "/vali/example.com", http.StatusOK},
		{"Exact match with trailing dot", "/vali/example.com.", http.StatusOK},
		{"Wildcard match", "/vali/app.internal.net", http.StatusOK},
		{"Nested wildcard match", "/vali/deep.sub.internal.net", http.StatusOK},
		{"Non-matching domain", "/vali/unknown.com", http.StatusNotFound},
		{"Malformed domain format", "/vali/invalid_domain!.com", http.StatusBadRequest},
		{"Health check", "/healthz", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", tt.urlPath, nil)
			if err != nil {
				t.Fatalf("Could not create request: %v", err)
			}

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("handler returned wrong status code for %s: got %v want %v",
					tt.urlPath, rr.Code, tt.expectedStatus)
			}
		})
	}
}
