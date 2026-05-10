// Package httpapi wires the auth + organization services into HTTP handlers.
package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/karimfan/liveaboard/internal/auth"
	"github.com/karimfan/liveaboard/internal/imports"
	"github.com/karimfan/liveaboard/internal/org"
	"github.com/karimfan/liveaboard/internal/store"
)

// Server bundles dependencies the chi router needs. Sprint 009 replaced
// the Clerk-backed Exchanger / WebhookReceiver / AdminHandlers surface
// with a single auth.Service that owns every flow (signup, verification,
// login, logout, forgot/reset/change-password, invitations, change-email).
//
// CookieSecure controls the Secure flag on the session cookie. AdminAPI
// is the Sprint 008 admin chrome (overview / boats / users / trips), kept
// untouched through this swap.
type Server struct {
	Org          *org.Service
	Log          *slog.Logger
	Auth         *auth.Service
	Session      *auth.SessionMiddleware
	GuestSession *auth.GuestSessionMiddleware
	AdminAPI     *AdminHandlers
	ImportRunner *imports.Runner
	DocumentsDir string
	CookieSecure bool
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(slogRequestLogger(s.Log))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(15 * time.Second))

	r.Route("/api", func(r chi.Router) {
		// Public auth surface — no session required.
		r.Post("/auth/signup", s.handleSignup)
		r.Post("/auth/verify-email", s.handleVerifyEmail)
		r.Post("/auth/resend-verification", s.handleResendVerification)
		r.Post("/auth/login", s.handleLogin)
		r.Post("/auth/forgot-password", s.handleForgotPassword)
		r.Post("/auth/reset-password", s.handleResetPassword)

		// Public invitation accept (token-bearing, no cookie).
		r.Get("/invitations/lookup", s.handleLookupInvitation)
		r.Post("/invitations/accept", s.handleAcceptInvitation)
		r.Get("/guest/invitations/{token}", s.handleLookupGuestInvitation)
		r.Post("/guest/invitations/{token}/accept", s.handleAcceptGuestInvitation)

		// Logout works whether you have a valid cookie or not — it always
		// returns 200 and best-effort deletes the row.
		r.Post("/auth/logout", s.handleLogout)

		r.Group(func(r chi.Router) {
			r.Use(s.guestSessionMiddleware().Wrap)
			r.Post("/guest/logout", s.handleGuestLogout)
			r.Get("/guest/trip-registrations/{trip_guest_id}", s.handleGetGuestRegistration)
			r.Patch("/guest/trip-registrations/{trip_guest_id}", s.handleSaveGuestRegistration)
			r.Post("/guest/trip-registrations/{trip_guest_id}/submit", s.handleSubmitGuestRegistration)
			r.Get("/guest/trip-registrations/{trip_guest_id}/documents", s.handleListGuestDocuments)
			r.Post("/guest/trip-registrations/{trip_guest_id}/documents", s.handleUploadGuestDocument)
			r.Get("/guest/trip-registrations/{trip_guest_id}/documents/{document_id}", s.handleOpenGuestDocument)
		})

		// Authenticated routes.
		r.Group(func(r chi.Router) {
			r.Use(s.Session.Wrap)

			r.Get("/me", s.handleMe)
			r.Get("/organization", s.handleOrganization)

			// Account self-service.
			r.Patch("/account/profile", s.handleUpdateProfile)
			r.Post("/account/change-password", s.handleChangePassword)
			r.Post("/account/request-email-change", s.handleRequestEmailChange)
			r.Get("/account/pending-email-change", s.handlePendingEmailChange)
			r.Post("/account/cancel-email-change", s.handleCancelEmailChange)
			r.Post("/checkout/quote", s.handleCheckoutQuote)

			// Confirm-email-change: authenticated when the user clicks
			// the link in the same browser; if not we still accept it
			// (the service falls back to "delete all sessions").
			r.Post("/account/confirm-email-change", s.handleConfirmEmailChange)

			// Org-admin-only invitation routes.
			r.Group(func(r chi.Router) {
				r.Use(auth.RequireOrgAdmin)
				r.Get("/invitations", s.handleListInvitations)
				r.Post("/invitations", s.handleCreateInvitation)
				r.Post("/invitations/{id}/resend", s.handleResendInvitation)
				r.Delete("/invitations/{id}", s.handleRevokeInvitation)
			})

			// Sprint 008 admin chrome surface; Sprint 010 adds the
			// cruise-director-overview endpoint and renames the trip
			// assignment handler.
			r.Route("/admin", func(r chi.Router) {
				r.Get("/trips", s.AdminAPI.HandleListTrips)
				r.Get("/trips/{id}/lifecycle", s.handleTripLifecycle)
				r.Post("/trips/{id}/start", s.handleStartTrip)
				r.Post("/trips/{id}/complete", s.handleCompleteTrip)
				r.Post("/trips/{id}/cancel", s.handleCancelTrip)
				r.Get("/trips/{id}/ledger", s.handleTripLedger)
				r.Post("/trips/{id}/ledger/lines", s.handleAddTripLedgerLine)
				r.Get("/trips/{id}/manifest", s.handleTripManifest)
				r.Get("/trips/{id}/cabins", s.handleTripCabinBoard)
				r.Post("/trips/{id}/guests", s.handleAddTripGuest)
				r.Post("/trips/{id}/guests/{guest_id}/resend", s.handleResendTripGuestInvite)
				r.Delete("/trips/{id}/guests/{guest_id}/invite", s.handleRevokeTripGuestInvite)
				r.Put("/trips/{id}/guests/{guest_id}/cabin-assignment", s.handleAssignGuestCabin)
				r.Delete("/trips/{id}/guests/{guest_id}/cabin-assignment", s.handleUnassignGuestCabin)
				r.Get("/trips/{id}/guests/{guest_id}/registration", s.handleStaffGuestRegistration)
				r.Get("/trips/{id}/guests/{guest_id}/documents", s.handleListStaffGuestDocuments)
				r.Post("/trips/{id}/guests/{guest_id}/documents", s.handleUploadStaffGuestDocument)
				r.Get("/trips/{id}/guests/{guest_id}/documents/{document_id}", s.handleOpenStaffGuestDocument)
				r.Delete("/trips/{id}/guests/{guest_id}/documents/{document_id}", s.handleArchiveStaffGuestDocument)
				r.Get("/trips/{id}/guests/{guest_id}/activity", s.handleGuestActivity)
				r.Get("/trips/{id}/guests/{guest_id}/folio", s.handleGetGuestFolio)
				r.Post("/trips/{id}/guests/{guest_id}/folio", s.handleOpenGuestFolio)
				r.Post("/trips/{id}/guests/{guest_id}/folio/lines", s.handleAddGuestFolioLine)
				r.Patch("/trips/{id}/guests/{guest_id}/folio/lines/{line_id}", s.handleUpdateGuestFolioLine)
				r.Delete("/trips/{id}/guests/{guest_id}/folio/lines/{line_id}", s.handleDeleteGuestFolioLine)
				r.Post("/trips/{id}/guests/{guest_id}/folio/close", s.handleCloseGuestFolio)
				r.Post("/trips/{id}/guests/{guest_id}/folio/resend-email", s.handleResendGuestFolioEmail)
				r.Get("/boats/{id}/cabins", s.handleGetBoatCabins)
				r.Post("/boats/{id}/cabins/preview", s.handlePreviewBoatCabins)
				r.Put("/boats/{id}/cabins", s.handleReplaceBoatCabins)
				r.Patch("/boats/{id}/cabins/{cabin_id}", s.handlePatchBoatCabin)
				r.Delete("/boats/{id}/cabins/{cabin_id}", s.handleDeleteBoatCabin)
				r.Patch("/boats/{id}/cabins/{cabin_id}/berths/{berth_id}", s.handlePatchBoatBerth)
				r.Delete("/boats/{id}/cabins/{cabin_id}/berths/{berth_id}", s.handleDeleteBoatBerth)

				// Cruise-director-only landing payload (profile + stats
				// + trips). The handler enforces the role itself; we
				// mount it inside the authenticated group, not the
				// admin-only group.
				r.Get("/cruise-director-overview", s.handleCruiseDirectorOverview)
				r.Get("/audit-events", s.handleAuditEvents)

				r.Group(func(r chi.Router) {
					r.Use(auth.RequireOrgAdmin)
					r.Get("/overview", s.AdminAPI.HandleOverview)
					r.Get("/organization/payment-settings", s.handleGetPaymentSettings)
					r.Patch("/organization/payment-settings", s.handleUpdatePaymentSettings)
					r.Get("/boats", s.AdminAPI.HandleListBoats)
					r.Get("/boats/{id}", s.AdminAPI.HandleGetBoat)
					r.Get("/boats/{id}/trips", s.AdminAPI.HandleListBoatTrips)
					r.Get("/boats/{id}/inventory", s.handleListBoatInventory)
					r.Get("/users", s.AdminAPI.HandleListUsers)
					r.Get("/catalog/categories", s.handleListCatalogCategories)
					r.Post("/catalog/categories", s.handleCreateCatalogCategory)
					r.Patch("/catalog/categories/{id}", s.handleUpdateCatalogCategory)
					r.Get("/catalog/items", s.handleListCatalogItems)
					r.Post("/catalog/items", s.handleCreateCatalogItem)
					r.Patch("/catalog/items/{id}", s.handleUpdateCatalogItem)
					r.Post("/catalog/defaults/apply", s.handleApplyCatalogDefaults)
					r.Get("/pricing/overrides", s.handleListPriceOverrides)
					r.Put("/pricing/boat-overrides", s.handleUpsertBoatPriceOverride)
					r.Put("/pricing/trip-overrides", s.handleUpsertTripPriceOverride)
					r.Delete("/pricing/overrides/{id}", s.handleArchivePriceOverride)
					r.Get("/inventory/boats", s.handleInventoryBoatSummary)
					r.Put("/boats/{id}/inventory/{item_id}", s.handleSetBoatInventory)
					r.Post("/boats/{id}/inventory/{item_id}/adjustments", s.handleAdjustBoatInventory)
					r.Get("/fx/rates", s.handleListFXRates)
					r.Post("/fx/rates", s.handleCreateFXRate)

					// Sprint 013 — 1:N trip cruise-director assignment.
					// Replaces the Sprint 008/010 PATCH that took a
					// single user id. Each call also dispatches an
					// email notification to the affected director.
					r.Post("/trips/{id}/cruise-directors", s.handleAssignCruiseDirector)
					r.Delete("/trips/{id}/cruise-directors/{user_id}", s.handleUnassignCruiseDirector)

					// Sprint 012 — native trip import. Two paths:
					// liveaboard.com (async via the runner) and
					// spreadsheet upload (sync preview + commit).
					r.Post("/import/liveaboard", s.handleKickLiveaboardImport)
					r.Get("/import/jobs/{id}", s.handleGetImportJob)
					r.Post("/import/spreadsheet/preview", s.handleSpreadsheetPreview)
					r.Post("/import/spreadsheet/commit", s.handleSpreadsheetCommit)
				})
			})

			r.Group(func(r chi.Router) {
				r.Use(auth.RequireOrgAdmin)
				r.Patch("/organization", s.AdminAPI.HandlePatchOrganization)
			})
		})
	})

	// Static SPA + fallback. Anything not under /api falls through here.
	r.Handle("/*", SPAHandler())

	return r
}

// --- core handlers ---

func (s *Server) guestSessionMiddleware() *auth.GuestSessionMiddleware {
	if s.GuestSession != nil {
		return s.GuestSession
	}
	return &auth.GuestSessionMiddleware{Store: s.Auth.Store, Log: s.Log}
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	writeJSON(w, http.StatusOK, authUserView(u))
}

func (s *Server) handleOrganization(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	view, err := s.Org.Dashboard(r.Context(), u.OrganizationID)
	if err != nil {
		s.Log.Error("dashboard failed", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, view)
}

// --- helpers ---

// userFromContext is a thin re-export used by sibling files that don't
// import internal/auth directly.
func userFromContext(ctx interface{ Value(any) any }) *store.User {
	if ctx == nil {
		return nil
	}
	v, _ := ctx.Value(auth.SessionUserKey).(*store.User)
	return v
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error":   code,
		"message": message,
	})
}

func slogRequestLogger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			log.Info("http",
				"method", r.Method,
				"path", redactedRequestPath(r.URL.Path),
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", middleware.GetReqID(r.Context()),
			)
		})
	}
}

func redactedRequestPath(path string) string {
	const prefix = "/api/guest/invitations/"
	if strings.HasPrefix(path, prefix) {
		rest := strings.TrimPrefix(path, prefix)
		if rest == "" {
			return path
		}
		parts := strings.Split(rest, "/")
		parts[0] = "{token}"
		return prefix + strings.Join(parts, "/")
	}
	return path
}
