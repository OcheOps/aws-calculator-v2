package estimates

import (
	"time"

	"github.com/shopspring/decimal"
)

type Estimate struct {
	ID        int64           `json:"id"`
	OrgID     *int64          `json:"org_id,omitempty"`
	CreatedBy *int64          `json:"created_by,omitempty"`
	Name      string          `json:"name"`
	Currency  string          `json:"currency"`
	Notes     string          `json:"notes,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	Items     []Item          `json:"items,omitempty"`
	Total     decimal.Decimal `json:"total,omitempty"`
}

// LineType distinguishes the two shapes an estimate row can take.
// Legacy rows carry a single SKU price directly on the row; curated rows
// carry an array of priced components + the user's original input so the
// UI can rebuild the human summary.
type LineType string

const (
	LineTypeLegacy  LineType = "legacy"
	LineTypeCurated LineType = "curated"
)

type Component struct {
	SKU         string          `json:"sku"`
	Unit        string          `json:"unit"`
	Quantity    decimal.Decimal `json:"quantity"`
	UnitPrice   decimal.Decimal `json:"unit_price"`
	MonthlyCost decimal.Decimal `json:"monthly_cost"`
	Currency    string          `json:"currency"`
	Attributes  map[string]string `json:"attributes,omitempty"`
	Region      string          `json:"region_code,omitempty"`
	Service     string          `json:"service_code,omitempty"`
}

type Item struct {
	ID                  int64             `json:"id"`
	EstimateID          int64             `json:"estimate_id"`
	LineType            LineType          `json:"line_type"`
	ServiceKey          string            `json:"service_key,omitempty"`
	ServiceID           int64             `json:"service_id"`
	ServiceCode         string            `json:"service_code,omitempty"`
	RegionCode          string            `json:"region_code"`
	SKU                 string            `json:"sku"`
	Quantity            decimal.Decimal   `json:"quantity"`
	Attributes          map[string]string `json:"attributes"`
	UnitPriceSnapshot   decimal.Decimal   `json:"unit_price"`
	CurrencySnapshot    string            `json:"currency"`
	MonthlyCostSnapshot decimal.Decimal   `json:"monthly_cost"`
	UserInputSnapshot   any               `json:"user_input,omitempty"`
	ComponentsSnapshot  []Component       `json:"components,omitempty"`
}

const HoursPerMonth = 730
