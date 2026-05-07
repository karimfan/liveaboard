package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	MovementInitialCount = "initial_count"
	MovementRestock      = "restock"
	MovementCorrection   = "correction"
	MovementBreakage     = "breakage"
	MovementSpoilage     = "spoilage"
	MovementInternalUse  = "internal_use"
	MovementFolioCharge  = "folio_charge"
	MovementFolioVoid    = "folio_void"
)

type BoatInventoryItem struct {
	ID               uuid.UUID
	OrganizationID   uuid.UUID
	BoatID           uuid.UUID
	CatalogItemID    uuid.UUID
	QuantityOnHand   int
	QuantityReserved int
	ReorderLevel     *int
	ParLevel         *int
	LastCountedAt    *time.Time
	Notes            *string
	CreatedAt        time.Time
	UpdatedAt        time.Time

	ItemName      string
	CategoryName  string
	Unit          string
	StockMode     string
	PriceUSDCents int64
	Status        string
}

type StockMovement struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	BoatID         uuid.UUID
	CatalogItemID  uuid.UUID
	ActorUserID    *uuid.UUID
	MovementType   string
	DeltaQuantity  int
	QuantityBefore int
	QuantityAfter  int
	SourceType     *string
	SourceID       *uuid.UUID
	Note           *string
	CreatedAt      time.Time
}

type InventorySetInput struct {
	QuantityOnHand int
	ReorderLevel   *int
	ParLevel       *int
	Notes          *string
}

type StockAdjustmentInput struct {
	ActorUserID   *uuid.UUID
	MovementType  string
	DeltaQuantity int
	SourceType    *string
	SourceID      *uuid.UUID
	Note          *string
}

const inventorySelect = `bi.id, bi.organization_id, bi.boat_id, bi.catalog_item_id,
	bi.quantity_on_hand, bi.quantity_reserved, bi.reorder_level, bi.par_level,
	bi.last_counted_at, bi.notes, bi.created_at, bi.updated_at,
	i.name, c.name, i.unit, i.stock_mode, i.price_usd_cents`

func scanInventoryItem(row interface{ Scan(dest ...any) error }, it *BoatInventoryItem) error {
	if err := row.Scan(
		&it.ID, &it.OrganizationID, &it.BoatID, &it.CatalogItemID,
		&it.QuantityOnHand, &it.QuantityReserved, &it.ReorderLevel, &it.ParLevel,
		&it.LastCountedAt, &it.Notes, &it.CreatedAt, &it.UpdatedAt,
		&it.ItemName, &it.CategoryName, &it.Unit, &it.StockMode, &it.PriceUSDCents,
	); err != nil {
		return err
	}
	it.Status = inventoryStatus(it.QuantityOnHand, it.ReorderLevel)
	return nil
}

func scanStockMovement(row interface{ Scan(dest ...any) error }, m *StockMovement) error {
	return row.Scan(&m.ID, &m.OrganizationID, &m.BoatID, &m.CatalogItemID, &m.ActorUserID, &m.MovementType, &m.DeltaQuantity, &m.QuantityBefore, &m.QuantityAfter, &m.SourceType, &m.SourceID, &m.Note, &m.CreatedAt)
}

func inventoryStatus(onHand int, reorder *int) string {
	if onHand == 0 {
		return "out"
	}
	if reorder != nil && onHand <= *reorder {
		return "low"
	}
	return "ok"
}

func (p *Pool) BoatInventory(ctx context.Context, orgID, boatID uuid.UUID) ([]*BoatInventoryItem, error) {
	if _, err := p.BoatByID(ctx, orgID, boatID); err != nil {
		return nil, err
	}
	rows, err := p.Query(ctx, `
		SELECT `+inventorySelect+`
		FROM boat_inventory_items bi
		JOIN catalog_items i ON i.id = bi.catalog_item_id
		JOIN catalog_categories c ON c.id = i.category_id
		WHERE bi.organization_id = $1 AND bi.boat_id = $2
		ORDER BY c.sort_order, lower(c.name), lower(i.name)
	`, orgID, boatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*BoatInventoryItem
	for rows.Next() {
		it := &BoatInventoryItem{}
		if err := scanInventoryItem(rows, it); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (p *Pool) InventoryFleetSummary(ctx context.Context, orgID uuid.UUID) ([]map[string]any, error) {
	rows, err := p.Query(ctx, `
		SELECT b.id, b.display_name,
		       count(bi.id) FILTER (WHERE bi.quantity_on_hand = 0)::int AS out_count,
		       count(bi.id) FILTER (WHERE bi.quantity_on_hand > 0 AND bi.reorder_level IS NOT NULL AND bi.quantity_on_hand <= bi.reorder_level)::int AS low_count
		FROM boats b
		LEFT JOIN boat_inventory_items bi ON bi.boat_id = b.id AND bi.organization_id = b.organization_id
		WHERE b.organization_id = $1
		GROUP BY b.id, b.display_name
		ORDER BY b.display_name
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id uuid.UUID
		var name string
		var outCount, lowCount int
		if err := rows.Scan(&id, &name, &outCount, &lowCount); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"boat_id":         id,
			"boat_name":       name,
			"out_stock_count": outCount,
			"low_stock_count": lowCount,
		})
	}
	return out, rows.Err()
}

func (p *Pool) SetBoatInventoryItem(ctx context.Context, orgID, boatID, itemID uuid.UUID, in InventorySetInput) (*BoatInventoryItem, error) {
	if in.QuantityOnHand < 0 {
		return nil, errors.New("quantity_on_hand must be non-negative")
	}
	if err := p.validateBoatAndCountedItem(ctx, orgID, boatID, itemID); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	it := &BoatInventoryItem{}
	err := scanInventoryItem(p.QueryRow(ctx, `
		WITH upserted AS (
			INSERT INTO boat_inventory_items (
				organization_id, boat_id, catalog_item_id, quantity_on_hand,
				reorder_level, par_level, last_counted_at, notes
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (organization_id, boat_id, catalog_item_id) DO UPDATE SET
				quantity_on_hand = EXCLUDED.quantity_on_hand,
				reorder_level = EXCLUDED.reorder_level,
				par_level = EXCLUDED.par_level,
				last_counted_at = EXCLUDED.last_counted_at,
				notes = EXCLUDED.notes,
				updated_at = now()
			RETURNING *
		)
		SELECT `+inventorySelect+`
		FROM upserted bi
		JOIN catalog_items i ON i.id = bi.catalog_item_id
		JOIN catalog_categories c ON c.id = i.category_id
	`, orgID, boatID, itemID, in.QuantityOnHand, in.ReorderLevel, in.ParLevel, now, in.Notes), it)
	if err != nil {
		return nil, err
	}
	return it, nil
}

func (p *Pool) AdjustStock(ctx context.Context, orgID, boatID, itemID uuid.UUID, in StockAdjustmentInput) (*StockMovement, *BoatInventoryItem, error) {
	if err := validateMovementType(in.MovementType); err != nil {
		return nil, nil, err
	}
	if in.DeltaQuantity == 0 {
		return nil, nil, errors.New("delta_quantity must be non-zero")
	}
	if err := p.validateBoatAndCountedItem(ctx, orgID, boatID, itemID); err != nil {
		return nil, nil, err
	}
	tx, err := p.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)

	var before int
	err = tx.QueryRow(ctx, `
		SELECT quantity_on_hand
		FROM boat_inventory_items
		WHERE organization_id = $1 AND boat_id = $2 AND catalog_item_id = $3
		FOR UPDATE
	`, orgID, boatID, itemID).Scan(&before)
	if isNoRows(err) {
		before = 0
		_, err = tx.Exec(ctx, `
			INSERT INTO boat_inventory_items (organization_id, boat_id, catalog_item_id, quantity_on_hand)
			VALUES ($1, $2, $3, 0)
		`, orgID, boatID, itemID)
		if err != nil {
			return nil, nil, err
		}
		// Lock the row we just created for the same transaction.
		if err := tx.QueryRow(ctx, `
			SELECT quantity_on_hand FROM boat_inventory_items
			WHERE organization_id = $1 AND boat_id = $2 AND catalog_item_id = $3
			FOR UPDATE
		`, orgID, boatID, itemID).Scan(&before); err != nil {
			return nil, nil, err
		}
	} else if err != nil {
		return nil, nil, err
	}

	after := before + in.DeltaQuantity
	if after < 0 {
		return nil, nil, errors.New("stock adjustment would make quantity negative")
	}

	_, err = tx.Exec(ctx, `
		UPDATE boat_inventory_items
		SET quantity_on_hand = $4, updated_at = now()
		WHERE organization_id = $1 AND boat_id = $2 AND catalog_item_id = $3
	`, orgID, boatID, itemID, after)
	if err != nil {
		return nil, nil, err
	}
	mv := &StockMovement{}
	err = scanStockMovement(tx.QueryRow(ctx, `
		INSERT INTO stock_movements (
			organization_id, boat_id, catalog_item_id, actor_user_id, movement_type,
			delta_quantity, quantity_before, quantity_after, source_type, source_id, note
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id, organization_id, boat_id, catalog_item_id, actor_user_id,
			movement_type, delta_quantity, quantity_before, quantity_after,
			source_type, source_id, note, created_at
	`, orgID, boatID, itemID, in.ActorUserID, in.MovementType, in.DeltaQuantity, before, after, in.SourceType, in.SourceID, in.Note), mv)
	if err != nil {
		return nil, nil, err
	}
	it := &BoatInventoryItem{}
	err = scanInventoryItem(tx.QueryRow(ctx, `
		SELECT `+inventorySelect+`
		FROM boat_inventory_items bi
		JOIN catalog_items i ON i.id = bi.catalog_item_id
		JOIN catalog_categories c ON c.id = i.category_id
		WHERE bi.organization_id = $1 AND bi.boat_id = $2 AND bi.catalog_item_id = $3
	`, orgID, boatID, itemID), it)
	if err != nil {
		return nil, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	return mv, it, nil
}

func (p *Pool) validateBoatAndCountedItem(ctx context.Context, orgID, boatID, itemID uuid.UUID) error {
	if _, err := p.BoatByID(ctx, orgID, boatID); err != nil {
		return err
	}
	item, err := p.CatalogItemByID(ctx, orgID, itemID)
	if err != nil {
		return err
	}
	if item.ArchivedAt != nil {
		return ErrNotFound
	}
	if item.StockMode != StockModeCounted {
		return errors.New("catalog item is not stock-tracked")
	}
	return nil
}

func validateMovementType(t string) error {
	switch strings.TrimSpace(t) {
	case MovementInitialCount, MovementRestock, MovementCorrection, MovementBreakage, MovementSpoilage, MovementInternalUse, MovementFolioCharge, MovementFolioVoid:
		return nil
	default:
		return fmt.Errorf("invalid movement_type %q", t)
	}
}
