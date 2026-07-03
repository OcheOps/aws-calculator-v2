package estimates

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/OcheOps/aws-calculator/api/internal/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

type Store struct{ pool *pgxpool.Pool }

func NewStore(p *pgxpool.Pool) *Store { return &Store{pool: p} }

// Pool exposes the underlying pgx pool so authorization checks in the HTTP
// layer can query estimate metadata without leaking a second dependency.
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

type CreateInput struct {
	OrgID     *int64
	CreatedBy *int64
	Name      string
	Currency  string
	Notes     string
}

// Create returns the new estimate. If OrgID is nil, an edit token is also
// generated and returned (the raw token, hash goes to the DB); anonymous
// clients keep the raw token to prove ownership on subsequent requests.
func (s *Store) Create(ctx context.Context, in CreateInput) (*Estimate, string, error) {
	var (
		est      Estimate
		editRaw  string
		editHash any = nil
	)
	if in.OrgID == nil {
		t, err := auth.GenerateToken()
		if err != nil {
			return nil, "", err
		}
		editRaw = t
		editHash = auth.HashToken(t)
	}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO estimates (org_id, created_by, name, currency, notes, edit_token_hash)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, name, currency, created_at, updated_at`,
		in.OrgID, in.CreatedBy, in.Name, in.Currency, in.Notes, editHash).
		Scan(&est.ID, &est.Name, &est.Currency, &est.CreatedAt, &est.UpdatedAt)
	if err != nil {
		return nil, "", err
	}
	est.OrgID = in.OrgID
	est.CreatedBy = in.CreatedBy
	est.Notes = in.Notes
	return &est, editRaw, nil
}

type AddItemInput struct {
	ServiceID  int64
	RegionCode string
	SKU        string
	Quantity   decimal.Decimal
	Attributes map[string]string
	UnitPrice  decimal.Decimal
	Unit       string
	Currency   string
}

func (s *Store) AddItem(ctx context.Context, estimateID int64, in AddItemInput) (*Item, error) {
	monthly := MonthlyCost(in.Unit, in.UnitPrice, in.Quantity)
	attrs, err := json.Marshal(in.Attributes)
	if err != nil {
		return nil, err
	}
	var item Item
	err = s.pool.QueryRow(ctx, `
		INSERT INTO estimate_items (estimate_id, service_id, region_code, sku, quantity, unit_override, attributes_snapshot, unit_price_snapshot, currency_snapshot, monthly_cost_snapshot)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id`,
		estimateID, in.ServiceID, in.RegionCode, in.SKU, in.Quantity.String(), in.Unit, attrs, in.UnitPrice.String(), in.Currency, monthly.String()).
		Scan(&item.ID)
	if err != nil {
		return nil, err
	}
	item.EstimateID = estimateID
	item.ServiceID = in.ServiceID
	item.RegionCode = in.RegionCode
	item.SKU = in.SKU
	item.Quantity = in.Quantity
	item.Attributes = in.Attributes
	item.UnitPriceSnapshot = in.UnitPrice
	item.CurrencySnapshot = in.Currency
	item.MonthlyCostSnapshot = monthly
	if _, err := s.pool.Exec(ctx, `UPDATE estimates SET updated_at = NOW() WHERE id = $1`, estimateID); err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Store) ClaimByEditTokens(ctx context.Context, orgID, userID int64, rawTokens []string) (int, error) {
	if len(rawTokens) == 0 {
		return 0, nil
	}
	hashes := make([]string, len(rawTokens))
	for i, t := range rawTokens {
		hashes[i] = auth.HashToken(t)
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE estimates SET org_id=$1, created_by=$2, edit_token_hash=NULL, updated_at=NOW()
		WHERE org_id IS NULL AND edit_token_hash = ANY($3)`, orgID, userID, hashes)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

func (s *Store) Get(ctx context.Context, id int64) (*Estimate, error) {
	var est Estimate
	err := s.pool.QueryRow(ctx, `
		SELECT id, org_id, created_by, name, currency, COALESCE(notes,''), created_at, updated_at
		FROM estimates WHERE id = $1`, id).Scan(&est.ID, &est.OrgID, &est.CreatedBy, &est.Name, &est.Currency, &est.Notes, &est.CreatedAt, &est.UpdatedAt)
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT ei.id, ei.estimate_id, ei.service_id, s.code, ei.region_code, ei.sku, ei.quantity::text, ei.attributes_snapshot, ei.unit_price_snapshot::text, ei.currency_snapshot, ei.monthly_cost_snapshot::text
		FROM estimate_items ei JOIN services s ON s.id = ei.service_id
		WHERE ei.estimate_id = $1 ORDER BY ei.id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	total := decimal.NewFromInt(0)
	for rows.Next() {
		var it Item
		var qty, price, monthly string
		var attrs []byte
		if err := rows.Scan(&it.ID, &it.EstimateID, &it.ServiceID, &it.ServiceCode, &it.RegionCode, &it.SKU, &qty, &attrs, &price, &it.CurrencySnapshot, &monthly); err != nil {
			return nil, err
		}
		it.Quantity, _ = decimal.NewFromString(qty)
		it.UnitPriceSnapshot, _ = decimal.NewFromString(price)
		it.MonthlyCostSnapshot, _ = decimal.NewFromString(monthly)
		_ = json.Unmarshal(attrs, &it.Attributes)
		est.Items = append(est.Items, it)
		total = total.Add(it.MonthlyCostSnapshot)
	}
	est.Total = total
	return &est, nil
}

func (s *Store) List(ctx context.Context, orgID int64) ([]Estimate, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, org_id, created_by, name, currency, created_at, updated_at FROM estimates WHERE org_id = $1 ORDER BY updated_at DESC LIMIT 200`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Estimate
	for rows.Next() {
		var e Estimate
		if err := rows.Scan(&e.ID, &e.OrgID, &e.CreatedBy, &e.Name, &e.Currency, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

type UpdatePatch struct {
	Name     *string
	Currency *string
	Notes    *string
}

func (s *Store) Update(ctx context.Context, id int64, p UpdatePatch) error {
	set := []string{"updated_at = NOW()"}
	args := []any{id}
	if p.Name != nil {
		args = append(args, *p.Name)
		set = append(set, fmt.Sprintf("name = $%d", len(args)))
	}
	if p.Currency != nil {
		args = append(args, *p.Currency)
		set = append(set, fmt.Sprintf("currency = $%d", len(args)))
	}
	if p.Notes != nil {
		args = append(args, *p.Notes)
		set = append(set, fmt.Sprintf("notes = $%d", len(args)))
	}
	q := "UPDATE estimates SET " + strings.Join(set, ", ") + " WHERE id = $1"
	_, err := s.pool.Exec(ctx, q, args...)
	return err
}

func (s *Store) Delete(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM estimates WHERE id = $1`, id)
	return err
}

func (s *Store) DeleteItem(ctx context.Context, estimateID, itemID int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM estimate_items WHERE id = $1 AND estimate_id = $2`, itemID, estimateID)
	return err
}

func (s *Store) CreateShare(ctx context.Context, estimateID int64) (string, error) {
	raw, err := auth.GenerateToken()
	if err != nil {
		return "", err
	}
	if _, err := s.pool.Exec(ctx, `INSERT INTO estimate_shares (estimate_id, share_token_hash) VALUES ($1, $2)`, estimateID, auth.HashToken(raw)); err != nil {
		return "", err
	}
	return raw, nil
}

func (s *Store) LookupByShare(ctx context.Context, rawToken string) (*Estimate, error) {
	var estID int64
	if err := s.pool.QueryRow(ctx, `SELECT estimate_id FROM estimate_shares WHERE share_token_hash = $1 AND (expires_at IS NULL OR expires_at > NOW())`, auth.HashToken(rawToken)).Scan(&estID); err != nil {
		return nil, pgx.ErrNoRows
	}
	return s.Get(ctx, estID)
}

// MatchesEditToken reports whether the raw edit token matches the estimate's
// stored hash. Returns false for authenticated (non-anonymous) estimates.
func (s *Store) MatchesEditToken(ctx context.Context, estimateID int64, raw string) bool {
	if raw == "" {
		return false
	}
	var hash string
	if err := s.pool.QueryRow(ctx, `SELECT COALESCE(edit_token_hash,'') FROM estimates WHERE id = $1`, estimateID).Scan(&hash); err != nil {
		return false
	}
	return hash != "" && hash == auth.HashToken(raw)
}
