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

type Item struct {
	ID                  int64             `json:"id"`
	EstimateID          int64             `json:"estimate_id"`
	ServiceID           int64             `json:"service_id"`
	ServiceCode         string            `json:"service_code,omitempty"`
	RegionCode          string            `json:"region_code"`
	SKU                 string            `json:"sku"`
	Quantity            decimal.Decimal   `json:"quantity"`
	Attributes          map[string]string `json:"attributes"`
	UnitPriceSnapshot   decimal.Decimal   `json:"unit_price"`
	CurrencySnapshot    string            `json:"currency"`
	MonthlyCostSnapshot decimal.Decimal   `json:"monthly_cost"`
}

const HoursPerMonth = 730
