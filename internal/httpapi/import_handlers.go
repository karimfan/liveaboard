package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/karimfan/liveaboard/internal/auth"
	"github.com/karimfan/liveaboard/internal/imports/spreadsheet"
	"github.com/karimfan/liveaboard/internal/store"
)

// import_handlers.go: Sprint 012 native trip import surface.
//
//   - POST /api/admin/import/liveaboard            kick off scrape job
//   - GET  /api/admin/import/jobs/{id}              poll job status
//   - POST /api/admin/import/spreadsheet/preview    parse uploaded file
//   - POST /api/admin/import/spreadsheet/commit     write trips
//
// All routes are mounted behind RequireOrgAdmin in httpapi.go.

const (
	importMaxUploadBytes = 2 << 20 // 2 MiB
)

// allowedScrapeHosts is the host allow-list for the liveaboard kick.
// SSRF defense: only the public site we already integrate with.
var allowedScrapeHosts = map[string]struct{}{
	"liveaboard.com":     {},
	"www.liveaboard.com": {},
}

// --- liveaboard kick + status ---

type kickLiveaboardReq struct {
	URL string `json:"url"`
}

func (s *Server) handleKickLiveaboardImport(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	var req kickLiveaboardReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	parsed, err := url.Parse(strings.TrimSpace(req.URL))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "url must be an https:// URL")
		return
	}
	if _, ok := allowedScrapeHosts[strings.ToLower(parsed.Host)]; !ok {
		writeError(w, http.StatusBadRequest, "invalid_input", "only liveaboard.com URLs are accepted")
		return
	}

	job, err := s.ImportRunner.Kick(r.Context(), u.OrganizationID, u.ID, parsed.String())
	if err != nil {
		s.Log.Error("kick liveaboard import", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusAccepted, importJobView(job))
}

func (s *Server) handleGetImportJob(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "id must be a uuid")
		return
	}
	job, err := s.Auth.Store.ImportJobByID(r.Context(), u.OrganizationID, id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "import job not found")
		return
	}
	if err != nil {
		s.Log.Error("get import job", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	out := importJobView(job)
	if job.Status == store.ImportStatusSucceeded {
		if boats, err := s.Auth.Store.UnconfiguredBoats(r.Context(), u.OrganizationID); err == nil {
			out["unconfigured_boats"] = boatLayoutSummariesView(boats)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// --- spreadsheet preview ---

func (s *Server) handleSpreadsheetPreview(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())

	r.Body = http.MaxBytesReader(w, r.Body, importMaxUploadBytes)
	if err := r.ParseMultipartForm(importMaxUploadBytes); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_upload", "file too large or malformed multipart")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_upload", "missing 'file' part")
		return
	}
	defer file.Close()

	filename := filepath.Base(header.Filename)
	ext := strings.ToLower(filepath.Ext(filename))

	var preview *spreadsheet.Preview
	switch ext {
	case ".csv":
		preview, err = spreadsheet.ParseCSV(filename, file)
	case ".xlsx":
		preview, err = spreadsheet.ParseXLSX(filename, file)
	default:
		writeError(w, http.StatusBadRequest, "invalid_upload", "unsupported file type; upload .csv or .xlsx")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "parse_error", err.Error())
		return
	}

	known, err := s.boatsByLowerName(r.Context(), u.OrganizationID)
	if err != nil {
		s.Log.Error("preview: load boats", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	suggestions := make(map[string]map[string]any, len(preview.VesselNames))
	for _, v := range preview.VesselNames {
		if b, ok := known[strings.ToLower(strings.TrimSpace(v))]; ok {
			suggestions[v] = map[string]any{
				"boat_id":      b.ID,
				"display_name": b.DisplayName,
			}
		}
	}

	payload, err := json.Marshal(preview)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	row, err := s.Auth.Store.CreateImportPreview(r.Context(), u.OrganizationID, u.ID, filename, payload)
	if err != nil {
		s.Log.Error("preview: persist", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"preview_id":         row.ID,
		"expires_at":         row.ExpiresAt,
		"payload":            preview,
		"vessel_suggestions": suggestions,
	})
}

// --- spreadsheet commit ---

type commitSpreadsheetReq struct {
	PreviewID     string                         `json:"preview_id"`
	VesselMapping map[string]vesselMappingChoice `json:"vessel_mapping"`
	RowsToSkip    []int                          `json:"rows_to_skip"`
}

type vesselMappingChoice struct {
	// "existing" or "create_new". When "existing", BoatID is required.
	Mode   string  `json:"mode"`
	BoatID *string `json:"boat_id,omitempty"`
}

func (s *Server) handleSpreadsheetCommit(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	var req commitSpreadsheetReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	previewID, err := uuid.Parse(req.PreviewID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "preview_id must be a uuid")
		return
	}
	now := time.Now().UTC()
	row, err := s.Auth.Store.ImportPreviewByID(r.Context(), u.OrganizationID, previewID, now)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusGone, "preview_expired", "this preview is no longer available; re-upload the file")
		return
	}
	if err != nil {
		s.Log.Error("commit: load preview", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	var preview spreadsheet.Preview
	if err := json.Unmarshal(row.Payload, &preview); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	skipSet := map[int]bool{}
	for _, ln := range req.RowsToSkip {
		skipSet[ln] = true
	}

	boatByVessel := map[string]uuid.UUID{}
	for _, vessel := range preview.VesselNames {
		choice, ok := req.VesselMapping[vessel]
		if !ok {
			writeError(w, http.StatusBadRequest, "invalid_input", fmt.Sprintf("missing mapping for vessel %q", vessel))
			return
		}
		switch choice.Mode {
		case "existing":
			if choice.BoatID == nil {
				writeError(w, http.StatusBadRequest, "invalid_input", fmt.Sprintf("missing boat_id for vessel %q", vessel))
				return
			}
			id, err := uuid.Parse(*choice.BoatID)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_input", fmt.Sprintf("invalid boat_id for vessel %q", vessel))
				return
			}
			b, err := s.Auth.Store.BoatByID(r.Context(), u.OrganizationID, id)
			if errors.Is(err, store.ErrNotFound) {
				writeError(w, http.StatusBadRequest, "invalid_input", fmt.Sprintf("boat %s not in your organization", id))
				return
			}
			if err != nil {
				writeError(w, http.StatusInternalServerError, "internal", "internal error")
				return
			}
			boatByVessel[vessel] = b.ID
		case "create_new":
			b, err := s.createManualBoat(r.Context(), u.OrganizationID, vessel, now)
			if err != nil {
				s.Log.Error("commit: create manual boat", "err", err)
				writeError(w, http.StatusInternalServerError, "internal", "internal error")
				return
			}
			boatByVessel[vessel] = b.ID
		default:
			writeError(w, http.StatusBadRequest, "invalid_input", fmt.Sprintf("vessel %q: mode must be 'existing' or 'create_new'", vessel))
			return
		}
	}

	scrapesByBoat := map[uuid.UUID][]store.TripScrape{}
	for _, row := range preview.Rows {
		if skipSet[row.LineNumber] {
			continue
		}
		if row.VesselName == "" || row.StartDate.IsZero() || row.EndDate.IsZero() || row.EndDate.Before(row.StartDate) {
			continue
		}
		boatID, ok := boatByVessel[row.VesselName]
		if !ok {
			continue
		}
		scrapesByBoat[boatID] = append(scrapesByBoat[boatID], store.TripScrape{
			StartDate:     row.StartDate,
			EndDate:       row.EndDate,
			Itinerary:     row.Itinerary,
			SourceURL:     "spreadsheet:" + preview.Filename,
			SourceTripKey: spreadsheetTripKey(boatID, row),
			NumGuests:     row.NumGuests,
		})
	}

	today := now.Truncate(24 * time.Hour)
	rep, err := s.Auth.Store.ReplaceSpreadsheetTrips(r.Context(), u.OrganizationID, scrapesByBoat, now, today)
	if err != nil {
		s.Log.Error("commit: replace trips", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	job, err := s.Auth.Store.CreateImportJob(r.Context(), u.OrganizationID, u.ID, store.ImportSourceSpreadsheet, preview.Filename)
	if err == nil {
		_ = s.Auth.Store.MarkImportJobSucceeded(r.Context(), job.ID, store.ImportResult{
			TripsInserted: rep.Inserts,
			TripsUpdated:  rep.Updates,
			TripsDeleted:  rep.StaleDeletes,
		})
	}
	_ = s.Auth.Store.DeleteImportPreview(r.Context(), u.OrganizationID, previewID)

	out := map[string]any{
		"trips_inserted": rep.Inserts,
		"trips_updated":  rep.Updates,
		"trips_deleted":  rep.StaleDeletes,
	}
	if job != nil {
		out["job_id"] = job.ID
	}
	writeJSON(w, http.StatusOK, out)
}

// --- helpers ---

func (s *Server) boatsByLowerName(ctx context.Context, orgID uuid.UUID) (map[string]*store.Boat, error) {
	boats, err := s.Auth.Store.BoatsForOrg(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]*store.Boat, len(boats)*2)
	for _, b := range boats {
		out[strings.ToLower(strings.TrimSpace(b.DisplayName))] = b
		if b.SourceName != "" && b.SourceName != b.DisplayName {
			out[strings.ToLower(strings.TrimSpace(b.SourceName))] = b
		}
	}
	return out, nil
}

// createManualBoat inserts a fresh boat with source_provider='manual'
// so subsequent spreadsheet uploads dedup against it via vessel-name
// match, and a future liveaboard.com scrape doesn't collide because
// it uses a different source_provider.
func (s *Server) createManualBoat(ctx context.Context, orgID uuid.UUID, vesselName string, now time.Time) (*store.Boat, error) {
	const sourceProvider = "manual"
	slug := manualSlug(vesselName, now)
	return s.Auth.Store.UpsertBoat(ctx, orgID, sourceProvider, store.BoatScrape{
		Slug:       slug,
		Name:       vesselName,
		URL:        "",
		ImageURL:   "",
		ExternalID: "",
	}, now)
}

// manualSlug generates a stable-ish slug from the vessel name for
// the manual source. We append a short timestamp suffix so two
// imports of the same name within the same org don't collide on
// the (org, source_provider, source_slug) unique index — though
// in practice a same-name vessel should resolve to the existing
// boat via the mapping UI.
func manualSlug(vesselName string, now time.Time) string {
	base := strings.ToLower(strings.TrimSpace(vesselName))
	var b strings.Builder
	for _, r := range base {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteByte('-')
		}
	}
	if b.Len() == 0 {
		b.WriteString("vessel")
	}
	return fmt.Sprintf("%s-%d", b.String(), now.Unix())
}

func spreadsheetTripKey(boatID uuid.UUID, row spreadsheet.Row) string {
	return fmt.Sprintf("%s|%s|%s|%s",
		boatID,
		row.StartDate.Format("2006-01-02"),
		row.EndDate.Format("2006-01-02"),
		strings.ToLower(strings.TrimSpace(row.Itinerary)),
	)
}

func importJobView(j *store.ImportJob) map[string]any {
	out := map[string]any{
		"id":           j.ID,
		"source":       j.Source,
		"source_input": j.SourceInput,
		"status":       j.Status,
		"started_at":   j.StartedAt,
		"completed_at": j.CompletedAt,
	}
	if j.BoatsInserted != nil {
		out["boats_inserted"] = *j.BoatsInserted
	}
	if j.BoatsUpdated != nil {
		out["boats_updated"] = *j.BoatsUpdated
	}
	if j.TripsInserted != nil {
		out["trips_inserted"] = *j.TripsInserted
	}
	if j.TripsUpdated != nil {
		out["trips_updated"] = *j.TripsUpdated
	}
	if j.TripsDeleted != nil {
		out["trips_deleted"] = *j.TripsDeleted
	}
	if j.ErrorMessage != nil {
		out["error_message"] = *j.ErrorMessage
	}
	return out
}

func boatLayoutSummariesView(rows []store.BoatLayoutSummary) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, map[string]any{
			"id":                 row.BoatID,
			"name":               row.BoatName,
			"active_berth_count": row.ActiveBerthCount,
		})
	}
	return out
}
