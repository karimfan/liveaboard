package httpapi_test

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/karimfan/liveaboard/internal/store"
)

func TestGuestFolioCheckoutClosesWithCardFeeStockAndEmail(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	adminCookie, org, _ := signInAsAdmin(t, h)
	tripID := seedManifestTrip(t, h, org.ID, 1)
	markTripActive(t, h, org.ID, tripID)
	boatID := boatIDForTrip(t, h, tripID)
	item := catalogItemByName(t, h, org.ID, "Beer - Can")
	if _, err := h.pool.SetBoatInventoryItem(context.Background(), org.ID, boatID, item.ID, store.InventorySetInput{QuantityOnHand: 5}); err != nil {
		t.Fatalf("SetBoatInventoryItem: %v", err)
	}

	resp, body := doJSON(t, c, "PATCH", h.server.URL+"/api/admin/organization/payment-settings", map[string]any{
		"default_currency":        "USD",
		"supported_currencies":    []string{"USD"},
		"enabled_payment_methods": []string{"card", "cash", "other"},
		"card_fee_basis_points":   300,
		"folio_email_footer":      "Thank you for sailing with us.",
	}, adminCookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("payment settings: %d %v", resp.StatusCode, body)
	}

	resp, body = doJSON(t, c, "POST", h.server.URL+"/api/admin/trips/"+tripID.String()+"/guests", map[string]any{
		"full_name": "Folio Guest",
		"email":     "folio@example.test",
		"berth_id":  nextBerthForTrip(t, h, tripID).String(),
	}, adminCookie)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add guest: %d %v", resp.StatusCode, body)
	}
	guestID, _ := uuid.Parse(body["id"].(string))

	resp, body = doJSON(t, c, "POST", h.server.URL+"/api/admin/trips/"+tripID.String()+"/guests/"+guestID.String()+"/folio", nil, adminCookie)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("open folio: %d %v", resp.StatusCode, body)
	}
	resp, body = doJSON(t, c, "POST", h.server.URL+"/api/admin/trips/"+tripID.String()+"/guests/"+guestID.String()+"/folio/lines", map[string]any{
		"line_type":       "catalog_item",
		"catalog_item_id": item.ID.String(),
		"quantity":        2,
	}, adminCookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("add item: %d %v", resp.StatusCode, body)
	}
	inv, err := h.pool.BoatInventory(context.Background(), org.ID, boatID)
	if err != nil {
		t.Fatalf("BoatInventory after add: %v", err)
	}
	var onHandAfterAdd int
	for _, row := range inv {
		if row.CatalogItemID == item.ID {
			onHandAfterAdd = row.QuantityOnHand
		}
	}
	if onHandAfterAdd != 3 {
		t.Fatalf("on hand after add = %d want 3", onHandAfterAdd)
	}
	resp, body = doJSON(t, c, "POST", h.server.URL+"/api/admin/trips/"+tripID.String()+"/guests/"+guestID.String()+"/folio/lines", map[string]any{
		"line_type":     "crew_tip",
		"tip_usd_cents": 1000,
	}, adminCookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("add tip: %d %v", resp.StatusCode, body)
	}

	resp, body = doJSON(t, c, "POST", h.server.URL+"/api/admin/trips/"+tripID.String()+"/guests/"+guestID.String()+"/folio/close", map[string]any{
		"payment_method":      "card",
		"settlement_currency": "USD",
	}, adminCookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("close: %d %v", resp.StatusCode, body)
	}
	if body["status"] != "closed" || body["subtotal_usd_cents"] != float64(2200) || body["card_fee_usd_cents"] != float64(66) || body["total_usd_cents"] != float64(2266) {
		t.Fatalf("closed totals: %v", body)
	}
	if body["email_send_status"] != "sent" {
		t.Fatalf("email status: %v", body["email_send_status"])
	}
	inv, err = h.pool.BoatInventory(context.Background(), org.ID, boatID)
	if err != nil {
		t.Fatalf("BoatInventory: %v", err)
	}
	var onHand int
	for _, row := range inv {
		if row.CatalogItemID == item.ID {
			onHand = row.QuantityOnHand
		}
	}
	if onHand != 3 {
		t.Fatalf("on hand = %d want 3", onHand)
	}
	if !strings.Contains(h.mail.Last().Subject, "folio") {
		t.Fatalf("folio email not sent: %+v", h.mail.Last())
	}

	resp, _ = doJSON(t, c, "POST", h.server.URL+"/api/admin/trips/"+tripID.String()+"/guests/"+guestID.String()+"/folio/close", map[string]any{
		"payment_method":      "card",
		"settlement_currency": "USD",
	}, adminCookie)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate close: %d want 409", resp.StatusCode)
	}
}

func TestGuestFolioDirectorScopingAndStockRollback(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	adminCookie, org, admin := signInAsAdmin(t, h)
	dirCookie, director := bootstrapDirector(t, h, org.ID)
	tripID := seedManifestTrip(t, h, org.ID, 1)
	markTripActive(t, h, org.ID, tripID)
	boatID := boatIDForTrip(t, h, tripID)
	item := catalogItemByName(t, h, org.ID, "Beer - Can")
	if _, err := h.pool.SetBoatInventoryItem(context.Background(), org.ID, boatID, item.ID, store.InventorySetInput{QuantityOnHand: 1}); err != nil {
		t.Fatalf("SetBoatInventoryItem: %v", err)
	}
	resp, body := doJSON(t, c, "POST", h.server.URL+"/api/admin/trips/"+tripID.String()+"/guests", map[string]any{
		"full_name": "Scoped Guest",
		"email":     "scoped@example.test",
		"berth_id":  nextBerthForTrip(t, h, tripID).String(),
	}, adminCookie)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add guest: %d %v", resp.StatusCode, body)
	}
	guestID, _ := uuid.Parse(body["id"].(string))

	resp, _ = doJSON(t, c, "POST", h.server.URL+"/api/admin/trips/"+tripID.String()+"/guests/"+guestID.String()+"/folio", nil, dirCookie)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("unassigned director open: %d want 403", resp.StatusCode)
	}
	if _, err := h.pool.AssignCruiseDirector(context.Background(), tripID, director.ID, &admin.ID); err != nil {
		t.Fatalf("AssignCruiseDirector: %v", err)
	}
	resp, body = doJSON(t, c, "POST", h.server.URL+"/api/admin/trips/"+tripID.String()+"/guests/"+guestID.String()+"/folio", nil, dirCookie)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("assigned director open: %d %v", resp.StatusCode, body)
	}
	resp, body = doJSON(t, c, "POST", h.server.URL+"/api/admin/trips/"+tripID.String()+"/guests/"+guestID.String()+"/folio/lines", map[string]any{
		"line_type":       "catalog_item",
		"catalog_item_id": item.ID.String(),
		"quantity":        2,
	}, dirCookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("director add line: %d %v", resp.StatusCode, body)
	}
	if warnings, _ := body["warnings"].([]any); len(warnings) == 0 {
		t.Fatalf("expected negative stock warning: %v", body)
	}
	resp, _ = doJSON(t, c, "POST", h.server.URL+"/api/admin/trips/"+tripID.String()+"/guests/"+guestID.String()+"/folio/close", map[string]any{
		"payment_method":      "cash",
		"settlement_currency": "USD",
	}, dirCookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("negative stock close: %d want 200", resp.StatusCode)
	}
	inv, _ := h.pool.BoatInventory(context.Background(), org.ID, boatID)
	for _, row := range inv {
		if row.CatalogItemID == item.ID && row.QuantityOnHand != -1 {
			t.Fatalf("stock after negative add/close: %d want -1", row.QuantityOnHand)
		}
	}
}

func TestTripLedgerLazyOpenIdempotentNegativeStock(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	adminCookie, org, _ := signInAsAdmin(t, h)
	tripID := seedManifestTrip(t, h, org.ID, 1)
	markTripActive(t, h, org.ID, tripID)
	resp, body := doJSON(t, c, "POST", h.server.URL+"/api/admin/trips/"+tripID.String()+"/guests", map[string]any{
		"full_name": "Ledger Guest",
		"email":     "ledger@example.test",
		"berth_id":  nextBerthForTrip(t, h, tripID).String(),
	}, adminCookie)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add guest: %d %v", resp.StatusCode, body)
	}
	guestID, _ := uuid.Parse(body["id"].(string))
	item := catalogItemByName(t, h, org.ID, "Beer - Can")
	resp, body = doJSON(t, c, "GET", h.server.URL+"/api/admin/trips/"+tripID.String()+"/ledger", nil, adminCookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get ledger: %d %v", resp.StatusCode, body)
	}
	requestID := "ledger-test-request"
	payload := map[string]any{
		"trip_guest_id":     guestID.String(),
		"catalog_item_id":   item.ID.String(),
		"quantity":          2,
		"client_request_id": requestID,
	}
	resp, body = doJSON(t, c, "POST", h.server.URL+"/api/admin/trips/"+tripID.String()+"/ledger/lines", payload, adminCookie)
	if resp.StatusCode != http.StatusOK {
		var folioCount, lineCount int
		_ = h.pool.QueryRow(context.Background(), `SELECT count(*) FROM guest_folios WHERE trip_guest_id = $1`, guestID).Scan(&folioCount)
		_ = h.pool.QueryRow(context.Background(), `SELECT count(*) FROM guest_folio_lines WHERE trip_guest_id = $1`, guestID).Scan(&lineCount)
		t.Fatalf("first ledger add: %d %v folios=%d lines=%d", resp.StatusCode, body, folioCount, lineCount)
	}
	if warnings, _ := body["warnings"].([]any); len(warnings) == 0 {
		t.Fatalf("expected negative stock warning: %v", body)
	}
	lines := body["lines"].([]any)
	if len(lines) != 1 {
		t.Fatalf("lines after first add = %d want 1", len(lines))
	}
	lineID := lines[0].(map[string]any)["id"]
	resp, body = doJSON(t, c, "POST", h.server.URL+"/api/admin/trips/"+tripID.String()+"/ledger/lines", payload, adminCookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("retry ledger add: %d %v", resp.StatusCode, body)
	}
	lines = body["lines"].([]any)
	if len(lines) != 1 || lines[0].(map[string]any)["id"] != lineID {
		t.Fatalf("idempotent retry duplicated or changed line: %v", lines)
	}
}

func TestTripLedgerUsesEffectivePriceOverrides(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	adminCookie, org, _ := signInAsAdmin(t, h)
	tripID := seedManifestTrip(t, h, org.ID, 1)
	markTripActive(t, h, org.ID, tripID)
	boatID := boatIDForTrip(t, h, tripID)
	resp, body := doJSON(t, c, "POST", h.server.URL+"/api/admin/trips/"+tripID.String()+"/guests", map[string]any{
		"full_name": "Override Guest",
		"email":     "override@example.test",
		"berth_id":  nextBerthForTrip(t, h, tripID).String(),
	}, adminCookie)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add guest: %d %v", resp.StatusCode, body)
	}
	guestID, _ := uuid.Parse(body["id"].(string))
	item := catalogItemByName(t, h, org.ID, "Beer - Can")

	resp, body = doJSON(t, c, "PUT", h.server.URL+"/api/admin/pricing/boat-overrides", map[string]any{
		"catalog_item_id": item.ID.String(),
		"boat_id":         boatID.String(),
		"price_usd_cents": 900,
		"notes":           "boat bar price",
	}, adminCookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("boat override: %d %v", resp.StatusCode, body)
	}
	resp, body = doJSON(t, c, "PUT", h.server.URL+"/api/admin/pricing/trip-overrides", map[string]any{
		"catalog_item_id": item.ID.String(),
		"trip_id":         tripID.String(),
		"price_usd_cents": 1100,
		"notes":           "trip special",
	}, adminCookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("trip override: %d %v", resp.StatusCode, body)
	}
	resp, body = doJSON(t, c, "GET", h.server.URL+"/api/admin/trips/"+tripID.String()+"/ledger", nil, adminCookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ledger: %d %v", resp.StatusCode, body)
	}
	var seen bool
	for _, raw := range body["catalog"].([]any) {
		row := raw.(map[string]any)
		if row["id"] == item.ID.String() {
			seen = true
			if row["effective_price_usd_cents"] != float64(1100) || row["price_source"] != "trip_override" {
				t.Fatalf("effective ledger item: %v", row)
			}
		}
	}
	if !seen {
		t.Fatalf("catalog item missing from ledger")
	}

	resp, body = doJSON(t, c, "POST", h.server.URL+"/api/admin/trips/"+tripID.String()+"/ledger/lines", map[string]any{
		"trip_guest_id":   guestID.String(),
		"catalog_item_id": item.ID.String(),
		"quantity":        2,
	}, adminCookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("add overridden line: %d %v", resp.StatusCode, body)
	}
	lines := body["lines"].([]any)
	if len(lines) != 1 {
		t.Fatalf("line count = %d want 1", len(lines))
	}
	line := lines[0].(map[string]any)
	if line["unit_price_usd_cents"] != float64(1100) || line["line_total_usd_cents"] != float64(2200) || line["price_source"] != "trip_override" {
		t.Fatalf("line did not snapshot trip override: %v", line)
	}

	resp, body = doJSON(t, c, "PUT", h.server.URL+"/api/admin/pricing/trip-overrides", map[string]any{
		"catalog_item_id": item.ID.String(),
		"trip_id":         tripID.String(),
		"price_usd_cents": 1300,
		"notes":           "changed",
	}, adminCookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update trip override: %d %v", resp.StatusCode, body)
	}
	resp, body = doJSON(t, c, "GET", h.server.URL+"/api/admin/trips/"+tripID.String()+"/guests/"+guestID.String()+"/folio", nil, adminCookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("folio after override change: %d %v", resp.StatusCode, body)
	}
	line = body["lines"].([]any)[0].(map[string]any)
	if line["unit_price_usd_cents"] != float64(1100) {
		t.Fatalf("historical line was repriced: %v", line)
	}
}

func TestPaymentSettingsDefaultsEURButAllowsRemoval(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	adminCookie, _, _ := signInAsAdmin(t, h)

	resp, body := doJSON(t, c, "GET", h.server.URL+"/api/admin/organization/payment-settings", nil, adminCookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("settings: %d %v", resp.StatusCode, body)
	}
	if !containsString(body["supported_currencies"].([]any), "USD") || !containsString(body["supported_currencies"].([]any), "EUR") {
		t.Fatalf("expected USD and EUR defaults: %v", body["supported_currencies"])
	}

	resp, body = doJSON(t, c, "PATCH", h.server.URL+"/api/admin/organization/payment-settings", map[string]any{
		"default_currency":        "USD",
		"supported_currencies":    []string{"USD"},
		"enabled_payment_methods": []string{"card", "cash"},
		"card_fee_basis_points":   0,
		"folio_email_footer":      nil,
	}, adminCookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("remove EUR: %d %v", resp.StatusCode, body)
	}
	if containsString(body["supported_currencies"].([]any), "EUR") {
		t.Fatalf("EUR should be removable: %v", body["supported_currencies"])
	}
	resp, body = doJSON(t, c, "GET", h.server.URL+"/api/admin/organization/payment-settings", nil, adminCookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("settings after removal: %d %v", resp.StatusCode, body)
	}
	if containsString(body["supported_currencies"].([]any), "EUR") {
		t.Fatalf("EnsurePaymentSettings re-added EUR: %v", body["supported_currencies"])
	}
}

func TestTripLedgerConcurrentLazyOpenSameRequest(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	adminCookie, org, _ := signInAsAdmin(t, h)
	tripID := seedManifestTrip(t, h, org.ID, 1)
	markTripActive(t, h, org.ID, tripID)
	resp, body := doJSON(t, c, "POST", h.server.URL+"/api/admin/trips/"+tripID.String()+"/guests", map[string]any{
		"full_name": "Concurrent Guest",
		"email":     "concurrent@example.test",
		"berth_id":  nextBerthForTrip(t, h, tripID).String(),
	}, adminCookie)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add guest: %d %v", resp.StatusCode, body)
	}
	guestID, _ := uuid.Parse(body["id"].(string))
	item := catalogItemByName(t, h, org.ID, "Beer - Can")
	payload := map[string]any{
		"trip_guest_id":     guestID.String(),
		"catalog_item_id":   item.ID.String(),
		"quantity":          1,
		"client_request_id": "concurrent-request",
	}
	var wg sync.WaitGroup
	statuses := make([]int, 2)
	for i := range statuses {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			resp, _ := doJSON(t, c, "POST", h.server.URL+"/api/admin/trips/"+tripID.String()+"/ledger/lines", payload, adminCookie)
			statuses[i] = resp.StatusCode
		}(i)
	}
	wg.Wait()
	for _, status := range statuses {
		if status != http.StatusOK {
			t.Fatalf("concurrent statuses = %v", statuses)
		}
	}
	var folioCount, lineCount int
	_ = h.pool.QueryRow(context.Background(), `SELECT count(*) FROM guest_folios WHERE trip_guest_id = $1`, guestID).Scan(&folioCount)
	_ = h.pool.QueryRow(context.Background(), `SELECT count(*) FROM guest_folio_lines WHERE trip_guest_id = $1`, guestID).Scan(&lineCount)
	if folioCount != 1 || lineCount != 1 {
		t.Fatalf("concurrent lazy-open created folios=%d lines=%d; want 1/1", folioCount, lineCount)
	}
}

func boatIDForTrip(t *testing.T, h *harness, tripID uuid.UUID) uuid.UUID {
	t.Helper()
	var boatID uuid.UUID
	if err := h.pool.QueryRow(context.Background(), `SELECT boat_id FROM trips WHERE id = $1`, tripID).Scan(&boatID); err != nil {
		t.Fatalf("boatIDForTrip: %v", err)
	}
	return boatID
}

func containsString(items []any, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func markTripActive(t *testing.T, h *harness, orgID, tripID uuid.UUID) {
	t.Helper()
	if _, err := h.pool.Exec(context.Background(), `UPDATE trips SET status = 'active', started_at = now() WHERE organization_id = $1 AND id = $2`, orgID, tripID); err != nil {
		t.Fatalf("markTripActive: %v", err)
	}
}

func catalogItemByName(t *testing.T, h *harness, orgID uuid.UUID, name string) *store.CatalogItem {
	t.Helper()
	items, err := h.pool.CatalogItems(context.Background(), orgID)
	if err != nil {
		t.Fatalf("CatalogItems: %v", err)
	}
	for _, item := range items {
		if item.Name == name {
			return item
		}
	}
	t.Fatalf("catalog item %q not found", name)
	return nil
}
