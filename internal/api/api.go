// Package api is the typed wrapper over the HenKaiPan REST endpoints referenced
// from issue #1. All requests carry the X-API-Key header (and optional
// CF-Access-Client-* headers when configured).
//
// Schema notes (derived from the live API and the existing henkaipan-action):
//
//	POST /api/v1/scans/external
//	  body: { repo_url, scanners[], branch, project_id? }
//	  resp: { status: "accepted", batch_id, scan_ids[] }
//
//	GET /api/v1/scans/{id}/status
//	  resp: { scan: { status, ... }, findings[] }
//
//	GET /api/findings
//	  query: severity, status, scanner, project_id, page, page_size
//	  resp: { findings[], total, page, page_size }
//
//	GET /api/findings/export
//	  query: format (json|csv), same filters as /findings
//	  resp: raw body of the requested format
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dyallab/henkaipan-cli/internal/config"
)

// Client wraps a single base URL with a configured http.Client.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	APIKey     string
	CFClientID string
	CFSecret   string
}

// NewClient builds a Client with the given base URL, auth, and timeout.
func NewClient(cfg *config.Config) *Client {
	return &Client{
		BaseURL:    cfg.APIURL,
		HTTPClient: &http.Client{Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second},
		APIKey:     cfg.APIKey.Value(),
		CFClientID: cfg.CFAccessClientID,
		CFSecret:   cfg.CFAccessClientSecret.Value(),
	}
}

// ── Request helpers ──────────────────────────────────────────────────────────

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return fmt.Errorf("api: invalid base url: %w", err)
	}
	u.Path = strings.TrimRight(u.Path, "/") + path
	if query != nil {
		u.RawQuery = query.Encode()
	}

	var buf io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("api: marshal body: %w", err)
		}
		buf = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), buf)
	if err != nil {
		return fmt.Errorf("api: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", config.UserAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.APIKey != "" {
		req.Header.Set("X-API-Key", c.APIKey)
	}
	if c.CFClientID != "" && c.CFSecret != "" {
		req.Header.Set("CF-Access-Client-Id", c.CFClientID)
		req.Header.Set("CF-Access-Client-Secret", c.CFSecret)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("api: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("api: read response: %w", err)
	}

	// Cloudflare / WAF challenges return HTML even on 4xx; surface that clearly.
	if len(respBody) > 0 && respBody[0] == '<' {
		return &ProxyChallengeError{URL: u.String(), Status: resp.StatusCode, Body: respBody}
	}

	if resp.StatusCode >= 400 {
		return &HTTPError{Status: resp.StatusCode, Body: respBody, URL: u.String()}
	}

	if out != nil {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("api: decode response: %w", err)
		}
	}
	return nil
}

// HTTPError is returned for non-2xx responses with a JSON (or other non-HTML)
// body. The full body is preserved so the caller can surface the server's
// diagnostic message.
type HTTPError struct {
	Status int
	Body   []byte
	URL    string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("api: HTTP %d from %s: %s", e.Status, e.URL, truncate(string(e.Body), 512))
}

// ProxyChallengeError is returned when the response body looks like HTML
// (e.g. a Cloudflare challenge page) instead of the expected JSON.
type ProxyChallengeError struct {
	URL    string
	Status int
	Body   []byte
}

func (e *ProxyChallengeError) Error() string {
	return fmt.Sprintf("api: received HTML (HTTP %d) from %s; likely a proxy/firewall challenge. "+
		"If this is Cloudflare Access, set --cf-access-client-id / --cf-access-client-secret. "+
		"Otherwise add a WAF Skip rule for /api/v1/* — first 200 chars: %s",
		e.Status, e.URL, truncate(string(e.Body), 200))
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

// ── Endpoint contracts ────────────────────────────────────────────────────────

// ScanRunRequest is the body for POST /api/v1/scans/external.
// ProjectID is optional — when empty, the server auto-creates a project from
// the repo URL (mirrors the action's auto-create-project behavior).
type ScanRunRequest struct {
	RepoURL   string   `json:"repo_url"`
	Scanners  []string `json:"scanners"`
	Branch    string   `json:"branch,omitempty"`
	ProjectID string   `json:"project_id,omitempty"`
}

// ScanRunResponse mirrors the action's parsing of /scans/external.
type ScanRunResponse struct {
	Status   string   `json:"status"`
	BatchID  string   `json:"batch_id"`
	ScanIDs  []string `json:"scan_ids"`
	ProjectID string  `json:"project_id,omitempty"`
}

// ScanStatusResponse is what /scans/{id}/status returns.
type ScanStatusResponse struct {
	Scan     Scan     `json:"scan"`
	Findings []Finding `json:"findings"`
}

// Scan is the embedded scan object inside ScanStatusResponse.
type Scan struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Branch    string `json:"branch"`
	ProjectID string `json:"project_id"`
	StartedAt string `json:"started_at"`
	FinishedAt string `json:"finished_at,omitempty"`
}

// Finding is the canonical finding shape used by /scans/{id}/status,
// /api/findings, and /api/findings/export.
type Finding struct {
	ID         string `json:"id"`
	Scanner    string `json:"scanner"`
	Severity   string `json:"severity"`
	Status     string `json:"status"`
	Title      string `json:"title"`
	File       string `json:"file"`
	Line       int    `json:"line"`
	Message    string `json:"message"`
	ProjectID  string `json:"project_id"`
	ScanID     string `json:"scan_id"`
	CreatedAt  string `json:"created_at"`
}

// FindingsListResponse is the paginated envelope around Finding.
type FindingsListResponse struct {
	Findings []Finding `json:"findings"`
	Total    int       `json:"total"`
	Page     int       `json:"page"`
	PageSize int       `json:"page_size"`
}

// ── Public methods ───────────────────────────────────────────────────────────

// TriggerScan POSTs /api/v1/scans/external.
func (c *Client) TriggerScan(ctx context.Context, req ScanRunRequest) (*ScanRunResponse, error) {
	var out ScanRunResponse
	if err := c.do(ctx, http.MethodPost, "/api/v1/scans/external", nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ScanStatus GETs /api/v1/scans/{id}/status.
func (c *Client) ScanStatus(ctx context.Context, id string) (*ScanStatusResponse, error) {
	var out ScanStatusResponse
	if err := c.do(ctx, http.MethodGet, "/api/v1/scans/"+url.PathEscape(id)+"/status", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListFindings GETs /api/findings with the supplied filters.
// Zero-valued filters are dropped from the query string.
func (c *Client) ListFindings(ctx context.Context, f FindingsFilter) (*FindingsListResponse, error) {
	q := url.Values{}
	if f.Severity != "" { q.Set("severity", f.Severity) }
	if f.Status != "" { q.Set("status", f.Status) }
	if f.Scanner != "" { q.Set("scanner", f.Scanner) }
	if f.ProjectID != "" { q.Set("project_id", f.ProjectID) }
	if f.Page > 0 { q.Set("page", fmt.Sprintf("%d", f.Page)) }
	if f.PageSize > 0 { q.Set("page_size", fmt.Sprintf("%d", f.PageSize)) }

	var out FindingsListResponse
	if err := c.do(ctx, http.MethodGet, "/api/findings", q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// FindingsFilter is the user-facing filter set for ListFindings.
type FindingsFilter struct {
	Severity  string
	Status    string
	Scanner   string
	ProjectID string
	Page      int
	PageSize  int
}

// ExportFindings GETs /api/findings/export with the given format and filters,
// returning the raw bytes so the caller can stream them to disk or stdout.
func (c *Client) ExportFindings(ctx context.Context, format string, f FindingsFilter) ([]byte, error) {
	q := url.Values{}
	q.Set("format", format)
	if f.Severity != "" { q.Set("severity", f.Severity) }
	if f.Status != "" { q.Set("status", f.Status) }
	if f.Scanner != "" { q.Set("scanner", f.Scanner) }
	if f.ProjectID != "" { q.Set("project_id", f.ProjectID) }

	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("api: invalid base url: %w", err)
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/api/findings/export"
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("api: build request: %w", err)
	}
	req.Header.Set("User-Agent", config.UserAgent)
	if c.APIKey != "" {
		req.Header.Set("X-API-Key", c.APIKey)
	}
	if c.CFClientID != "" && c.CFSecret != "" {
		req.Header.Set("CF-Access-Client-Id", c.CFClientID)
		req.Header.Set("CF-Access-Client-Secret", c.CFSecret)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("api: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("api: read response: %w", err)
	}
	if resp.StatusCode >= 400 {
		if len(body) > 0 && body[0] == '<' {
			return nil, &ProxyChallengeError{URL: u.String(), Status: resp.StatusCode, Body: body}
		}
		return nil, &HTTPError{Status: resp.StatusCode, Body: body, URL: u.String()}
	}
	return body, nil
}