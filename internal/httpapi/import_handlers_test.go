package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
)

// TestImportLiveaboardRBAC: Cruise Director can't kick a scrape.
func TestImportLiveaboardRBAC(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	_, org, _ := signInAsAdmin(t, h)
	dirCookie, _ := bootstrapDirector(t, h, org.ID)

	resp, _ := doJSON(t, c, "POST", h.server.URL+"/api/admin/import/liveaboard", map[string]any{
		"url": "https://www.liveaboard.com/diving/indonesia/gaia-love",
	}, dirCookie)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("director kick: %d want 403", resp.StatusCode)
	}
}

// TestImportLiveaboardRequiresHTTPS: refuses non-https URLs.
func TestImportLiveaboardRequiresHTTPS(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	cookie, _, _ := signInAsAdmin(t, h)

	for _, bad := range []string{
		"http://www.liveaboard.com/diving/x",
		"file:///etc/passwd",
		"not-a-url",
	} {
		resp, body := doJSON(t, c, "POST", h.server.URL+"/api/admin/import/liveaboard", map[string]any{"url": bad}, cookie)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("url %q: %d %v", bad, resp.StatusCode, body)
		}
	}
}

// TestImportLiveaboardHostAllowlist: only liveaboard.com hosts pass.
func TestImportLiveaboardHostAllowlist(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	cookie, _, _ := signInAsAdmin(t, h)

	resp, body := doJSON(t, c, "POST", h.server.URL+"/api/admin/import/liveaboard", map[string]any{
		"url": "https://evil.example.com/diving/x",
	}, cookie)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("evil host: %d %v want 400", resp.StatusCode, body)
	}
}

// TestImportJobNotFound: GET an unknown job returns 404.
func TestImportJobNotFound(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	cookie, _, _ := signInAsAdmin(t, h)
	resp, _ := doJSON(t, c, "GET", h.server.URL+"/api/admin/import/jobs/00000000-0000-0000-0000-000000000000", nil, cookie)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown job: %d want 404", resp.StatusCode)
	}
}

// TestSpreadsheetPreviewMissingColumn: missing required column → 400.
func TestSpreadsheetPreviewMissingColumn(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	cookie, _, _ := signInAsAdmin(t, h)

	body, contentType := buildMultipart(t, "no_vessel.csv",
		"trip start date,trip end date,itinerary\n2026-06-02,2026-06-09,Komodo\n")
	req, _ := http.NewRequestWithContext(context.Background(), "POST",
		h.server.URL+"/api/admin/import/spreadsheet/preview", body)
	req.Header.Set("Content-Type", contentType)
	req.AddCookie(cookie)
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: %d want 400", resp.StatusCode)
	}
	raw, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(raw), "vessel name") {
		t.Errorf("error should mention vessel name: %s", raw)
	}
}

// TestSpreadsheetPreviewHappyPath: returns preview_id and parses rows.
func TestSpreadsheetPreviewHappyPath(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	cookie, _, _ := signInAsAdmin(t, h)

	csv := "vessel name,trip start date,trip end date,itinerary,number of guests\n" +
		"Gaia Love,2026-06-02,2026-06-09,Komodo,12\n"
	body, contentType := buildMultipart(t, "ok.csv", csv)
	req, _ := http.NewRequest("POST",
		h.server.URL+"/api/admin/import/spreadsheet/preview", body)
	req.Header.Set("Content-Type", contentType)
	req.AddCookie(cookie)
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("preview: %d %s", resp.StatusCode, raw)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if _, ok := out["preview_id"].(string); !ok {
		t.Errorf("missing preview_id: %v", out)
	}
	if _, ok := out["payload"]; !ok {
		t.Errorf("missing payload: %v", out)
	}
}

// TestSpreadsheetCommitExpiredPreview: unknown preview id → 410.
func TestSpreadsheetCommitExpiredPreview(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	cookie, _, _ := signInAsAdmin(t, h)

	resp, _ := doJSON(t, c, "POST",
		h.server.URL+"/api/admin/import/spreadsheet/commit",
		map[string]any{
			"preview_id":     "11111111-1111-1111-1111-111111111111",
			"vessel_mapping": map[string]any{},
			"rows_to_skip":   []int{},
		}, cookie)
	if resp.StatusCode != http.StatusGone {
		t.Fatalf("status: %d want 410", resp.StatusCode)
	}
}

func buildMultipart(t *testing.T, filename, content string) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf, w.FormDataContentType()
}
