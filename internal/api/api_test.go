package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newTestServer wires up an httptest server with the given handler and
// returns a Client configured to talk to it. Always defer srv.Close().
func newTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *Client) {
	t.Helper()
	srv := httptest.NewServer(handler)
	c := &Client{
		BaseURL:    srv.URL,
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
		APIKey:     "test-key",
	}
	return srv, c
}

func TestTriggerScanSendsHeadersAndPayload(t *testing.T) {
	var gotBody ScanRunRequest
	var gotAPIKey, gotUA string
	srv, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get("X-API-Key")
		gotUA = r.Header.Get("User-Agent")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode body: %v", err)
		}
		_ = json.NewEncoder(w).Encode(ScanRunResponse{
			Status: "accepted", BatchID: "b1", ScanIDs: []string{"s1", "s2"},
		})
	})
	defer srv.Close()

	resp, err := c.TriggerScan(context.Background(), ScanRunRequest{
		RepoURL:  "https://github.com/o/r",
		Scanners: []string{"semgrep"},
		Branch:   "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotAPIKey != "test-key" {
		t.Errorf("X-API-Key = %q, want test-key", gotAPIKey)
	}
	if !strings.HasPrefix(gotUA, "henkaipan-cli/") {
		t.Errorf("User-Agent = %q", gotUA)
	}
	if resp.BatchID != "b1" || len(resp.ScanIDs) != 2 {
		t.Errorf("response = %+v", resp)
	}
	if gotBody.RepoURL != "https://github.com/o/r" {
		t.Errorf("payload repo_url = %q", gotBody.RepoURL)
	}
}

func TestScanStatus(t *testing.T) {
	srv, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/scans/abc/status") {
			t.Errorf("path = %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(ScanStatusResponse{
			Scan:     Scan{ID: "abc", Status: "completed"},
			Findings: []Finding{{ID: "f1", Severity: "high"}},
		})
	})
	defer srv.Close()

	st, err := c.ScanStatus(context.Background(), "abc")
	if err != nil {
		t.Fatal(err)
	}
	if st.Scan.Status != "completed" || len(st.Findings) != 1 {
		t.Errorf("got %+v", st)
	}
}

func TestListFindingsFilterShape(t *testing.T) {
	var captured string
	srv, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		captured = r.URL.RawQuery
		_ = json.NewEncoder(w).Encode(FindingsListResponse{
			Findings: []Finding{}, Total: 0, Page: 1, PageSize: 50,
		})
	})
	defer srv.Close()

	_, err := c.ListFindings(context.Background(), FindingsFilter{
		Severity: "high", ProjectID: "p1", Page: 2, PageSize: 25,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"severity=high", "project_id=p1", "page=2", "page_size=25"} {
		if !strings.Contains(captured, want) {
			t.Errorf("query missing %q: got %q", want, captured)
		}
	}
	if strings.Contains(captured, "status=") || strings.Contains(captured, "scanner=") {
		t.Errorf("empty filters should not appear: %q", captured)
	}
}

func TestProxyChallengeDetected(t *testing.T) {
	srv, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(403)
		_, _ = io.WriteString(w, "<!DOCTYPE html><html>cf challenge</html>")
	})
	defer srv.Close()

	_, err := c.ListFindings(context.Background(), FindingsFilter{})
	if err == nil {
		t.Fatal("expected error for HTML challenge")
	}
	if _, ok := err.(*ProxyChallengeError); !ok {
		t.Errorf("expected ProxyChallengeError, got %T: %v", err, err)
	}
}

func TestCFAccessHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("CF-Access-Client-Id") != "cf-cid" ||
			r.Header.Get("CF-Access-Client-Secret") != "cf-cs" {
			t.Errorf("cf headers missing: id=%q secret=%q",
				r.Header.Get("CF-Access-Client-Id"),
				r.Header.Get("CF-Access-Client-Secret"))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := &Client{
		BaseURL:    srv.URL,
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
		APIKey:     "k", CFClientID: "cf-cid", CFSecret: "cf-cs",
	}
	if _, err := c.ScanStatus(context.Background(), "x"); err != nil {
		// 204 has no JSON body; the wrapper will error trying to decode it.
		// That's fine — the test only asserts the CF headers were sent.
		t.Logf("status call returned %v (expected for 204)", err)
	}
}