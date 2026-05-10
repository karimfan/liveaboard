package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	ChargeTypeSale      = "sale"
	ChargeTypeRental    = "rental"
	ChargeTypeService   = "service"
	ChargeTypeFee       = "fee"
	ChargeTypeGratuity  = "gratuity"
	ChargeTypeDeposit   = "deposit"
	ChargeTypeDamage    = "damage"
	ChargeTypeIncluded  = "included"
	StockModeNone       = "none"
	StockModeCounted    = "counted"
	defaultTemplateFlag = true
)

type CatalogCategory struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	TemplateKey    *string
	Name           string
	SortOrder      int
	IsDefaultSeed  bool
	ArchivedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	ItemCount      int
}

type CatalogItem struct {
	ID                     uuid.UUID
	OrganizationID         uuid.UUID
	CategoryID             uuid.UUID
	TemplateKey            *string
	Name                   string
	Description            *string
	Unit                   string
	ChargeType             string
	StockMode              string
	PriceUSDCents          int64
	IsTaxable              bool
	IsRequiredFee          bool
	IsActive               bool
	ArchivedAt             *time.Time
	CreatedAt              time.Time
	UpdatedAt              time.Time
	CategoryName           string
	EffectivePriceUSDCents *int64
	PriceSource            string
	PriceOverrideID        *uuid.UUID
}

type CatalogItemInput struct {
	CategoryID    uuid.UUID
	Name          string
	Description   *string
	Unit          string
	ChargeType    string
	StockMode     string
	PriceUSDCents int64
	IsTaxable     bool
	IsRequiredFee bool
	IsActive      bool
}

type defaultCategorySeed struct {
	Key       string
	Name      string
	SortOrder int
}

type defaultItemSeed struct {
	Key           string
	CategoryKey   string
	Name          string
	Unit          string
	ChargeType    string
	StockMode     string
	PriceUSDCents int64
	IsTaxable     bool
	IsRequiredFee bool
}

var defaultCategories = []defaultCategorySeed{
	{Key: "bar", Name: "Bar", SortOrder: 10},
	{Key: "soft_drinks", Name: "Soft Drinks", SortOrder: 20},
	{Key: "dive_services", Name: "Dive Services", SortOrder: 30},
	{Key: "gear_rental", Name: "Gear Rental", SortOrder: 40},
	{Key: "retail", Name: "Retail", SortOrder: 50},
	{Key: "spa", Name: "Spa", SortOrder: 60},
	{Key: "laundry", Name: "Laundry", SortOrder: 70},
	{Key: "connectivity", Name: "Connectivity", SortOrder: 80},
	{Key: "fees", Name: "Fees", SortOrder: 90},
	{Key: "gratuities", Name: "Gratuities", SortOrder: 100},
	{Key: "miscellaneous", Name: "Miscellaneous", SortOrder: 110},
}

var defaultItems = []defaultItemSeed{
	{Key: "beer_can", CategoryKey: "bar", Name: "Beer - Can", Unit: "can", ChargeType: ChargeTypeSale, StockMode: StockModeCounted, PriceUSDCents: 600, IsTaxable: true},
	{Key: "beer_bottle", CategoryKey: "bar", Name: "Beer - Bottle", Unit: "bottle", ChargeType: ChargeTypeSale, StockMode: StockModeCounted, PriceUSDCents: 700, IsTaxable: true},
	{Key: "house_wine_glass", CategoryKey: "bar", Name: "House Wine - Glass", Unit: "glass", ChargeType: ChargeTypeSale, StockMode: StockModeNone, PriceUSDCents: 800, IsTaxable: true},
	{Key: "house_wine_bottle", CategoryKey: "bar", Name: "House Wine - Bottle", Unit: "bottle", ChargeType: ChargeTypeSale, StockMode: StockModeCounted, PriceUSDCents: 3800, IsTaxable: true},
	{Key: "premium_wine_bottle", CategoryKey: "bar", Name: "Premium Wine - Bottle", Unit: "bottle", ChargeType: ChargeTypeSale, StockMode: StockModeCounted, PriceUSDCents: 6500, IsTaxable: true},
	{Key: "cocktail", CategoryKey: "bar", Name: "Cocktail", Unit: "each", ChargeType: ChargeTypeSale, StockMode: StockModeNone, PriceUSDCents: 1200, IsTaxable: true},
	{Key: "corkage_wine_bottle", CategoryKey: "bar", Name: "Corkage - Wine Bottle", Unit: "bottle", ChargeType: ChargeTypeFee, StockMode: StockModeNone, PriceUSDCents: 2000},
	{Key: "soft_drink_can", CategoryKey: "soft_drinks", Name: "Soft Drink - Can", Unit: "can", ChargeType: ChargeTypeSale, StockMode: StockModeCounted, PriceUSDCents: 300, IsTaxable: true},
	{Key: "fresh_juice", CategoryKey: "soft_drinks", Name: "Fresh Juice", Unit: "glass", ChargeType: ChargeTypeSale, StockMode: StockModeNone, PriceUSDCents: 500, IsTaxable: true},
	{Key: "nitrox_fill", CategoryKey: "dive_services", Name: "Nitrox - Fill", Unit: "fill", ChargeType: ChargeTypeService, StockMode: StockModeNone, PriceUSDCents: 1000},
	{Key: "nitrox_day", CategoryKey: "dive_services", Name: "Nitrox - Day", Unit: "day", ChargeType: ChargeTypeService, StockMode: StockModeNone, PriceUSDCents: 2000},
	{Key: "nitrox_week", CategoryKey: "dive_services", Name: "Nitrox - Week", Unit: "week", ChargeType: ChargeTypeService, StockMode: StockModeNone, PriceUSDCents: 15000},
	{Key: "extra_dive", CategoryKey: "dive_services", Name: "Extra Dive", Unit: "each", ChargeType: ChargeTypeService, StockMode: StockModeNone, PriceUSDCents: 4500},
	{Key: "full_rental_kit", CategoryKey: "gear_rental", Name: "Full Rental Kit", Unit: "trip", ChargeType: ChargeTypeRental, StockMode: StockModeNone, PriceUSDCents: 17500},
	{Key: "bcd_rental", CategoryKey: "gear_rental", Name: "BCD Rental", Unit: "trip", ChargeType: ChargeTypeRental, StockMode: StockModeCounted, PriceUSDCents: 6000},
	{Key: "regulator_rental", CategoryKey: "gear_rental", Name: "Regulator Rental", Unit: "trip", ChargeType: ChargeTypeRental, StockMode: StockModeCounted, PriceUSDCents: 6000},
	{Key: "dive_computer_rental", CategoryKey: "gear_rental", Name: "Dive Computer Rental", Unit: "trip", ChargeType: ChargeTypeRental, StockMode: StockModeCounted, PriceUSDCents: 6000},
	{Key: "wetsuit_rental", CategoryKey: "gear_rental", Name: "Wetsuit Rental", Unit: "trip", ChargeType: ChargeTypeRental, StockMode: StockModeCounted, PriceUSDCents: 5000},
	{Key: "night_light_rental", CategoryKey: "gear_rental", Name: "Night Light Rental", Unit: "night", ChargeType: ChargeTypeRental, StockMode: StockModeCounted, PriceUSDCents: 800},
	{Key: "t_shirt", CategoryKey: "retail", Name: "T-Shirt", Unit: "each", ChargeType: ChargeTypeSale, StockMode: StockModeCounted, PriceUSDCents: 2500, IsTaxable: true},
	{Key: "hoodie", CategoryKey: "retail", Name: "Hoodie", Unit: "each", ChargeType: ChargeTypeSale, StockMode: StockModeCounted, PriceUSDCents: 5500, IsTaxable: true},
	{Key: "mug", CategoryKey: "retail", Name: "Mug", Unit: "each", ChargeType: ChargeTypeSale, StockMode: StockModeCounted, PriceUSDCents: 1800, IsTaxable: true},
	{Key: "rash_guard", CategoryKey: "retail", Name: "Rash Guard", Unit: "each", ChargeType: ChargeTypeSale, StockMode: StockModeCounted, PriceUSDCents: 4500, IsTaxable: true},
	{Key: "reef_safe_sunscreen", CategoryKey: "retail", Name: "Reef-Safe Sunscreen", Unit: "each", ChargeType: ChargeTypeSale, StockMode: StockModeCounted, PriceUSDCents: 1800, IsTaxable: true},
	{Key: "dry_bag", CategoryKey: "retail", Name: "Dry Bag", Unit: "each", ChargeType: ChargeTypeSale, StockMode: StockModeCounted, PriceUSDCents: 3000, IsTaxable: true},
	{Key: "massage_30", CategoryKey: "spa", Name: "Massage - 30 Minutes", Unit: "session", ChargeType: ChargeTypeService, StockMode: StockModeNone, PriceUSDCents: 4500},
	{Key: "massage_60", CategoryKey: "spa", Name: "Massage - 60 Minutes", Unit: "session", ChargeType: ChargeTypeService, StockMode: StockModeNone, PriceUSDCents: 8500},
	{Key: "laundry_item", CategoryKey: "laundry", Name: "Laundry - Item", Unit: "item", ChargeType: ChargeTypeService, StockMode: StockModeNone, PriceUSDCents: 500},
	{Key: "laundry_bag", CategoryKey: "laundry", Name: "Laundry - Bag", Unit: "bag", ChargeType: ChargeTypeService, StockMode: StockModeNone, PriceUSDCents: 2500},
	{Key: "wifi_week", CategoryKey: "connectivity", Name: "WiFi - Week Code", Unit: "week", ChargeType: ChargeTypeService, StockMode: StockModeNone, PriceUSDCents: 3000},
	{Key: "marine_park_fee", CategoryKey: "fees", Name: "Marine Park Fee", Unit: "person", ChargeType: ChargeTypeFee, StockMode: StockModeNone, PriceUSDCents: 15000, IsRequiredFee: true},
	{Key: "fuel_surcharge", CategoryKey: "fees", Name: "Fuel Surcharge", Unit: "person", ChargeType: ChargeTypeFee, StockMode: StockModeNone, PriceUSDCents: 15000},
	{Key: "airport_transfer", CategoryKey: "fees", Name: "Airport Transfer", Unit: "person", ChargeType: ChargeTypeService, StockMode: StockModeNone, PriceUSDCents: 2500},
	{Key: "crew_gratuity", CategoryKey: "gratuities", Name: "Crew Gratuity", Unit: "person", ChargeType: ChargeTypeGratuity, StockMode: StockModeNone, PriceUSDCents: 15000},
	{Key: "custom_charge", CategoryKey: "miscellaneous", Name: "Custom Charge", Unit: "each", ChargeType: ChargeTypeSale, StockMode: StockModeNone, PriceUSDCents: 0},
}

func scanCatalogCategory(row interface{ Scan(dest ...any) error }, c *CatalogCategory) error {
	return row.Scan(&c.ID, &c.OrganizationID, &c.TemplateKey, &c.Name, &c.SortOrder, &c.IsDefaultSeed, &c.ArchivedAt, &c.CreatedAt, &c.UpdatedAt, &c.ItemCount)
}

func scanCatalogItem(row interface{ Scan(dest ...any) error }, i *CatalogItem) error {
	return row.Scan(&i.ID, &i.OrganizationID, &i.CategoryID, &i.TemplateKey, &i.Name, &i.Description, &i.Unit, &i.ChargeType, &i.StockMode, &i.PriceUSDCents, &i.IsTaxable, &i.IsRequiredFee, &i.IsActive, &i.ArchivedAt, &i.CreatedAt, &i.UpdatedAt, &i.CategoryName)
}

const catalogCategorySelect = `c.id, c.organization_id, c.template_key, c.name, c.sort_order, c.is_default_seed, c.archived_at, c.created_at, c.updated_at, count(i.id)::int`
const catalogItemSelect = `i.id, i.organization_id, i.category_id, i.template_key, i.name, i.description, i.unit, i.charge_type, i.stock_mode, i.price_usd_cents, i.is_taxable, i.is_required_fee, i.is_active, i.archived_at, i.created_at, i.updated_at, c.name`

func (p *Pool) SeedDefaultCatalog(ctx context.Context, orgID uuid.UUID) error {
	tx, err := p.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := seedDefaultCatalogTx(ctx, tx, orgID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func seedDefaultCatalogTx(ctx context.Context, tx pgx.Tx, orgID uuid.UUID) error {
	categoryIDs := map[string]uuid.UUID{}
	for _, c := range defaultCategories {
		var id uuid.UUID
		err := tx.QueryRow(ctx, `
			INSERT INTO catalog_categories (organization_id, template_key, name, sort_order, is_default_seed)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (organization_id, template_key) WHERE template_key IS NOT NULL DO UPDATE SET
				updated_at = catalog_categories.updated_at
			RETURNING id
		`, orgID, c.Key, c.Name, c.SortOrder, defaultTemplateFlag).Scan(&id)
		if err != nil {
			return fmt.Errorf("seed category %s: %w", c.Key, err)
		}
		categoryIDs[c.Key] = id
	}

	for _, item := range defaultItems {
		categoryID, ok := categoryIDs[item.CategoryKey]
		if !ok {
			return fmt.Errorf("seed item %s: missing category %s", item.Key, item.CategoryKey)
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO catalog_items (
				organization_id, category_id, template_key, name, unit, charge_type,
				stock_mode, price_usd_cents, is_taxable, is_required_fee
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			ON CONFLICT (organization_id, template_key) WHERE template_key IS NOT NULL DO NOTHING
		`, orgID, categoryID, item.Key, item.Name, item.Unit, item.ChargeType, item.StockMode, item.PriceUSDCents, item.IsTaxable, item.IsRequiredFee)
		if err != nil {
			return fmt.Errorf("seed item %s: %w", item.Key, err)
		}
	}
	return nil
}

func (p *Pool) CatalogCategories(ctx context.Context, orgID uuid.UUID) ([]*CatalogCategory, error) {
	rows, err := p.Query(ctx, `
		SELECT `+catalogCategorySelect+`
		FROM catalog_categories c
		LEFT JOIN catalog_items i ON i.category_id = c.id AND i.archived_at IS NULL
		WHERE c.organization_id = $1
		GROUP BY c.id
		ORDER BY c.archived_at NULLS FIRST, c.sort_order, lower(c.name)
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*CatalogCategory
	for rows.Next() {
		c := &CatalogCategory{}
		if err := scanCatalogCategory(rows, c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (p *Pool) CreateCatalogCategory(ctx context.Context, orgID uuid.UUID, name string, sortOrder int) (*CatalogCategory, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("store.CreateCatalogCategory: name required")
	}
	c := &CatalogCategory{}
	err := scanCatalogCategory(p.QueryRow(ctx, `
		WITH inserted AS (
			INSERT INTO catalog_categories (organization_id, name, sort_order)
			VALUES ($1, $2, $3)
			RETURNING *
		)
		SELECT `+catalogCategorySelect+`
		FROM inserted c
		LEFT JOIN catalog_items i ON false
		GROUP BY c.id, c.organization_id, c.template_key, c.name, c.sort_order, c.is_default_seed, c.archived_at, c.created_at, c.updated_at
	`, orgID, name, sortOrder), c)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (p *Pool) UpdateCatalogCategory(ctx context.Context, orgID, categoryID uuid.UUID, name string, sortOrder int, archived *bool) (*CatalogCategory, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("store.UpdateCatalogCategory: name required")
	}
	if archived != nil && *archived {
		var n int
		if err := p.QueryRow(ctx, `
			SELECT count(*) FROM catalog_items
			WHERE organization_id = $1 AND category_id = $2 AND archived_at IS NULL
		`, orgID, categoryID).Scan(&n); err != nil {
			return nil, err
		}
		if n > 0 {
			return nil, errors.New("category has active items")
		}
	}
	archiveSQL := "archived_at"
	if archived != nil {
		if *archived {
			archiveSQL = "now()"
		} else {
			archiveSQL = "NULL"
		}
	}
	c := &CatalogCategory{}
	err := scanCatalogCategory(p.QueryRow(ctx, `
		WITH updated AS (
			UPDATE catalog_categories
			SET name = $3, sort_order = $4, archived_at = `+archiveSQL+`, updated_at = now()
			WHERE organization_id = $1 AND id = $2
			RETURNING *
		)
		SELECT `+catalogCategorySelect+`
		FROM updated c
		LEFT JOIN catalog_items i ON i.category_id = c.id AND i.archived_at IS NULL
		GROUP BY c.id, c.organization_id, c.template_key, c.name, c.sort_order, c.is_default_seed, c.archived_at, c.created_at, c.updated_at
	`, orgID, categoryID, name, sortOrder), c)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (p *Pool) CatalogItems(ctx context.Context, orgID uuid.UUID) ([]*CatalogItem, error) {
	rows, err := p.Query(ctx, `
		SELECT `+catalogItemSelect+`
		FROM catalog_items i
		JOIN catalog_categories c ON c.id = i.category_id
		WHERE i.organization_id = $1
		ORDER BY c.sort_order, lower(c.name), lower(i.name)
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*CatalogItem
	for rows.Next() {
		i := &CatalogItem{}
		if err := scanCatalogItem(rows, i); err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

func (p *Pool) CatalogItemByID(ctx context.Context, orgID, itemID uuid.UUID) (*CatalogItem, error) {
	i := &CatalogItem{}
	err := scanCatalogItem(p.QueryRow(ctx, `
		SELECT `+catalogItemSelect+`
		FROM catalog_items i
		JOIN catalog_categories c ON c.id = i.category_id
		WHERE i.organization_id = $1 AND i.id = $2
	`, orgID, itemID), i)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return i, nil
}

func (p *Pool) CreateCatalogItem(ctx context.Context, orgID uuid.UUID, in CatalogItemInput) (*CatalogItem, error) {
	if err := validateCatalogItemInput(in); err != nil {
		return nil, err
	}
	if err := p.ensureCategoryInOrg(ctx, orgID, in.CategoryID); err != nil {
		return nil, err
	}
	i := &CatalogItem{}
	err := scanCatalogItem(p.QueryRow(ctx, `
		WITH inserted AS (
			INSERT INTO catalog_items (
				organization_id, category_id, name, description, unit, charge_type,
				stock_mode, price_usd_cents, is_taxable, is_required_fee, is_active
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
			RETURNING *
		)
		SELECT `+catalogItemSelect+`
		FROM inserted i
		JOIN catalog_categories c ON c.id = i.category_id
	`, orgID, in.CategoryID, strings.TrimSpace(in.Name), in.Description, strings.TrimSpace(in.Unit), in.ChargeType, in.StockMode, in.PriceUSDCents, in.IsTaxable, in.IsRequiredFee, in.IsActive), i)
	if err != nil {
		return nil, err
	}
	return i, nil
}

func (p *Pool) UpdateCatalogItem(ctx context.Context, orgID, itemID uuid.UUID, in CatalogItemInput, archived *bool) (*CatalogItem, error) {
	if err := validateCatalogItemInput(in); err != nil {
		return nil, err
	}
	if err := p.ensureCategoryInOrg(ctx, orgID, in.CategoryID); err != nil {
		return nil, err
	}
	archiveSQL := "archived_at"
	if archived != nil {
		if *archived {
			archiveSQL = "now()"
		} else {
			archiveSQL = "NULL"
		}
	}
	i := &CatalogItem{}
	err := scanCatalogItem(p.QueryRow(ctx, `
		WITH updated AS (
			UPDATE catalog_items
			SET category_id = $3, name = $4, description = $5, unit = $6,
				charge_type = $7, stock_mode = $8, price_usd_cents = $9,
				is_taxable = $10, is_required_fee = $11, is_active = $12,
				archived_at = `+archiveSQL+`, updated_at = now()
			WHERE organization_id = $1 AND id = $2
			RETURNING *
		)
		SELECT `+catalogItemSelect+`
		FROM updated i
		JOIN catalog_categories c ON c.id = i.category_id
	`, orgID, itemID, in.CategoryID, strings.TrimSpace(in.Name), in.Description, strings.TrimSpace(in.Unit), in.ChargeType, in.StockMode, in.PriceUSDCents, in.IsTaxable, in.IsRequiredFee, in.IsActive), i)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return i, nil
}

func (p *Pool) ensureCategoryInOrg(ctx context.Context, orgID, categoryID uuid.UUID) error {
	var exists bool
	if err := p.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM catalog_categories WHERE organization_id = $1 AND id = $2 AND archived_at IS NULL)`, orgID, categoryID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	return nil
}

func validateCatalogItemInput(in CatalogItemInput) error {
	if strings.TrimSpace(in.Name) == "" {
		return errors.New("catalog item name required")
	}
	if strings.TrimSpace(in.Unit) == "" {
		return errors.New("catalog item unit required")
	}
	if in.PriceUSDCents < 0 {
		return errors.New("catalog item price must be non-negative")
	}
	switch in.ChargeType {
	case ChargeTypeSale, ChargeTypeRental, ChargeTypeService, ChargeTypeFee, ChargeTypeGratuity, ChargeTypeDeposit, ChargeTypeDamage, ChargeTypeIncluded:
	default:
		return errors.New("invalid charge_type")
	}
	switch in.StockMode {
	case StockModeNone, StockModeCounted:
	default:
		return errors.New("invalid stock_mode")
	}
	return nil
}
