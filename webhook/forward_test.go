package webhook

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestForwardEvent_RefusesSchemeChangingRedirect(t *testing.T) {
	t.Parallel()

	// Simulate a local server that redirects http -> https, the exact
	// scenario reported in https://github.com/cli/gh-webhook/issues/32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpsURL := "https://" + r.Host + r.URL.Path + "/"
		http.Redirect(w, r, httpsURL, http.StatusMovedPermanently)
	}))
	defer srv.Close()

	ev := wsEventReceived{
		Header: make(http.Header),
		Body:   []byte("test payload"),
	}

	_, err := forwardEvent(nil, srv.URL+"/webhooks", ev)
	if err == nil {
		t.Fatal("expected an error when target redirects to a different scheme, got nil")
	}

	if !strings.Contains(err.Error(), "refusing to follow redirect") {
		t.Errorf("expected a clear scheme-change error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "http") || !strings.Contains(err.Error(), "https") {
		t.Errorf("expected error to mention both schemes involved, got: %v", err)
	}
}

func TestForwardEvent_AllowsSameSchemeRedirect(t *testing.T) {
	t.Parallel()

	redirectTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer redirectTarget.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirectTarget.URL+"/webhooks", http.StatusMovedPermanently)
	}))
	defer srv.Close()

	ev := wsEventReceived{
		Header: make(http.Header),
		Body:   []byte("test payload"),
	}

	resp, err := forwardEvent(nil, srv.URL+"/webhooks", ev)
	if err != nil {
		t.Fatalf("expected same-scheme redirect to succeed, got error: %v", err)
	}
	if resp.Status != http.StatusOK {
		t.Errorf("expected status 200 after following same-scheme redirect, got: %d", resp.Status)
	}
}

func TestForwardEvent_SuccessfulRequest(t *testing.T) {
	t.Parallel()

	var receivedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		receivedBody = string(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ev := wsEventReceived{
		Header: make(http.Header),
		Body:   []byte("hello webhook"),
	}

	resp, err := forwardEvent(nil, srv.URL+"/webhooks", ev)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != http.StatusOK {
		t.Errorf("expected status 200, got: %d", resp.Status)
	}
	if receivedBody != "hello webhook" {
		t.Errorf("expected server to receive %q, got %q", "hello webhook", receivedBody)
	}
}
