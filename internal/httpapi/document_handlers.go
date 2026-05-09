package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/karimfan/liveaboard/internal/auth"
	"github.com/karimfan/liveaboard/internal/documents"
	"github.com/karimfan/liveaboard/internal/store"
)

func (s *Server) documentService() *documents.Service {
	return documents.New(s.DocumentsDir)
}

func (s *Server) handleListGuestDocuments(w http.ResponseWriter, r *http.Request) {
	_, tripGuest, ok := s.guestRegistrationAccess(w, r)
	if !ok {
		return
	}
	docs, err := s.Auth.Store.GuestDocumentsForTripGuest(r.Context(), tripGuest.OrganizationID, tripGuest.TripID, tripGuest.ID, false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"documents": guestDocumentViews(docs, true, tripGuest.TripID, tripGuest.ID)})
}

func (s *Server) handleUploadGuestDocument(w http.ResponseWriter, r *http.Request) {
	guest, tripGuest, ok := s.guestRegistrationAccess(w, r)
	if !ok {
		return
	}
	if !s.ensureTripMutable(w, r, tripGuest.OrganizationID, tripGuest.TripID) {
		return
	}
	doc, err := s.uploadDocument(w, r, documentUploadAccess{
		OrganizationID: tripGuest.OrganizationID,
		TripID:         tripGuest.TripID,
		TripGuestID:    tripGuest.ID,
		GuestUserID:    &guest.ID,
	})
	if err != nil {
		writeDocumentError(w, err)
		return
	}
	_, _ = s.Auth.Store.RecordAuditEvent(r.Context(), store.AuditEventInput{
		OrganizationID: tripGuest.OrganizationID,
		Actor:          store.GuestAuditActor(guest.ID),
		Action:         "guest.document_uploaded",
		EntityType:     "guest_document",
		EntityID:       &doc.ID,
		TripID:         &tripGuest.TripID,
		TripGuestID:    &tripGuest.ID,
		Metadata:       documentAuditMetadata(doc, true),
	})
	writeJSON(w, http.StatusCreated, guestDocumentView(doc, true, tripGuest.TripID, tripGuest.ID))
}

func (s *Server) handleOpenGuestDocument(w http.ResponseWriter, r *http.Request) {
	guest, tripGuest, ok := s.guestRegistrationAccess(w, r)
	if !ok {
		return
	}
	documentID, ok := uuidParam(w, r, "document_id")
	if !ok {
		return
	}
	doc, err := s.Auth.Store.GuestDocumentByID(r.Context(), tripGuest.OrganizationID, tripGuest.TripID, tripGuest.ID, documentID)
	if err != nil {
		writeDocumentError(w, err)
		return
	}
	if doc.ArchivedAt != nil {
		writeError(w, http.StatusNotFound, "not_found", "document not found")
		return
	}
	_, _ = s.Auth.Store.RecordAuditEvent(r.Context(), store.AuditEventInput{
		OrganizationID: tripGuest.OrganizationID,
		Actor:          store.GuestAuditActor(guest.ID),
		Action:         "guest.document_downloaded",
		EntityType:     "guest_document",
		EntityID:       &doc.ID,
		TripID:         &tripGuest.TripID,
		TripGuestID:    &tripGuest.ID,
		Metadata:       documentAuditMetadata(doc, false),
	})
	s.streamDocument(w, r, doc)
}

func (s *Server) handleListStaffGuestDocuments(w http.ResponseWriter, r *http.Request) {
	_, tripID, guestID, ok := s.staffGuestAccess(w, r)
	if !ok {
		return
	}
	u := auth.UserFromContext(r.Context())
	docs, err := s.Auth.Store.GuestDocumentsForTripGuest(r.Context(), u.OrganizationID, tripID, guestID, true)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"documents": guestDocumentViews(docs, false, tripID, guestID)})
}

func (s *Server) handleUploadStaffGuestDocument(w http.ResponseWriter, r *http.Request) {
	u, tripID, guestID, ok := s.staffGuestAccess(w, r)
	if !ok {
		return
	}
	if !s.ensureTripMutable(w, r, u.OrganizationID, tripID) {
		return
	}
	doc, err := s.uploadDocument(w, r, documentUploadAccess{
		OrganizationID: u.OrganizationID,
		TripID:         tripID,
		TripGuestID:    guestID,
		UserID:         &u.ID,
	})
	if err != nil {
		writeDocumentError(w, err)
		return
	}
	_, _ = s.Auth.Store.RecordAuditEvent(r.Context(), store.AuditEventInput{
		OrganizationID: u.OrganizationID,
		Actor:          store.StaffAuditActor(u.ID),
		Action:         "guest.document_uploaded",
		EntityType:     "guest_document",
		EntityID:       &doc.ID,
		TripID:         &tripID,
		TripGuestID:    &guestID,
		Metadata:       documentAuditMetadata(doc, true),
	})
	writeJSON(w, http.StatusCreated, guestDocumentView(doc, false, tripID, guestID))
}

func (s *Server) handleOpenStaffGuestDocument(w http.ResponseWriter, r *http.Request) {
	u, tripID, guestID, ok := s.staffGuestAccess(w, r)
	if !ok {
		return
	}
	documentID, ok := uuidParam(w, r, "document_id")
	if !ok {
		return
	}
	doc, err := s.Auth.Store.GuestDocumentByID(r.Context(), u.OrganizationID, tripID, guestID, documentID)
	if err != nil {
		writeDocumentError(w, err)
		return
	}
	_, _ = s.Auth.Store.RecordAuditEvent(r.Context(), store.AuditEventInput{
		OrganizationID: u.OrganizationID,
		Actor:          store.StaffAuditActor(u.ID),
		Action:         "guest.document_downloaded",
		EntityType:     "guest_document",
		EntityID:       &doc.ID,
		TripID:         &tripID,
		TripGuestID:    &guestID,
		Metadata:       documentAuditMetadata(doc, false),
	})
	s.streamDocument(w, r, doc)
}

func (s *Server) handleArchiveStaffGuestDocument(w http.ResponseWriter, r *http.Request) {
	u, tripID, guestID, ok := s.staffGuestAccess(w, r)
	if !ok {
		return
	}
	if !s.ensureTripMutable(w, r, u.OrganizationID, tripID) {
		return
	}
	documentID, ok := uuidParam(w, r, "document_id")
	if !ok {
		return
	}
	doc, err := s.Auth.Store.ArchiveGuestDocument(r.Context(), u.OrganizationID, tripID, guestID, documentID, u.ID)
	if err != nil {
		writeDocumentError(w, err)
		return
	}
	_, _ = s.Auth.Store.RecordAuditEvent(r.Context(), store.AuditEventInput{
		OrganizationID: u.OrganizationID,
		Actor:          store.StaffAuditActor(u.ID),
		Action:         "guest.document_archived",
		EntityType:     "guest_document",
		EntityID:       &doc.ID,
		TripID:         &tripID,
		TripGuestID:    &guestID,
		Metadata:       map[string]any{"category": doc.Category, "display_name": doc.DisplayName},
	})
	writeJSON(w, http.StatusOK, guestDocumentView(doc, false, tripID, guestID))
}

func (s *Server) handleGuestActivity(w http.ResponseWriter, r *http.Request) {
	u, tripID, guestID, ok := s.staffGuestAccess(w, r)
	if !ok {
		return
	}
	events, err := s.Auth.Store.AuditEventsForTripGuest(r.Context(), u.OrganizationID, tripID, guestID, 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": auditEventViews(events)})
}

func (s *Server) handleAuditEvents(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	q := r.URL.Query()
	f := store.AuditEventFilters{
		Action:     strings.TrimSpace(q.Get("action")),
		EntityType: strings.TrimSpace(q.Get("entity_type")),
		ActorType:  strings.TrimSpace(q.Get("actor_type")),
		Limit:      intQuery(q.Get("limit")),
	}
	if v := q.Get("trip_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_input", "trip_id must be a uuid")
			return
		}
		f.TripID = &id
	}
	if v := q.Get("trip_guest_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_input", "trip_guest_id must be a uuid")
			return
		}
		f.TripGuestID = &id
	}
	if v := q.Get("date_from"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_input", "date_from must be RFC3339")
			return
		}
		f.DateFrom = &t
	}
	if v := q.Get("date_to"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_input", "date_to must be RFC3339")
			return
		}
		f.DateTo = &t
	}
	if u.Role == store.RoleCruiseDirector {
		f.AssignedTo = &u.ID
	}
	events, err := s.Auth.Store.AuditEvents(r.Context(), u.OrganizationID, f)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": auditEventViews(events)})
}

type documentUploadAccess struct {
	OrganizationID uuid.UUID
	TripID         uuid.UUID
	TripGuestID    uuid.UUID
	UserID         *uuid.UUID
	GuestUserID    *uuid.UUID
}

func (s *Server) uploadDocument(w http.ResponseWriter, r *http.Request, access documentUploadAccess) (*store.GuestDocument, error) {
	r.Body = http.MaxBytesReader(w, r.Body, documents.MaxUploadBytes+(1<<20))
	if err := r.ParseMultipartForm(documents.MaxUploadBytes + 1<<20); err != nil {
		return nil, fmt.Errorf("%w: multipart", documents.ErrInvalidFile)
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		return nil, fmt.Errorf("%w: file", documents.ErrInvalidFile)
	}
	defer file.Close()
	notes := strings.TrimSpace(r.FormValue("notes"))
	var notesPtr *string
	if notes != "" {
		notesPtr = &notes
	}
	svc := s.documentService()
	prepared, err := svc.PrepareUpload(documents.UploadInput{
		OrganizationID:        access.OrganizationID,
		TripID:                access.TripID,
		TripGuestID:           access.TripGuestID,
		UploadedByUserID:      access.UserID,
		UploadedByGuestUserID: access.GuestUserID,
		Category:              r.FormValue("category"),
		DisplayName:           r.FormValue("display_name"),
		Notes:                 notesPtr,
		File:                  file,
		Header:                header,
	})
	if err != nil {
		return nil, err
	}
	if err := svc.FinalizePath(prepared.TempPath, prepared.Document.StorageKey); err != nil {
		return nil, err
	}
	doc, err := s.Auth.Store.CreateGuestDocument(r.Context(), *prepared.Document)
	if err != nil {
		svc.Remove(svc.Path(prepared.Document.StorageKey))
		return nil, err
	}
	return doc, nil
}

func (s *Server) streamDocument(w http.ResponseWriter, r *http.Request, doc *store.GuestDocument) {
	path := s.documentService().Path(doc.StorageKey)
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusNotFound, "not_found", "document file not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	defer f.Close()
	disposition := "inline"
	if r.URL.Query().Get("download") == "1" || !documents.IsBrowserInline(doc.ContentType) {
		disposition = "attachment"
	}
	w.Header().Set("Content-Type", doc.ContentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": doc.OriginalFilename}))
	http.ServeContent(w, r, doc.OriginalFilename, doc.CreatedAt, f)
}

func (s *Server) staffGuestAccess(w http.ResponseWriter, r *http.Request) (*store.User, uuid.UUID, uuid.UUID, bool) {
	u := auth.UserFromContext(r.Context())
	tripID, guestID, ok := tripAndGuestParams(w, r)
	if !ok {
		return nil, uuid.Nil, uuid.Nil, false
	}
	if _, ok := s.authorizeManifestAccess(w, r, u, tripID); !ok {
		return nil, uuid.Nil, uuid.Nil, false
	}
	if _, err := s.Auth.Store.TripGuestByID(r.Context(), u.OrganizationID, tripID, guestID); err != nil {
		writeGuestServiceError(w, err)
		return nil, uuid.Nil, uuid.Nil, false
	}
	return u, tripID, guestID, true
}

func guestDocumentViews(docs []*store.GuestDocument, guestRoute bool, tripID, tripGuestID uuid.UUID) []map[string]any {
	out := make([]map[string]any, 0, len(docs))
	for _, doc := range docs {
		out = append(out, guestDocumentView(doc, guestRoute, tripID, tripGuestID))
	}
	return out
}

func guestDocumentView(doc *store.GuestDocument, guestRoute bool, tripID, tripGuestID uuid.UUID) map[string]any {
	base := ""
	if guestRoute {
		base = fmt.Sprintf("/guest/trip-registrations/%s/documents/%s", tripGuestID, doc.ID)
	} else {
		base = fmt.Sprintf("/admin/trips/%s/guests/%s/documents/%s", tripID, tripGuestID, doc.ID)
	}
	return map[string]any{
		"id":                doc.ID,
		"category":          doc.Category,
		"display_name":      doc.DisplayName,
		"original_filename": doc.OriginalFilename,
		"content_type":      doc.ContentType,
		"size_bytes":        doc.SizeBytes,
		"notes":             doc.Notes,
		"archived_at":       doc.ArchivedAt,
		"created_at":        doc.CreatedAt,
		"view_url":          base,
		"download_url":      base + "?download=1",
	}
}

func documentAuditMetadata(doc *store.GuestDocument, includeSize bool) map[string]any {
	out := map[string]any{
		"category":     doc.Category,
		"display_name": doc.DisplayName,
		"content_type": doc.ContentType,
	}
	if includeSize {
		out["size_bytes"] = doc.SizeBytes
	}
	return out
}

func auditEventViews(events []*store.AuditEvent) []map[string]any {
	out := make([]map[string]any, 0, len(events))
	for _, ev := range events {
		var meta map[string]any
		_ = json.Unmarshal(ev.Metadata, &meta)
		out = append(out, map[string]any{
			"id":            ev.ID,
			"actor_type":    ev.ActorType,
			"action":        ev.Action,
			"entity_type":   ev.EntityType,
			"entity_id":     ev.EntityID,
			"trip_id":       ev.TripID,
			"trip_guest_id": ev.TripGuestID,
			"metadata":      meta,
			"created_at":    ev.CreatedAt,
		})
	}
	return out
}

func intQuery(v string) int {
	var n int
	for _, r := range v {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func writeDocumentError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, documents.ErrInvalidFile), errors.Is(err, store.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "invalid_input", trimSentinel(err.Error()))
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "not found")
	default:
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
	}
}
